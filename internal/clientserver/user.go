package clientserver

import (
	"context"
	"net/http"
	"strings"
)

const (
	clientIdentityHeader = "header"
	clientIdentityOIDC   = "oidc"
	clientIdentityLocal  = "local"
)

type clientIdentityContextKey struct{}

type clientIdentity struct {
	UserID      string
	DisplayName string
	Email       string
	AvatarURL   string
	Source      string
	Issuer      string
	Subject     string
}

func currentUserID(r *http.Request) string {
	if identity, ok := r.Context().Value(clientIdentityContextKey{}).(clientIdentity); ok {
		if userID := strings.TrimSpace(identity.UserID); userID != "" {
			return userID
		}
	}
	if identity, ok := identityFromHeader(r); ok {
		return identity.UserID
	}
	return clientIdentityLocal
}

func currentClientIdentity(r *http.Request) (clientIdentity, bool) {
	identity, ok := r.Context().Value(clientIdentityContextKey{}).(clientIdentity)
	return identity, ok
}

func withClientIdentity(ctx context.Context, identity clientIdentity) context.Context {
	return context.WithValue(ctx, clientIdentityContextKey{}, identity)
}

func identityFromHeader(r *http.Request) (clientIdentity, bool) {
	userID := strings.TrimSpace(r.Header.Get("x-hc-user-id"))
	if userID == "" {
		return clientIdentity{}, false
	}
	displayName := strings.TrimSpace(r.Header.Get("x-hc-user-name"))
	if displayName == "" {
		displayName = userID
	}
	return clientIdentity{
		UserID:      userID,
		DisplayName: displayName,
		Source:      clientIdentityHeader,
	}, true
}
