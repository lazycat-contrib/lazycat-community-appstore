package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"strings"
	"time"
)

type WebDAVBackend struct {
	baseURL    string
	username   string
	password   string
	rootPrefix string
	publicURL  string
	client     *http.Client
}

func NewWebDAVBackend(baseURL, username, password, publicURL string, rootPrefix ...string) *WebDAVBackend {
	baseURL = strings.TrimRight(baseURL, "/")
	publicURL = strings.TrimRight(publicURL, "/")
	if publicURL == "" {
		publicURL = baseURL
	}
	prefix := ""
	if len(rootPrefix) > 0 {
		prefix = cleanObjectPrefix(rootPrefix[0])
	}
	return &WebDAVBackend{
		baseURL:    baseURL,
		username:   username,
		password:   password,
		rootPrefix: prefix,
		publicURL:  publicURL,
		client:     &http.Client{Timeout: 60 * time.Second},
	}
}

func (b *WebDAVBackend) Save(ctx context.Context, filename string, r io.Reader) (Object, error) {
	rel := path.Join(time.Now().Format("2006/01/02"), randomName()+strings.ToLower(filepath.Ext(filename)))
	return b.saveAt(ctx, rel, r)
}

func (b *WebDAVBackend) SaveObject(ctx context.Context, objectPath string, r io.Reader) (Object, error) {
	rel, err := CleanObjectPath(objectPath)
	if err != nil {
		return Object{}, err
	}
	return b.saveAt(ctx, rel, r)
}

func (b *WebDAVBackend) saveAt(ctx context.Context, rel string, r io.Reader) (Object, error) {
	body, err := io.ReadAll(r)
	if err != nil {
		return Object{}, err
	}
	if err := b.mkcol(ctx, path.Dir(rel)); err != nil {
		return Object{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, b.objectURL(rel), bytes.NewReader(body))
	if err != nil {
		return Object{}, err
	}
	b.auth(req)
	resp, err := b.client.Do(req)
	if err != nil {
		return Object{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Object{}, fmt.Errorf("webdav put failed: %s", resp.Status)
	}
	return Object{Path: rel, Size: int64(len(body)), DownloadURL: b.PublicURL(rel)}, nil
}

func (b *WebDAVBackend) Delete(ctx context.Context, objectPath string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, b.objectURL(objectPath), nil)
	if err != nil {
		return err
	}
	b.auth(req)
	resp, err := b.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound || (resp.StatusCode >= 200 && resp.StatusCode < 300) {
		return nil
	}
	return fmt.Errorf("webdav delete failed: %s", resp.Status)
}

func (b *WebDAVBackend) PublicURL(objectPath string) string {
	return strings.TrimRight(b.publicURL, "/") + "/" + strings.TrimLeft(path.Clean(objectPath), "/")
}

func (b *WebDAVBackend) Open(ctx context.Context, objectPath string) (Reader, error) {
	cleaned := strings.TrimLeft(path.Clean(objectPath), "/")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, b.objectURL(cleaned), nil)
	if err != nil {
		return Reader{}, err
	}
	b.auth(req)
	resp, err := b.client.Do(req)
	if err != nil {
		return Reader{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_ = resp.Body.Close()
		return Reader{}, fmt.Errorf("webdav get failed: %s", resp.Status)
	}
	modTime := time.Now()
	if raw := resp.Header.Get("Last-Modified"); raw != "" {
		if parsed, err := http.ParseTime(raw); err == nil {
			modTime = parsed
		}
	}
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = mime.TypeByExtension(strings.ToLower(path.Ext(cleaned)))
	}
	return Reader{
		Body:        resp.Body,
		Name:        path.Base(cleaned),
		Size:        resp.ContentLength,
		ModTime:     modTime,
		ContentType: contentType,
	}, nil
}

func (b *WebDAVBackend) mkcol(ctx context.Context, dir string) error {
	if dir == "." || dir == "/" || dir == "" {
		return nil
	}
	parts := strings.Split(dir, "/")
	current := ""
	for _, part := range parts {
		if part == "" {
			continue
		}
		current = path.Join(current, part)
		req, err := http.NewRequestWithContext(ctx, "MKCOL", b.objectURL(current), nil)
		if err != nil {
			return err
		}
		b.auth(req)
		resp, err := b.client.Do(req)
		if err != nil {
			return err
		}
		_ = resp.Body.Close()
		if resp.StatusCode == http.StatusMethodNotAllowed || resp.StatusCode == http.StatusConflict || resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusOK {
			continue
		}
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			continue
		}
		return fmt.Errorf("webdav mkcol failed: %s", resp.Status)
	}
	return nil
}

func (b *WebDAVBackend) objectURL(objectPath string) string {
	parsed, err := url.Parse(b.baseURL)
	if err != nil {
		return b.baseURL + "/" + strings.TrimLeft(b.objectPath(objectPath), "/")
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/" + strings.TrimLeft(b.objectPath(objectPath), "/")
	return parsed.String()
}

func (b *WebDAVBackend) objectPath(objectPath string) string {
	cleaned := strings.TrimLeft(path.Clean(objectPath), "/")
	if b.rootPrefix == "" {
		return cleaned
	}
	if cleaned == "" || cleaned == "." {
		return b.rootPrefix
	}
	return path.Join(b.rootPrefix, cleaned)
}

func (b *WebDAVBackend) auth(req *http.Request) {
	if b.username != "" || b.password != "" {
		req.SetBasicAuth(b.username, b.password)
	}
}
