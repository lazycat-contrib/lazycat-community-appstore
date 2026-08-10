package server

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"

	"lazycat.community/appstore/ent"
)

type wishActor struct {
	ClientUserID string
	OwnerID      string
	DisplayName  string
}

type wishViewer struct {
	User         *ent.User
	ClientUserID string
	OwnerID      string
}

func (v wishViewer) isAdmin() bool { return v.User != nil && isAdmin(v.User) }

func (s *Server) resolveWishClientActor(r *http.Request) (wishActor, int, string, string) {
	if _, ok := s.authenticate(r); ok {
		return wishActor{}, http.StatusForbidden, "LAZYCAT_CLIENT_REQUIRED", "Wishes can only be submitted from the MiaoMiao client"
	}
	if !s.cfg.TrustLazyCatClientComments || r.Header.Get("X-LazyCat-Client-Proxy") != "lazycat-appstore-client" {
		return wishActor{}, http.StatusForbidden, "LAZYCAT_CLIENT_REQUIRED", "Wishes require the MiaoMiao client"
	}
	deviceID := sanitizeIdentity(r.Header.Get("X-LazyCat-Client-Device-ID"))
	if deviceID == "" {
		return wishActor{}, http.StatusForbidden, "LAZYCAT_CLIENT_REQUIRED", "Wishes require an identified LazyCat device"
	}
	clientUserID := sanitizeIdentity(r.Header.Get("X-LazyCat-Client-User-ID"))
	if clientUserID == "" || !strings.HasPrefix(clientUserID, "lc_") {
		return wishActor{}, http.StatusForbidden, "LAZYCAT_CLIENT_REQUIRED", "Wishes require an identified LazyCat user"
	}
	if !s.sourcePasswordAllowsClientComment(r) {
		return wishActor{}, http.StatusUnauthorized, "SOURCE_PASSWORD_REQUIRED", "A valid source password is required"
	}
	displayName := sanitizeDisplayName(r.Header.Get("X-LazyCat-Client-Display-Name"))
	if displayName == "" {
		displayName = "MiaoMiao " + trimRunes(clientUserID, 12)
	}
	return wishActor{ClientUserID: clientUserID, OwnerID: scopedWishOwnerID(clientUserID, deviceID), DisplayName: displayName}, 0, "", ""
}

func (s *Server) wishViewerForRequest(r *http.Request) wishViewer {
	if u := s.optionalUser(r); u != nil {
		return wishViewer{User: u}
	}
	if s.cfg.TrustLazyCatClientComments &&
		r.Header.Get("X-LazyCat-Client-Proxy") == "lazycat-appstore-client" &&
		sanitizeIdentity(r.Header.Get("X-LazyCat-Client-Device-ID")) != "" &&
		s.sourcePasswordAllowsClientComment(r) {
		clientUserID := sanitizeIdentity(r.Header.Get("X-LazyCat-Client-User-ID"))
		if strings.HasPrefix(clientUserID, "lc_") {
			deviceID := sanitizeIdentity(r.Header.Get("X-LazyCat-Client-Device-ID"))
			return wishViewer{ClientUserID: clientUserID, OwnerID: scopedWishOwnerID(clientUserID, deviceID)}
		}
	}
	return wishViewer{}
}

func scopedWishOwnerID(clientUserID, deviceID string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(clientUserID) + "\x00" + strings.TrimSpace(deviceID)))
	return "lcw_" + hex.EncodeToString(sum[:])[:24]
}
