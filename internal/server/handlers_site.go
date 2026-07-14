package server

import (
	"context"
	"net/http"
)

func (s *Server) handleSiteProfile(w http.ResponseWriter, r *http.Request) {
	value, err := s.sharedFirstLoad(r.Context(), firstLoadKey(r, "site-profile"), s.siteProfileResponse)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "SITE_PROFILE_FAILED", "Could not load site profile", nil)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, value)
}

func (s *Server) siteProfileResponse(ctx context.Context) (any, error) {
	return map[string]any{"site": s.siteProfile(ctx)}, nil
}
