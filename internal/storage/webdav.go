package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"strings"
	"time"
)

type WebDAVBackend struct {
	baseURL   string
	username  string
	password  string
	publicURL string
	client    *http.Client
}

func NewWebDAVBackend(baseURL, username, password, publicURL string) *WebDAVBackend {
	baseURL = strings.TrimRight(baseURL, "/")
	publicURL = strings.TrimRight(publicURL, "/")
	if publicURL == "" {
		publicURL = baseURL
	}
	return &WebDAVBackend{
		baseURL:   baseURL,
		username:  username,
		password:  password,
		publicURL: publicURL,
		client:    &http.Client{Timeout: 60 * time.Second},
	}
}

func (b *WebDAVBackend) Save(ctx context.Context, filename string, r io.Reader) (Object, error) {
	rel := path.Join(time.Now().Format("2006/01/02"), randomName()+strings.ToLower(filepath.Ext(filename)))
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
		return b.baseURL + "/" + strings.TrimLeft(objectPath, "/")
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/" + strings.TrimLeft(path.Clean(objectPath), "/")
	return parsed.String()
}

func (b *WebDAVBackend) auth(req *http.Request) {
	if b.username != "" || b.password != "" {
		req.SetBasicAuth(b.username, b.password)
	}
}
