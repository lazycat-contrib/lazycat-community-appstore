package server

import (
	"context"
	"mime/multipart"
	"net/url"

	"lazycat.community/appstore/internal/lpkinspect"
	"lazycat.community/appstore/internal/lpkmeta"
)

type lpkInspection = lpkinspect.Inspection

var (
	lpkInspectionTotalTimeout = lpkinspect.DefaultInspectionTotalTimeout
	lpkFetchCandidateTimeout  = lpkinspect.DefaultFetchCandidateTimeout
)

func parseUploadedLPKMetadata(file multipart.File, header *multipart.FileHeader, maxBytes int64) (lpkmeta.Metadata, error) {
	return lpkinspect.ParseUploaded(file, header, maxBytes)
}

func (s *Server) inspectLPKURL(ctx context.Context, rawURL string, maxBytes int64, useMirrorDownload bool) (lpkInspection, error) {
	return lpkinspect.InspectURL(ctx, rawURL, lpkinspect.URLOptions{
		MaxBytes:          maxBytes,
		UseMirrorDownload: useMirrorDownload,
		Mirrors:           s.effectiveGitHubMirrors(ctx),
		AllowPrivateHosts: s.allowPrivateLPKURLHosts,
		TotalTimeout:      lpkInspectionTotalTimeout,
		CandidateTimeout:  lpkFetchCandidateTimeout,
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
