package storage

import (
	"context"
	"io"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type S3Backend struct {
	client    *minio.Client
	bucket    string
	publicURL string
}

type S3Options struct {
	Endpoint  string
	Bucket    string
	AccessKey string
	SecretKey string
	UseSSL    bool
	PublicURL string
}

func NewS3Backend(options S3Options) (*S3Backend, error) {
	client, err := minio.New(options.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(options.AccessKey, options.SecretKey, ""),
		Secure: options.UseSSL,
	})
	if err != nil {
		return nil, err
	}
	return &S3Backend{
		client:    client,
		bucket:    options.Bucket,
		publicURL: strings.TrimRight(options.PublicURL, "/"),
	}, nil
}

func (b *S3Backend) Save(ctx context.Context, filename string, r io.Reader) (Object, error) {
	rel := path.Join(time.Now().Format("2006/01/02"), randomName()+strings.ToLower(filepath.Ext(filename)))
	info, err := b.client.PutObject(ctx, b.bucket, rel, r, -1, minio.PutObjectOptions{ContentType: "application/octet-stream"})
	if err != nil {
		return Object{}, err
	}
	return Object{Path: rel, Size: info.Size, DownloadURL: b.PublicURL(rel)}, nil
}

func (b *S3Backend) Delete(ctx context.Context, objectPath string) error {
	return b.client.RemoveObject(ctx, b.bucket, objectPath, minio.RemoveObjectOptions{})
}

func (b *S3Backend) PublicURL(objectPath string) string {
	if b.publicURL != "" {
		return b.publicURL + "/" + strings.TrimLeft(path.Clean(objectPath), "/")
	}
	return "/" + strings.TrimLeft(path.Clean(objectPath), "/")
}
