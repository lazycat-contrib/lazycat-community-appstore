package lpkinspect

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"lazycat.community/appstore/internal/mirror"
	"lazycat.community/appstore/internal/storage"
)

const (
	DefaultInspectionTotalTimeout = 90 * time.Second
	DefaultFetchCandidateTimeout  = 8 * time.Second
)

type Inspection struct {
	Metadata Metadata
	SHA256   string
	Size     int64
}

type URLOptions struct {
	MaxBytes          int64
	UseMirrorDownload bool
	Mirrors           []mirror.Entry
	AllowPrivateHosts bool
	TotalTimeout      time.Duration
	CandidateTimeout  time.Duration
}

func ParseUploaded(file multipart.File, header *multipart.FileHeader, maxBytes int64) (Metadata, error) {
	if strings.ToLower(filepath.Ext(header.Filename)) != ".lpk" {
		return Metadata{}, storage.ErrInvalidLPK
	}
	if maxBytes > 0 && header.Size > maxBytes {
		return Metadata{}, storage.ErrTooLarge
	}
	meta, err := parseLPKReaderAt(context.Background(), file, header.Size, maxBytes)
	if _, seekErr := file.Seek(0, io.SeekStart); seekErr != nil && err == nil {
		err = seekErr
	}
	return meta, err
}

func InspectURL(ctx context.Context, rawURL string, opts URLOptions) (Inspection, error) {
	opts = opts.withDefaults()
	if opts.MaxBytes <= 0 {
		return Inspection{}, errors.New("max LPK size must be positive")
	}

	ctx, cancel := context.WithTimeout(ctx, opts.TotalTimeout)
	defer cancel()
	candidates, err := FetchURLs(rawURL, opts.UseMirrorDownload, opts.Mirrors)
	if err != nil {
		return Inspection{}, err
	}
	failures := make([]string, 0, len(candidates))
	for _, parsed := range candidates {
		if err := ctx.Err(); err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", parsed.Host, err))
			break
		}
		candidateCtx, cancelCandidate := context.WithTimeout(ctx, opts.CandidateTimeout)
		inspected, err := inspectFetchCandidate(candidateCtx, parsed, opts)
		cancelCandidate()
		if err == nil {
			return inspected, nil
		}
		failures = append(failures, fmt.Sprintf("%s: %v", parsed.Host, err))
	}
	return Inspection{}, fmt.Errorf("could not fetch LPK URL after trying %d candidate(s): %s", len(candidates), summarizeFetchFailures(failures))
}

func FetchURLs(rawURL string, useMirrorDownload bool, mirrors []mirror.Entry) ([]*url.URL, error) {
	normalized := NormalizeGitHubRawURL(rawURL)
	parsed, err := ParseHTTPDownloadURL(normalized)
	if err != nil {
		return nil, err
	}
	candidates := []*url.URL{}
	seen := map[string]struct{}{}
	addCandidate := func(raw string) error {
		candidate, err := ParseHTTPDownloadURL(raw)
		if err != nil {
			return err
		}
		key := candidate.String()
		if _, ok := seen[key]; ok {
			return nil
		}
		seen[key] = struct{}{}
		candidates = append(candidates, candidate)
		return nil
	}
	if useMirrorDownload {
		for _, entry := range GitHubMirrorsForURL(mirrors, parsed.String()) {
			if err := addCandidate(mirror.RewriteGitHub(parsed.String(), entry)); err != nil {
				return nil, err
			}
		}
	}
	if err := addCandidate(parsed.String()); err != nil {
		return nil, err
	}
	return candidates, nil
}

func ParseHTTPDownloadURL(rawURL string) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Host == "" {
		return nil, errors.New("downloadUrl must be a valid URL")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, errors.New("downloadUrl must use http or https")
	}
	return parsed, nil
}

func FirstGitHubMirrorForURL(entries []mirror.Entry, rawURL string) (mirror.Entry, bool) {
	mirrors := GitHubMirrorsForURL(entries, rawURL)
	if len(mirrors) == 0 {
		return mirror.Entry{}, false
	}
	return mirrors[0], true
}

func GitHubMirrorsForURL(entries []mirror.Entry, rawURL string) []mirror.Entry {
	kind := mirror.KindForURL(rawURL)
	if kind == "" {
		return nil
	}
	out := []mirror.Entry{}
	for _, entry := range entries {
		if entry.Kind == kind {
			out = append(out, entry)
		}
	}
	return out
}

