package storage

import (
	"context"
	"io"
	"mime"
	"net/url"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
)

type S3Backend struct {
	client     *awss3.Client
	bucket     string
	rootPrefix string
	publicURL  string
}

type S3Options struct {
	Endpoint   string
	Bucket     string
	Region     string
	AccessKey  string
	SecretKey  string
	UseSSL     bool
	PathStyle  bool
	RootPrefix string
	PublicURL  string
}

func NewS3Backend(options S3Options) (*S3Backend, error) {
	endpoint := normalizeS3Endpoint(options.Endpoint, options.UseSSL)
	region := strings.TrimSpace(options.Region)
	if region == "" {
		region = "auto"
	}
	cfg := aws.Config{
		Region: region,
		Credentials: aws.NewCredentialsCache(credentials.NewStaticCredentialsProvider(
			options.AccessKey,
			options.SecretKey,
			"",
		)),
	}
	client := awss3.NewFromConfig(cfg, func(s3Options *awss3.Options) {
		if endpoint != "" {
			s3Options.BaseEndpoint = aws.String(endpoint)
		}
		s3Options.UsePathStyle = options.PathStyle
	})
	return &S3Backend{
		client:     client,
		bucket:     options.Bucket,
		rootPrefix: cleanObjectPrefix(options.RootPrefix),
		publicURL:  strings.TrimRight(options.PublicURL, "/"),
	}, nil
}

func (b *S3Backend) Save(ctx context.Context, filename string, r io.Reader) (Object, error) {
	rel := path.Join(time.Now().Format("2006/01/02"), randomName()+strings.ToLower(filepath.Ext(filename)))
	return b.saveAt(ctx, rel, r)
}

func (b *S3Backend) SaveObject(ctx context.Context, objectPath string, r io.Reader) (Object, error) {
	rel, err := CleanObjectPath(objectPath)
	if err != nil {
		return Object{}, err
	}
	return b.saveAt(ctx, rel, r)
}

func (b *S3Backend) saveAt(ctx context.Context, rel string, r io.Reader) (Object, error) {
	key := b.objectKey(rel)
	contentType := mime.TypeByExtension(strings.ToLower(path.Ext(rel)))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	if _, err := b.client.PutObject(ctx, &awss3.PutObjectInput{
		Bucket:      aws.String(b.bucket),
		Key:         aws.String(key),
		Body:        r,
		ContentType: aws.String(contentType),
	}); err != nil {
		return Object{}, err
	}
	return Object{Path: rel, DownloadURL: b.PublicURL(rel)}, nil
}

func (b *S3Backend) Delete(ctx context.Context, objectPath string) error {
	_, err := b.client.DeleteObject(ctx, &awss3.DeleteObjectInput{
		Bucket: aws.String(b.bucket),
		Key:    aws.String(b.objectKey(objectPath)),
	})
	return err
}

func (b *S3Backend) ListObjects(ctx context.Context, prefix string) ([]ObjectInfo, error) {
	cleanedPrefix := cleanObjectPrefix(prefix)
	keyPrefix := b.objectKey(cleanedPrefix)
	if keyPrefix != "" && !strings.HasSuffix(keyPrefix, "/") {
		keyPrefix += "/"
	}
	paginator := awss3.NewListObjectsV2Paginator(b.client, &awss3.ListObjectsV2Input{
		Bucket: aws.String(b.bucket),
		Prefix: aws.String(keyPrefix),
	})
	objects := []ObjectInfo{}
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, item := range page.Contents {
			rel, ok := b.relativeObjectPath(aws.ToString(item.Key))
			if !ok {
				continue
			}
			modTime := time.Time{}
			if item.LastModified != nil {
				modTime = *item.LastModified
			}
			size := int64(0)
			if item.Size != nil {
				size = *item.Size
			}
			objects = append(objects, ObjectInfo{Path: rel, Size: size, ModTime: modTime})
		}
	}
	return objects, nil
}

func (b *S3Backend) PublicURL(objectPath string) string {
	if b.publicURL != "" {
		return b.publicURL + "/" + strings.TrimLeft(path.Clean(objectPath), "/")
	}
	return "/" + strings.TrimLeft(path.Clean(objectPath), "/")
}

func (b *S3Backend) Open(ctx context.Context, objectPath string) (Reader, error) {
	cleaned := strings.TrimLeft(path.Clean(objectPath), "/")
	key := b.objectKey(cleaned)
	info, err := b.client.HeadObject(ctx, &awss3.HeadObjectInput{
		Bucket: aws.String(b.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return Reader{}, err
	}
	object, err := b.client.GetObject(ctx, &awss3.GetObjectInput{
		Bucket: aws.String(b.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return Reader{}, err
	}
	contentType := aws.ToString(info.ContentType)
	if contentType == "" {
		contentType = mime.TypeByExtension(strings.ToLower(path.Ext(cleaned)))
	}
	modTime := time.Now()
	if info.LastModified != nil {
		modTime = *info.LastModified
	}
	size := int64(-1)
	if info.ContentLength != nil {
		size = *info.ContentLength
	}
	return Reader{
		Body:        object.Body,
		Name:        path.Base(cleaned),
		Size:        size,
		ModTime:     modTime,
		ContentType: contentType,
	}, nil
}

func (b *S3Backend) objectKey(objectPath string) string {
	cleaned := strings.TrimLeft(path.Clean(objectPath), "/")
	if b.rootPrefix == "" {
		return cleaned
	}
	if cleaned == "" || cleaned == "." {
		return b.rootPrefix
	}
	return path.Join(b.rootPrefix, cleaned)
}

func (b *S3Backend) relativeObjectPath(key string) (string, bool) {
	cleaned := strings.TrimLeft(path.Clean(key), "/")
	if cleaned == "" || cleaned == "." {
		return "", false
	}
	if b.rootPrefix == "" {
		return cleaned, true
	}
	root := strings.TrimRight(b.rootPrefix, "/")
	if cleaned == root {
		return "", false
	}
	prefix := root + "/"
	if !strings.HasPrefix(cleaned, prefix) {
		return "", false
	}
	rel := strings.TrimPrefix(cleaned, prefix)
	if rel == "" || rel == "." {
		return "", false
	}
	return rel, true
}

func normalizeS3Endpoint(endpoint string, useSSL bool) string {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return ""
	}
	if parsed, err := url.Parse(endpoint); err == nil && parsed.Scheme != "" {
		return strings.TrimRight(endpoint, "/")
	}
	scheme := "http"
	if useSSL {
		scheme = "https"
	}
	return scheme + "://" + strings.TrimRight(endpoint, "/")
}

func cleanObjectPrefix(prefix string) string {
	prefix = strings.Trim(strings.TrimSpace(prefix), "/")
	if prefix == "." {
		return ""
	}
	return prefix
}
