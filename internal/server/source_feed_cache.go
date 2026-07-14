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
	sourceFeedCacheTTL          = 24 * time.Hour
	sourceFeedBuildTimeout      = 30 * time.Second
	sourceFeedBrotliQuality     = 5
	sourceFeedCacheKeyPrefix    = "source-feed/"
	sourceFeedCacheMaxSnapshots = 256
	sourceFeedCacheMaxBytes     = 256 << 20
	sourceFeedBuildConcurrency  = 4
)

var errSourceFeedGenerationChanged = errors.New("source feed cache generation changed")

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
	raw, _ := json.Marshal(canonical)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

type sourceFeedSnapshot struct {
	Identity   []byte
	Brotli     []byte
	Gzip       []byte
	ETag       string
	BuiltAt    time.Time
	ValidUntil time.Time
}

func (snapshot sourceFeedSnapshot) expired(now time.Time) bool {
	return !snapshot.ValidUntil.IsZero() && !now.Before(snapshot.ValidUntil)
}

type sourceFeedBuild struct {
	Identity   []byte
	ValidUntil time.Time
}

type sourceFeedBuilder func(context.Context, int, sourceFeedAccessScope, []string) (sourceFeedBuild, error)

type sourceFeedCache struct {
	db            *badger.DB
	build         sourceFeedBuilder
	bootID        string
	ctx           context.Context
	cancel        context.CancelFunc
	generation    atomic.Uint64
	loads         singleflight.Group
	maintenance   chan struct{}
	buildSlots    chan struct{}
	mu            sync.Mutex
	admitted      map[string]int64
	admittedBytes int64
	closed        bool
	wg            sync.WaitGroup
}

type sourceFeedSnapshotMeta struct {
	ETag       string    `json:"etag"`
	BuiltAt    time.Time `json:"builtAt"`
	ValidUntil time.Time `json:"validUntil,omitempty"`
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
	if err := db.DropPrefix([]byte(sourceFeedCacheKeyPrefix)); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("clear previous source feed cache: %w", err)
	}
	bootID, err := randomSourceFeedBootID()
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	ctx, cancel := context.WithCancel(parent)
	cache := &sourceFeedCache{
		db:          db,
		build:       build,
		bootID:      bootID,
		ctx:         ctx,
		cancel:      cancel,
		maintenance: make(chan struct{}, 1),
		buildSlots:  make(chan struct{}, sourceFeedBuildConcurrency),
		admitted:    map[string]int64{},
	}
	cache.start(cache.runMaintenance)
	return cache, nil
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
	if ctx == nil {
		ctx = context.Background()
	}
	scope = scope.canonical()
	generation := cache.generation.Load()
	key := cache.snapshotPrefix(generation, version, scope.cacheKey())
	result := cache.loads.DoChan(key, func() (any, error) {
		if !cache.beginWork() {
			return nil, context.Canceled
		}
		defer cache.wg.Done()
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
			return sourceFeedSnapshot{}, errSourceFeedGenerationChanged
		}
		snapshot := item.Val.(sourceFeedSnapshot)
		if snapshot.expired(time.Now().UTC()) {
			cache.loads.Forget(key)
			return sourceFeedSnapshot{}, errSourceFeedGenerationChanged
		}
		return snapshot, nil
	}
}

func (cache *sourceFeedCache) BuildUncached(ctx context.Context, version int, scope sourceFeedAccessScope, invalidGroupCodes []string) (sourceFeedSnapshot, error) {
	if cache == nil {
		return sourceFeedSnapshot{}, errors.New("source feed cache is unavailable")
	}
	if !cache.beginWork() {
		return sourceFeedSnapshot{}, context.Canceled
	}
	defer cache.wg.Done()
	return cache.buildSnapshot(ctx, version, scope.canonical(), invalidGroupCodes)
}

func (cache *sourceFeedCache) loadOrBuild(ctx context.Context, prefix string, version int, scope sourceFeedAccessScope) (sourceFeedSnapshot, error) {
	snapshot, err := cache.load(prefix)
	if err == nil {
		return snapshot, nil
	}
	if !errors.Is(err, badger.ErrKeyNotFound) {
		slog.Warn("source feed cache read failed; rebuilding without cache", "error", err)
		return cache.buildSnapshot(ctx, version, scope, nil)
	}
	snapshot, err = cache.buildSnapshot(ctx, version, scope, nil)
	if err != nil {
		return sourceFeedSnapshot{}, err
	}
	if snapshot.expired(time.Now().UTC()) {
		return sourceFeedSnapshot{}, errSourceFeedGenerationChanged
	}
	cacheBytes := int64(len(snapshot.Identity) + len(snapshot.Brotli) + len(snapshot.Gzip))
	if cache.admit(prefix, cacheBytes) {
		if err := cache.store(prefix, snapshot); err != nil {
			slog.Warn("source feed cache write failed; serving generated snapshot", "error", err)
		}
	}
	return snapshot, nil
}

