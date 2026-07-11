package server

import (
	"context"
	"mime/multipart"
	"net/url"
	"time"

	"lazycat.community/appstore/internal/lpkinspect"
)

type lpkInspection = lpkinspect.Inspection

var (
	lpkInspectionTotalTimeout = lpkinspect.DefaultInspectionTotalTimeout
	lpkFetchCandidateTimeout  = lpkinspect.DefaultFetchCandidateTimeout
)

func parseUploadedLPKMetadata(file multipart.File, header *multipart.FileHeader, maxBytes int64) (lpkinspect.Metadata, error) {
	return lpkinspect.ParseUploaded(file, header, maxBytes)
}

func (s *Server) inspectLPKURL(ctx context.Context, rawURL string, maxBytes int64, useMirrorDownload bool) (lpkInspection, error) {
	return s.inspectLPKURLWithTimeout(ctx, rawURL, maxBytes, useMirrorDownload, lpkInspectionTotalTimeout)
}

func (s *Server) inspectLPKURLWithTimeout(ctx context.Context, rawURL string, maxBytes int64, useMirrorDownload bool, timeout time.Duration) (lpkInspection, error) {
	if timeout <= 0 {
		return lpkInspection{}, context.DeadlineExceeded
	}
	return lpkinspect.InspectURL(ctx, rawURL, lpkinspect.URLOptions{
		MaxBytes:          maxBytes,
		UseMirrorDownload: useMirrorDownload,
		Mirrors:           s.effectiveGitHubMirrors(ctx),
		AllowPrivateHosts: s.allowPrivateLPKURLHosts,
		TotalTimeout:      timeout,
		CandidateTimeout:  min(lpkFetchCandidateTimeout, timeout),
	})
}

func (s *Server) lpkFetchURL(ctx context.Context, rawURL string, useMirrorDownload bool) (*url.URL, error) {
	candidates, err := s.lpkFetchURLs(ctx, rawURL, useMirrorDownload)
	if err != nil {
		return nil, err
	}
	return candidates[0], nil
}

func (s *Server) lpkFetchURLs(ctx context.Context, rawURL string, useMirrorDownload bool) ([]*url.URL, error) {
	return lpkinspect.FetchURLs(rawURL, useMirrorDownload, s.effectiveGitHubMirrors(ctx))
}

func normalizeGitHubRawURL(rawURL string) string {
	return lpkinspect.NormalizeGitHubRawURL(rawURL)
}
