package storage

import (
	"context"
	"fmt"
	"io"
)

type ExternalLinkBackend struct {
	name string
}

func NewExternalLinkBackend(name string) *ExternalLinkBackend {
	return &ExternalLinkBackend{name: name}
}

func (b *ExternalLinkBackend) Save(ctx context.Context, filename string, r io.Reader) (Object, error) {
	select {
	case <-ctx.Done():
		return Object{}, ctx.Err()
	default:
	}
	return Object{}, fmt.Errorf("%s storage accepts external download URLs only", b.name)
}

func (b *ExternalLinkBackend) Delete(ctx context.Context, path string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

func (b *ExternalLinkBackend) PublicURL(path string) string {
	return path
}

func (b *ExternalLinkBackend) Open(ctx context.Context, path string) (Reader, error) {
	select {
	case <-ctx.Done():
		return Reader{}, ctx.Err()
	default:
	}
	return Reader{}, fmt.Errorf("%s storage cannot proxy external download URLs", b.name)
}