func (cache *sourceFeedCache) buildSnapshot(ctx context.Context, version int, scope sourceFeedAccessScope, invalidGroupCodes []string) (sourceFeedSnapshot, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case cache.buildSlots <- struct{}{}:
		defer func() { <-cache.buildSlots }()
	case <-ctx.Done():
		return sourceFeedSnapshot{}, ctx.Err()
	}
	built, err := cache.build(ctx, version, scope, invalidGroupCodes)
	if err != nil {
		return sourceFeedSnapshot{}, err
	}
	return newSourceFeedSnapshotUntil(built.Identity, built.ValidUntil)
}

func newSourceFeedSnapshotUntil(identity []byte, validUntil time.Time) (sourceFeedSnapshot, error) {
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
		Identity:   bytes.Clone(identity),
		Brotli:     brotliBytes,
		Gzip:       gzipBytes,
		ETag:       `W/"` + hex.EncodeToString(sum[:]) + `"`,
		BuiltAt:    time.Now().UTC(),
		ValidUntil: validUntil.UTC(),
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
		if !meta.ValidUntil.IsZero() && !time.Now().UTC().Before(meta.ValidUntil) {
			return badger.ErrKeyNotFound
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
		snapshot = sourceFeedSnapshot{
			Identity:   identity,
			Brotli:     brotliBytes,
			Gzip:       gzipBytes,
			ETag:       meta.ETag,
			BuiltAt:    meta.BuiltAt,
			ValidUntil: meta.ValidUntil,
		}
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

func (cache *sourceFeedCache) admit(prefix string, size int64) bool {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if cache.closed || !strings.HasPrefix(prefix, cache.generationPrefix(cache.generation.Load())) {
		return false
	}
	if previous, ok := cache.admitted[prefix]; ok {
		delta := size - previous
		if size <= 0 || size > sourceFeedCacheMaxBytes || delta > 0 && cache.admittedBytes+delta > sourceFeedCacheMaxBytes {
			return false
		}
		cache.admitted[prefix] = size
		cache.admittedBytes += delta
		return true
	}
	if size <= 0 || size > sourceFeedCacheMaxBytes || len(cache.admitted) >= sourceFeedCacheMaxSnapshots || cache.admittedBytes+size > sourceFeedCacheMaxBytes {
		return false
	}
	cache.admitted[prefix] = size
	cache.admittedBytes += size
	return true
}

func (cache *sourceFeedCache) store(prefix string, snapshot sourceFeedSnapshot) error {
	meta, err := json.Marshal(sourceFeedSnapshotMeta{ETag: snapshot.ETag, BuiltAt: snapshot.BuiltAt, ValidUntil: snapshot.ValidUntil})
	if err != nil {
		return err
	}
	ttl := sourceFeedCacheTTL
	if !snapshot.ValidUntil.IsZero() {
		remaining := time.Until(snapshot.ValidUntil)
		if remaining <= 0 {
			return nil
		}
		if remaining < ttl {
			ttl = remaining
		}
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
			entry := badger.NewEntry([]byte(value.key), value.value).WithTTL(ttl)
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
	cache.mu.Lock()
	if cache.closed {
		cache.mu.Unlock()
		return
	}
	cache.generation.Add(1)
	cache.admitted = map[string]int64{}
	cache.admittedBytes = 0
	cache.mu.Unlock()
	select {
	case cache.maintenance <- struct{}{}:
	default:
	}
}

func (cache *sourceFeedCache) runMaintenance() {
	var cleanedThrough uint64
	for {
		select {
		case <-cache.ctx.Done():
			return
		case <-cache.maintenance:
		}
		for {
			select {
			case <-cache.maintenance:
				continue
			default:
			}
			break
		}
		target := cache.generation.Load()
		for _, version := range []int{1, 2} {
			_, err := cache.GetOrBuild(cache.ctx, version, sourceFeedAccessScope{})
			if errors.Is(err, context.Canceled) {
				return
			}
			if errors.Is(err, errSourceFeedGenerationChanged) || cache.generation.Load() != target {
				break
			}
			if err != nil {
				slog.Warn("source feed cache warm failed", "version", version, "error", err)
			}
		}
		if cache.generation.Load() != target || cleanedThrough >= target {
			continue
		}
		prefixes := make([][]byte, 0, target-cleanedThrough)
		for generation := cleanedThrough; generation < target; generation++ {
			prefixes = append(prefixes, []byte(cache.generationPrefix(generation)))
		}
		if err := cache.db.DropPrefix(prefixes...); err != nil {
			slog.Warn("source feed cache cleanup failed", "error", err)
			continue
		}
		cleanedThrough = target
	}
}

func (cache *sourceFeedCache) beginWork() bool {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if cache.closed {
		return false
	}
	cache.wg.Add(1)
	return true
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
