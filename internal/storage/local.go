package storage

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type LocalBackend struct {
	root      string
	urlPrefix string
}

func NewLocalBackend(root, urlPrefix string) *LocalBackend {
	if urlPrefix == "" {
		urlPrefix = "/files/"
	}
	if !strings.HasSuffix(urlPrefix, "/") {
		urlPrefix += "/"
	}
	return &LocalBackend{root: root, urlPrefix: urlPrefix}
}

func (b *LocalBackend) Save(ctx context.Context, filename string, r io.Reader) (Object, error) {
	rel := filepath.Join(time.Now().Format("2006/01/02"), randomName()+strings.ToLower(filepath.Ext(filename)))
	full := filepath.Join(b.root, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return Object{}, err
	}

	out, err := os.OpenFile(full, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return Object{}, err
	}
	defer out.Close()

	size, err := io.Copy(out, readerWithContext{ctx: ctx, reader: r})
	if err != nil {
		_ = os.Remove(full)
		return Object{}, err
	}
	return Object{Path: filepath.ToSlash(rel), Size: size, DownloadURL: b.PublicURL(rel)}, nil
}

func (b *LocalBackend) Delete(ctx context.Context, path string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	if path == "" {
		return nil
	}
	full, err := b.safePath(path)
	if err != nil {
		return err
	}
	return os.Remove(full)
}

func (b *LocalBackend) PublicURL(path string) string {
	return b.urlPrefix + strings.TrimLeft(filepath.ToSlash(path), "/")
}

func (b *LocalBackend) safePath(path string) (string, error) {
	root, err := filepath.Abs(b.root)
	if err != nil {
		return "", err
	}
	clean := filepath.Clean(strings.TrimLeft(filepath.ToSlash(path), "/"))
	full, err := filepath.Abs(filepath.Join(root, filepath.FromSlash(clean)))
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(root, full)
	if err != nil {
		return "", err
	}
	if rel == "." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".." || filepath.IsAbs(rel) {
		return "", fmt.Errorf("storage path escapes root: %q", path)
	}
	return full, nil
}

type readerWithContext struct {
	ctx    context.Context
	reader io.Reader
}

func (r readerWithContext) Read(p []byte) (int, error) {
	select {
	case <-r.ctx.Done():
		return 0, r.ctx.Err()
	default:
		return r.reader.Read(p)
	}
}

func randomName() string {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return time.Now().Format("150405.000000000")
	}
	return hex.EncodeToString(buf[:])
}
