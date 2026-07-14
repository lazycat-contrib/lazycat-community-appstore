package server

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/andybalholm/brotli"
	"github.com/dgraph-io/badger/v4"
	"golang.org/x/sync/singleflight"
)

const (
	sourceFeedCacheTTL       = 24 * time.Hour
	sourceFeedBuildTimeout   = 30 * time.Second
	sourceFeedBrotliQuality  = 5
	sourceFeedCacheKeyPrefix = "source-feed/"
)

type sourceFeedAccessScope struct {
	GroupIDs []int
	Groups   []sourceGroupDTO
}

func (scope sourceFeedAccessScope) canonical() sourceFeedAccessScope {
	ids := slices.Clone(scope.GroupIDs)
	slices.Sort(ids)
	ids = slices.Compact(ids)
	groups := slices.Clone(scope.Groups)
	slices.SortFunc(groups, func(left, right sourceGroupDTO) int {
		return left.ID - right.ID
	})
	return sourceFeedAccessScope{GroupIDs: ids, Groups: groups}
}

func (scope sourceFeedAccessScope) cacheKey() string {
	canonical := scope.canonical()
	var value strings.Builder
	value.Grow(len(canonical.GroupIDs) * 8)
	for index, id := range canonical.GroupIDs {
		if index > 0 {
			value.WriteByte(',')
		}
		value.WriteString(strconv.Itoa(id))
	}
	sum := sha256.Sum256([]byte(value.String()))
	return hex.EncodeToString(sum[:])
}

type sourceFeedSnapshot struct {
	Identity []byte
	Brotli   []byte
	Gzip     []byte
	ETag     string
	BuiltAt  time.Time
}

type sourceFeedBuilder func(context.Context, int, sourceFeedAccessScope) ([]byte, error)

type sourceFeedCache struct {
	db         *badger.DB
	build      sourceFeedBuilder
	bootID     string
	ctx        context.Context
	cancel     context.CancelFunc
	generation atomic.Uint64
	loads      singleflight.Group
	mu         sync.Mutex
	closed     bool
	wg         sync.WaitGroup
}

type sourceFeedSnapshotMeta struct {
	ETag    string    `json:"etag"`
	BuiltAt time.Time `json:"builtAt"`
}

func newSourceFeedCache(parent context.Context, path string, build sourceFeedBuilder) (*sourceFeedCache, error) {
	if parent == nil {
		parent = context.Background()
	}
	if build == nil {
		return nil, errors.New("source feed builder is required")
	}
	options := badger.DefaultOptions(strings.TrimSpace(path)).WithLogger(nil)
	if strings.TrimSpace(path) == "" {
		options = options.WithInMemory(true)
	}
	db, err := badger.Open(options)
	if err != nil {
		return nil, fmt.Errorf("open source feed cache: %w", err)
	}
	bootID, err := randomSourceFeedBootID()
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	ctx, cancel := context.WithCancel(parent)
	return &sourceFeedCache{db: db, build: build, bootID: bootID, ctx: ctx, cancel: cancel}, nil
}

func randomSourceFeedBootID() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("create source feed cache boot id: %w", err)
	}
	return hex.EncodeToString(raw[:]), nil
}

func (cache *sourceFeedCache) GetOrBuild(ctx context.Context, version int, scope sourceFeedAccessScope) (sourceFeedSnapshot, error) {
	if cache == nil {
		return sourceFeedSnapshot{}, errors.New("source feed cache is unavailable")
	}
	scope = scope.canonical()
	for {
		generation := cache.generation.Load()
		key := cache.snapshotPrefix(generation, version, scope.cacheKey())
		result := cache.loads.DoChan(key, func() (any, error) {
			buildCtx, cancel := context.WithTimeout(cache.ctx, sourceFeedBuildTimeout)
			defer cancel()
			return cache.loadOrBuild(buildCtx, key, version, scope)
		})
		select {
		case <-ctx.Done():
			return sourceFeedSnapshot{}, ctx.Err()
		case item := <-result:
			if item.Err != nil {
				return sourceFeedSnapshot{}, item.Err
			}
			if generation != cache.generation.Load() {
				continue
			}
			return item.Val.(sourceFeedSnapshot), nil
		}
	}
}

func (cache *sourceFeedCache) loadOrBuild(ctx context.Context, prefix string, version int, scope sourceFeedAccessScope) (sourceFeedSnapshot, error) {
	snapshot, err := cache.load(prefix)
	if err == nil {
		return snapshot, nil
	}
	if !errors.Is(err, badger.ErrKeyNotFound) {
		slog.Warn("source feed cache read failed; rebuilding without cache", "error", err)
		return cache.buildSnapshot(ctx, version, scope)
	}
	snapshot, err = cache.buildSnapshot(ctx, version, scope)
	if err != nil {
		return sourceFeedSnapshot{}, err
	}
	if err := cache.store(prefix, snapshot); err != nil {
		slog.Warn("source feed cache write failed; serving generated snapshot", "error", err)
	}
	return snapshot, nil
}

func (cache *sourceFeedCache) buildSnapshot(ctx context.Context, version int, scope sourceFeedAccessScope) (sourceFeedSnapshot, error) {
	identity, err := cache.build(ctx, version, scope)
	if err != nil {
		return sourceFeedSnapshot{}, err
	}
	return newSourceFeedSnapshot(identity)
}

