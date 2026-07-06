package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"
)

var (
	ErrInvalidLPK = errors.New("only .lpk files are supported")
	ErrTooLarge   = errors.New("lpk file exceeds configured size limit")
)

type Object struct {
	Path        string
	DownloadURL string
	Size        int64
	SHA256      string
}

type Reader struct {
	Body        io.ReadCloser
	Name        string
	Size        int64
	ModTime     time.Time
	ContentType string
}

type Backend interface {
	Save(ctx context.Context, filename string, r io.Reader) (Object, error)
	Delete(ctx context.Context, path string) error
	PublicURL(path string) string
	Open(ctx context.Context, path string) (Reader, error)
}

func SaveLPK(ctx context.Context, backend Backend, r io.Reader, filename string, maxBytes int64) (Object, error) {
	if strings.ToLower(filepath.Ext(filename)) != ".lpk" {
		return Object{}, ErrInvalidLPK
	}
	return SaveFile(ctx, backend, r, filename, maxBytes)
}

func SaveFile(ctx context.Context, backend Backend, r io.Reader, filename string, maxBytes int64) (Object, error) {
	if maxBytes <= 0 {
		return Object{}, fmt.Errorf("maxBytes must be positive")
	}

	hasher := sha256.New()
	limited := &limitedHashReader{
		reader:    r,
		hasher:    hasher,
		remaining: maxBytes + 1,
	}

	obj, err := backend.Save(ctx, filename, limited)
	if err != nil {
		return Object{}, err
	}
	if limited.total > maxBytes {
		_ = backend.Delete(ctx, obj.Path)
		return Object{}, ErrTooLarge
	}

	obj.Size = limited.total
	obj.SHA256 = hex.EncodeToString(hasher.Sum(nil))
	obj.DownloadURL = backend.PublicURL(obj.Path)
	return obj, nil
}

type limitedHashReader struct {
	reader    io.Reader
	hasher    io.Writer
	remaining int64
	total     int64
}

func (r *limitedHashReader) Read(p []byte) (int, error) {
	if r.remaining <= 0 {
		return 0, io.EOF
	}
	if int64(len(p)) > r.remaining {
		p = p[:r.remaining]
	}
	n, err := r.reader.Read(p)
	if n > 0 {
		r.remaining -= int64(n)
		r.total += int64(n)
		_, _ = r.hasher.Write(p[:n])
	}
	return n, err
}