func NormalizeGitHubRawURL(rawURL string) string {
	trimmed := strings.TrimSpace(rawURL)
	parsed, err := url.Parse(trimmed)
	if err != nil || !strings.EqualFold(parsed.Hostname(), "github.com") {
		return trimmed
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) < 5 || parts[2] != "raw" {
		return trimmed
	}
	normalized := &url.URL{
		Scheme:   parsed.Scheme,
		Host:     "raw.githubusercontent.com",
		Path:     "/" + strings.Join(append(parts[:2], parts[3:]...), "/"),
		RawQuery: parsed.RawQuery,
		Fragment: parsed.Fragment,
	}
	if normalized.Scheme == "" {
		normalized.Scheme = "https"
	}
	return normalized.String()
}

func ValidateURLHost(ctx context.Context, parsed *url.URL, allowPrivate bool) error {
	if allowPrivate {
		return nil
	}
	host := strings.TrimSpace(parsed.Hostname())
	if host == "" {
		return errors.New("downloadUrl host is required")
	}
	if strings.EqualFold(host, "localhost") {
		return errors.New("downloadUrl host must not resolve to a private or local address")
	}
	if ip := net.ParseIP(host); ip != nil {
		if UnsafeIP(ip) {
			return errors.New("downloadUrl host must not resolve to a private or local address")
		}
		return nil
	}
	addrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return fmt.Errorf("could not resolve downloadUrl host: %w", err)
	}
	if len(addrs) == 0 {
		return errors.New("downloadUrl host did not resolve")
	}
	for _, addr := range addrs {
		if UnsafeIP(addr.IP) {
			return errors.New("downloadUrl host must not resolve to a private or local address")
		}
	}
	return nil
}

func UnsafeIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if ip4 := ip.To4(); ip4 != nil {
		ip = ip4
	}
	return ip.IsLoopback() ||
		ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsMulticast() ||
		ip.IsUnspecified() ||
		!ip.IsGlobalUnicast()
}

func inspectFetchCandidate(ctx context.Context, parsed *url.URL, opts URLOptions) (Inspection, error) {
	if err := ValidateURLHost(ctx, parsed, opts.AllowPrivateHosts); err != nil {
		return Inspection{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return Inspection{}, err
	}
	client := &http.Client{
		Timeout: 30 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return errors.New("too many redirects")
			}
			if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
				return errors.New("redirect target must use http or https")
			}
			if err := ValidateURLHost(req.Context(), req.URL, opts.AllowPrivateHosts); err != nil {
				return err
			}
			return nil
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		return Inspection{}, fmt.Errorf("could not fetch LPK URL: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Inspection{}, fmt.Errorf("LPK URL returned HTTP %d", resp.StatusCode)
	}
	if resp.ContentLength > opts.MaxBytes {
		return Inspection{}, storage.ErrTooLarge
	}

	tmp, err := os.CreateTemp("", "lazycat-appstore-lpk-*.lpk")
	if err != nil {
		return Inspection{}, err
	}
	tmpName := tmp.Name()
	defer func() {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
	}()

	hasher := sha256.New()
	written, err := io.Copy(io.MultiWriter(tmp, hasher), io.LimitReader(resp.Body, opts.MaxBytes+1))
	if err != nil {
		return Inspection{}, err
	}
	if written > opts.MaxBytes {
		return Inspection{}, storage.ErrTooLarge
	}
	if err := tmp.Close(); err != nil {
		return Inspection{}, err
	}
	meta, err := parseLPKFile(ctx, tmpName, opts.MaxBytes)
	if err != nil {
		return Inspection{}, err
	}
	return Inspection{
		Metadata: meta,
		SHA256:   hex.EncodeToString(hasher.Sum(nil)),
		Size:     written,
	}, nil
}

func (opts URLOptions) withDefaults() URLOptions {
	if opts.TotalTimeout <= 0 {
		opts.TotalTimeout = DefaultInspectionTotalTimeout
	}
	if opts.CandidateTimeout <= 0 {
		opts.CandidateTimeout = DefaultFetchCandidateTimeout
	}
	return opts
}

func summarizeFetchFailures(failures []string) string {
	const maxVisible = 4
	if len(failures) <= maxVisible {
		return strings.Join(failures, "; ")
	}
	return strings.Join(failures[:maxVisible], "; ") + fmt.Sprintf("; ... %d more", len(failures)-maxVisible)
}
