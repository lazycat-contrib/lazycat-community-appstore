package server

import (
	"net/http"
	"strings"
)

type responseStatusWriter struct {
	http.ResponseWriter
	status int
}

func (writer *responseStatusWriter) WriteHeader(status int) {
	if writer.status != 0 {
		return
	}
	writer.status = status
	writer.ResponseWriter.WriteHeader(status)
}

func (writer *responseStatusWriter) Write(raw []byte) (int, error) {
	if writer.status == 0 {
		writer.WriteHeader(http.StatusOK)
	}
	return writer.ResponseWriter.Write(raw)
}

func (writer *responseStatusWriter) Unwrap() http.ResponseWriter {
	return writer.ResponseWriter
}

func (s *Server) withSourceFeedInvalidation(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isSourceFeedMutationRequest(r) {
			next.ServeHTTP(w, r)
			return
		}
		writer := &responseStatusWriter{ResponseWriter: w}
		next.ServeHTTP(writer, r)
		status := writer.status
		if status == 0 {
			status = http.StatusOK
		}
		if status >= http.StatusOK && status < http.StatusBadRequest {
			s.invalidateSourceFeed()
		}
	})
}

func (s *Server) invalidateSourceFeed() {
	if s.sourceFeedCache != nil {
		s.sourceFeedCache.InvalidateAndWarm()
	}
}

func isSourceFeedMutationRequest(r *http.Request) bool {
	if r == nil || (r.Method != http.MethodPost && r.Method != http.MethodPatch && r.Method != http.MethodDelete) {
		return false
	}
	path := r.URL.Path
	if path == "/api/v1/setup" || path == "/api/v1/me/profile" || path == "/api/v1/admin/migration/import" {
		return true
	}
	if path == "/api/v1/apps" {
		return r.Method == http.MethodPost
	}
	if strings.HasPrefix(path, "/api/v1/apps/") {
		for _, excluded := range []string{"/comments", "/chat", "/favorites", "/rating", "/collaborator"} {
			if strings.Contains(path, excluded) {
				return false
			}
		}
		return true
	}
	if strings.HasPrefix(path, "/api/v1/admin/reviews/") {
		return true
	}
	if strings.HasPrefix(path, "/api/v1/groups") {
		return !strings.Contains(path, "/members/") && path != "/api/v1/groups/client-config"
	}
	for _, prefix := range []string{
		"/api/v1/admin/categories",
		"/api/v1/admin/tags",
		"/api/v1/admin/users",
		"/api/v1/admin/settings",
		"/api/v1/admin/announcements",
		"/api/v1/admin/ads",
	} {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}