func newSourceFeedSnapshot(identity []byte) (sourceFeedSnapshot, error) {
	brotliBytes, err := compressSourceFeedBrotli(identity)
	if err != nil {
		return sourceFeedSnapshot{}, fmt.Errorf("brotli source feed: %w", err)
	}
	gzipBytes, err := compressSourceFeedGzip(identity)
	if err != nil {
		return sourceFeedSnapshot{}, fmt.Errorf("gzip source feed: %w", err)
	}
	sum := sha256.Sum256(identity)
	return sourceFeedSnapshot{
		Identity: bytes.Clone(identity),
		Brotli:   brotliBytes,
		Gzip:     gzipBytes,
		ETag:     `W/"` + hex.EncodeToString(sum[:]) + `"`,
		BuiltAt:  time.Now().UTC(),
	}, nil
}

func compressSourceFeedBrotli(raw []byte) ([]byte, error) {
	var buffer bytes.Buffer
	writer := brotli.NewWriterLevel(&buffer, sourceFeedBrotliQuality)
	if _, err := writer.Write(raw); err != nil {
		_ = writer.Close()
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func compressSourceFeedGzip(raw []byte) ([]byte, error) {
	var buffer bytes.Buffer
	writer := gzip.NewWriter(&buffer)
	if _, err := writer.Write(raw); err != nil {
		_ = writer.Close()
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func (cache *sourceFeedCache) load(prefix string) (sourceFeedSnapshot, error) {
	var snapshot sourceFeedSnapshot
	err := cache.db.View(func(tx *badger.Txn) error {
		metaRaw, err := badgerValueCopy(tx, prefix+"meta")
		if err != nil {
			return err
		}
		var meta sourceFeedSnapshotMeta
		if err := json.Unmarshal(metaRaw, &meta); err != nil {
			return err
		}
		identity, err := badgerValueCopy(tx, prefix+"identity")
		if err != nil {
			return err
		}
		brotliBytes, err := badgerValueCopy(tx, prefix+"br")
		if err != nil {
			return err
		}
		gzipBytes, err := badgerValueCopy(tx, prefix+"gzip")
		if err != nil {
			return err
		}
		snapshot = sourceFeedSnapshot{Identity: identity, Brotli: brotliBytes, Gzip: gzipBytes, ETag: meta.ETag, BuiltAt: meta.BuiltAt}
		return nil
	})
	return snapshot, err
}

func badgerValueCopy(tx *badger.Txn, key string) ([]byte, error) {
	item, err := tx.Get([]byte(key))
	if err != nil {
		return nil, err
	}
	return item.ValueCopy(nil)
}

func (cache *sourceFeedCache) store(prefix string, snapshot sourceFeedSnapshot) error {
	meta, err := json.Marshal(sourceFeedSnapshotMeta{ETag: snapshot.ETag, BuiltAt: snapshot.BuiltAt})
	if err != nil {
		return err
	}
	return cache.db.Update(func(tx *badger.Txn) error {
		values := []struct {
			key   string
			value []byte
		}{
			{key: prefix + "meta", value: meta},
			{key: prefix + "identity", value: snapshot.Identity},
			{key: prefix + "br", value: snapshot.Brotli},
			{key: prefix + "gzip", value: snapshot.Gzip},
		}
		for _, value := range values {
			entry := badger.NewEntry([]byte(value.key), value.value).WithTTL(sourceFeedCacheTTL)
			if err := tx.SetEntry(entry); err != nil {
				return err
			}
		}
		return nil
	})
}

func (cache *sourceFeedCache) InvalidateAndWarm() {
	if cache == nil {
		return
	}
	previous := cache.generation.Add(1) - 1
	cache.start(func() {
		if err := cache.db.DropPrefix([]byte(cache.generationPrefix(previous))); err != nil {
			slog.Warn("source feed cache cleanup failed", "error", err)
		}
	})
	for _, version := range []int{1, 2} {
		cache.start(func() {
			if _, err := cache.GetOrBuild(cache.ctx, version, sourceFeedAccessScope{}); err != nil && !errors.Is(err, context.Canceled) {
				slog.Warn("source feed cache warm failed", "version", version, "error", err)
			}
		})
	}
}

func (cache *sourceFeedCache) start(run func()) bool {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if cache.closed {
		return false
	}
	cache.wg.Go(run)
	return true
}

func (cache *sourceFeedCache) Close() error {
	if cache == nil {
		return nil
	}
	cache.mu.Lock()
	if cache.closed {
		cache.mu.Unlock()
		return nil
	}
	cache.closed = true
	cache.cancel()
	cache.mu.Unlock()
	cache.wg.Wait()
	return cache.db.Close()
}

func (cache *sourceFeedCache) snapshotPrefix(generation uint64, version int, scopeKey string) string {
	return cache.generationPrefix(generation) + strconv.Itoa(version) + "/" + scopeKey + "/"
}

func (cache *sourceFeedCache) generationPrefix(generation uint64) string {
	return sourceFeedCacheKeyPrefix + cache.bootID + "/" + strconv.FormatUint(generation, 10) + "/"
}
