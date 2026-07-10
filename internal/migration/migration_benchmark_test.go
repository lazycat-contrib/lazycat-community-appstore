package migration

import (
	"context"
	"errors"
	"fmt"
	"io"
	"testing"

	"lazycat.community/appstore/internal/storage"
)

func BenchmarkMigrationAttachmentStreaming(b *testing.B) {
	for _, size := range []int64{1 << 20, 32 << 20} {
		b.Run(fmt.Sprintf("bytes_%d", size), func(b *testing.B) {
			exporter := newAttachmentBenchmarkExporter(b, size)
			b.ReportAllocs()
			for b.Loop() {
				if _, err := exporter.Export(b.Context(), io.Discard, Options{IncludeApps: true, IncludeFiles: true}); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

type attachmentBenchmarkBackend struct {
	size int64
}

func (b attachmentBenchmarkBackend) Open(context.Context, string) (storage.Reader, error) {
	return storage.Reader{
		Body: io.NopCloser(io.LimitReader(zeroReader{}, b.size)),
		Size: b.size,
	}, nil
}

func (attachmentBenchmarkBackend) Save(context.Context, string, io.Reader) (storage.Object, error) {
	return storage.Object{}, errors.New("benchmark backend is read-only")
}

func (attachmentBenchmarkBackend) Delete(context.Context, string) error { return nil }
func (attachmentBenchmarkBackend) PublicURL(string) string              { return "" }

func newAttachmentBenchmarkExporter(b *testing.B, size int64) *Exporter {
	b.Helper()
	db := newMigrationTestDB(b)
	seedMigrationData(b, db)
	backend := attachmentBenchmarkBackend{size: size}
	resolver := StorageResolverFunc(func(context.Context, string) (storage.Backend, error) {
		return backend, nil
	})
	return NewExporter(db, resolver, "benchmark")
}
