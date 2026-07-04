package server

import (
	"net/http"
	"strings"
)

func (s *Server) handleClientInstalled(w http.ResponseWriter, r *http.Request) {
	apps, err := s.pkg.QueryInstalled(r.Context(), currentLazyCatUserID(r))
	if err != nil {
		writeError(w, http.StatusBadGateway, "LAZYCAT_SDK_UNAVAILABLE", err.Error(), nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"apps": apps})
}

func currentLazyCatUserID(r *http.Request) string {
	userID := strings.TrimSpace(r.Header.Get("x-hc-user-id"))
	if userID == "" {
		return "local"
	}
	return userID
}
