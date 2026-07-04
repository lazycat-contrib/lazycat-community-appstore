package clientserver

import (
	"net/http"
	"strings"
)

func currentUserID(r *http.Request) string {
	userID := strings.TrimSpace(r.Header.Get("x-hc-user-id"))
	if userID == "" {
		return "local"
	}
	return userID
}
