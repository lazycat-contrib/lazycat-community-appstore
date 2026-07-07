package server

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

	"lazycat.community/appstore/internal/lpkmeta"
	"lazycat.community/appstore/internal/mirror"
	"lazycat.community/appstore/internal/storage"
)

type lpkInspection struct {
	Metadata lpkmeta.Metadata
	SHA256   string
	Size     int64
}

var (
	lpkInspectionTotalTimeout = 90 * time.Second
	lpkFetchCandidateTimeout  = 8 * time.Second
)

func parseUploadedLPKMetadata(file multipart.File, header *multipart.FileHeader, maxBytes int64) (lpkmeta.Metadata, error) {
	if strings.ToLower(filepath.Ext(header.Filename)) != ".lpk" {
		return lpkmeta.Metadata{}, storage.ErrInvalidLPK
	}
	if maxBytes > 0 && header.Size > maxBytes {
		return lpkmeta.Metadata{}, storage.ErrTooLarge
	}
	meta, err := lpkmeta.ParseReaderAt(file, header.Size)
	if _, seekErr := file.Seek(0, io.SeekStart); seekErr != nil && err == nil {
		err = seekErr
	}
	return meta, err
}

func (s *Server) inspectLPKURL(ctx context.Context, rawURL string, maxBytes int64, useMirrorDownload bool) (lpkInspection, error) {
	if maxBytes <= 0 {
		return lpkInspection{}, errors.New("max LPK size must be positive")
	}

	ctx, cancel := context.WithTimeout(ctx, lpkInspectionTotalTimeout)
	defer cancel()
	candidates, err := s.lpkFetchURLs(ctx, rawURL, useMirrorDownload)
	if err != nil {
		return lpkInspection{}, err
	}
	failures := make([]string, 0, len(candidates))
	for _, parsed := range candidates {
		if err := ctx.Err(); err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", parsed.Host, err))
			break
		}
		candidateCtx, cancelCandidate := context.WithTimeout(ctx, lpkFetchCandidateTimeout)
		inspected, err := s.inspectLPKFetchCandidate(candidateCtx, parsed, maxBytes)
		cancelCandidate()
		if err == nil {
			return inspected, nil
		}
		failures = append(failures, fmt.Sprintf("%s: %v", parsed.Host, err))
	}
	return lpkInspection{}, fmt.Errorf("could not fetch LPK URL after trying %d candidate(s): %s", len(candidates), summarizeLPKFetchFailures(failures))
}

func (s *Server) inspectLPKFetchCandidate(ctx context.Context, parsed *url.URL, maxBytes int64) (lpkInspection, error) {
	if err := validateLPKURLHost(ctx, parsed, s.allowPrivateLPKURLHosts); err != nil {
		return lpkInspection{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return lpkInspection{}, err
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
			if err := validateLPKURLHost(req.Context(), req.URL, s.allowPrivateLPKURLHosts); err != nil {
				return err
			}
			return nil
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		return lpkInspection{}, fmt.Errorf("could not fetch LPK URL: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return lpkInspection{}, fmt.Errorf("LPK URL returned HTTP %d", resp.StatusCode)
	}
	if resp.ContentLength > maxBytes {
		return lpkInspection{}, storage.ErrTooLarge
	}

	tmp, err := os.CreateTemp("", "lazycat-appstore-lpk-*.lpk")
	if err != nil {
		return lpkInspection{}, err
	}
	tmpName := tmp.Name()
	defer func() {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
	}()

	hasher := sha256.New()
	written, err := io.Copy(io.MultiWriter(tmp, hasher), io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return lpkInspection{}, err
	}
	if written > maxBytes {
		return lpkInspection{}, storage.ErrTooLarge
	}
	if err := tmp.Close(); err != nil {
		return lpkInspection{}, err
	}
	meta, err := lpkmeta.ParseFile(tmpName)
	if err != nil {
		return lpkInspection{}, err
	}
	return lpkInspection{
		Metadata: meta,
		SHA256:   hex.EncodeToString(hasher.Sum(nil)),
		Size:     written,
	}, nil
}

func (s *Server) lpkFetchURL(ctx context.Context, rawURL string, useMirrorDownload bool) (*url.URL, error) {
	candidates, err := s.lpkFetchURLs(ctx, rawURL, useMirrorDownload)
	if err != nil {
		return nil, err
	}
	return candidates[0], nil
}

func (s *Server) lpkFetchURLs(ctx context.Context, rawURL string, useMirrorDownload bool) ([]*url.URL, error) {
	normalized := normalizeGitHubRawURL(rawURL)
	parsed, err := parseHTTPDownloadURL(normalized)
	if err != nil {
		return nil, err
	}
	candidates := []*url.URL{}
	seen := map[string]struct{}{}
	addCandidate := func(raw string) error {
		candidate, err := parseHTTPDownloadURL(raw)
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
		for _, entry := range gitHubMirrorsForURL(s.effectiveGitHubMirrors(ctx), parsed.String()) {
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

func summarizeLPKFetchFailures(failures []string) string {
	const maxVisible = 4
	if len(failures) <= maxVisible {
		return strings.Join(failures, "; ")
	}
	return strings.Join(failures[:maxVisible], "; ") + fmt.Sprintf("; ... %d more", len(failures)-maxVisible)
}

func parseHTTPDownloadURL(rawURL string) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Host == "" {
		return nil, errors.New("downloadUrl must be a valid URL")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, errors.New("downloadUrl must use http or https")
	}
	return parsed, nil
}

func firstGitHubMirrorForURL(entries []mirror.Entry, rawURL string) (mirror.Entry, bool) {
	mirrors := gitHubMirrorsForURL(entries, rawURL)
	if len(mirrors) == 0 {
		return mirror.Entry{}, false
	}
	return mirrors[0], true
}

func gitHubMirrorsForURL(entries []mirror.Entry, rawURL string) []mirror.Entry {
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

func normalizeGitHubRawURL(rawURL string) string {
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

func validateLPKURLHost(ctx context.Context, parsed *url.URL, allowPrivate bool) error {
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
		if unsafeIP(ip) {
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
		if unsafeIP(addr.IP) {
			return errors.New("downloadUrl host must not resolve to a private or local address")
		}
	}
	return nil
}

func unsafeIP(ip net.IP) bool {
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
