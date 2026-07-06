package server

import "net/http"

func (s *Server) handleSiteProfile(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"site": s.siteProfile(r.Context())})
}
