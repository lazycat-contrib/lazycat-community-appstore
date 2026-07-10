package storage

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"strconv"
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
	if err := b.mkcol(ctx, path.Dir(rel)); err != nil {
		return Object{}, err
	}
	body := &countingReader{reader: r}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, b.objectURL(rel), body)
	if err != nil {
		return Object{}, err
	}
	b.auth(req)
	resp, err := b.client.Do(req)
	if err != nil {
		return Object{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Object{}, fmt.Errorf("webdav put failed: %s", resp.Status)
	}
	return Object{Path: rel, Size: body.total, DownloadURL: b.PublicURL(rel)}, nil
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
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusNotFound || (resp.StatusCode >= 200 && resp.StatusCode < 300) {
		return nil
	}
	return fmt.Errorf("webdav delete failed: %s", resp.Status)
}

func (b *WebDAVBackend) ListObjects(ctx context.Context, prefix string) ([]ObjectInfo, error) {
	cleanedPrefix := cleanObjectPrefix(prefix)
	body := bytes.NewBufferString(`<?xml version="1.0" encoding="utf-8"?><d:propfind xmlns:d="DAV:"><d:prop><d:getlastmodified/><d:getcontentlength/><d:resourcetype/></d:prop></d:propfind>`)
	req, err := http.NewRequestWithContext(ctx, "PROPFIND", b.objectURL(cleanedPrefix), body)
	if err != nil {
		return nil, err
	}
	b.auth(req)
	req.Header.Set("Depth", "1")
	req.Header.Set("Content-Type", "application/xml; charset=utf-8")
	resp, err := b.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("webdav propfind failed: %s", resp.Status)
	}

	var listing webDAVMultiStatus
	if err := xml.NewDecoder(resp.Body).Decode(&listing); err != nil {
		return nil, err
	}
	objects := make([]ObjectInfo, 0, len(listing.Responses))
	for _, response := range listing.Responses {
		prop, ok := response.okProp()
		if !ok || prop.ResourceType.Collection != nil {
			continue
		}
		rel, ok := b.relativeObjectPathFromHref(response.Href, cleanedPrefix)
		if !ok || rel == "" || rel == cleanedPrefix {
			continue
		}
		size := int64(0)
		if raw := strings.TrimSpace(prop.ContentLength); raw != "" {
			if parsed, err := strconv.ParseInt(raw, 10, 64); err == nil {
				size = parsed
			}
		}
		modTime := time.Time{}
		if raw := strings.TrimSpace(prop.LastModified); raw != "" {
			if parsed, err := http.ParseTime(raw); err == nil {
				modTime = parsed
			}
		}
		objects = append(objects, ObjectInfo{Path: rel, Size: size, ModTime: modTime})
	}
	return objects, nil
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

func (b *WebDAVBackend) relativeObjectPathFromHref(href, fallbackPrefix string) (string, bool) {
	href = strings.TrimSpace(href)
	if href == "" {
		return "", false
	}
	parsedHref, err := url.Parse(href)
	if err == nil && !parsedHref.IsAbs() && !strings.HasPrefix(parsedHref.Path, "/") {
		rel := cleanObjectPrefix(parsedHref.Path)
		if rel == "" {
			return "", true
		}
		if fallbackPrefix != "" {
			return path.Join(fallbackPrefix, rel), true
		}
		return rel, true
	}
	if err != nil {
		return "", false
	}
	hrefPath := parsedHref.Path
	if unescaped, err := url.PathUnescape(hrefPath); err == nil {
		hrefPath = unescaped
	}
	hrefPath = path.Clean("/" + strings.TrimLeft(hrefPath, "/"))

	base, err := url.Parse(b.baseURL)
	if err != nil {
		return "", false
	}
	basePath := base.Path
	if unescaped, err := url.PathUnescape(basePath); err == nil {
		basePath = unescaped
	}
	rootPath := path.Clean("/" + strings.TrimLeft(path.Join(basePath, b.rootPrefix), "/"))
	if rootPath == "/" {
		rel := strings.TrimLeft(hrefPath, "/")
		if rel == "" || rel == "." {
			return "", true
		}
		return path.Clean(rel), true
	}
	if hrefPath == rootPath {
		return "", true
	}
	prefix := strings.TrimRight(rootPath, "/") + "/"
	if !strings.HasPrefix(hrefPath, prefix) {
		return "", false
	}
	rel := strings.TrimPrefix(hrefPath, prefix)
	if rel == "" || rel == "." {
		return "", true
	}
	return path.Clean(rel), true
}

func (b *WebDAVBackend) auth(req *http.Request) {
	if b.username != "" || b.password != "" {
		req.SetBasicAuth(b.username, b.password)
	}
}

type webDAVMultiStatus struct {
	Responses []webDAVResponse `xml:"response"`
}

type webDAVResponse struct {
	Href      string           `xml:"href"`
	PropStats []webDAVPropStat `xml:"propstat"`
}

type webDAVPropStat struct {
	Status string     `xml:"status"`
	Prop   webDAVProp `xml:"prop"`
}

type webDAVProp struct {
	LastModified  string             `xml:"getlastmodified"`
	ContentLength string             `xml:"getcontentlength"`
	ResourceType  webDAVResourceType `xml:"resourcetype"`
}

type webDAVResourceType struct {
	Collection *struct{} `xml:"collection"`
}

func (r webDAVResponse) okProp() (webDAVProp, bool) {
	for _, propStat := range r.PropStats {
		status := strings.TrimSpace(propStat.Status)
		if status == "" || strings.Contains(status, " 200 ") || strings.HasSuffix(status, " 200") || strings.HasSuffix(status, " 200 OK") {
			return propStat.Prop, true
		}
	}
	return webDAVProp{}, false
}

type countingReader struct {
	reader io.Reader
	total  int64
}

func (r *countingReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	r.total += int64(n)
	return n, err
}
