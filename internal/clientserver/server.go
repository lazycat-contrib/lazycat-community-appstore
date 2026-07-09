package clientserver

import (
	"context"
	"io/fs"
	"net/http"
	"path"
	"strings"

	"lazycat.community/appstore/clientembed"
	"lazycat.community/appstore/ent"
)

type Server struct {
	cfg           Config
	db            *ent.Client
	pkg           PackageManager
	mux           *http.ServeMux
	syncScheduler *sourceSyncScheduler
	auth          *clientAuth
}

func New(cfg Config) (*Server, error) {
	db, err := openDB(cfg)
	if err != nil {
		return nil, err
	}
	if err := migrateSchema(context.Background(), db); err != nil {
		_ = db.Close()
		return nil, err
	}
	auth, err := newClientAuth(context.Background(), cfg)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	s := &Server{cfg: cfg, db: db, pkg: NewLazyCatPackageManager(), mux: http.NewServeMux(), auth: auth}
	s.routes()
	syncScheduler, err := newSourceSyncScheduler(s)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	s.syncScheduler = syncScheduler
	return s, nil
}

func newTestServer(db *ent.Client) *Server {
	s := &Server{
		cfg:  Config{DefaultSourceName: "喵喵私有商店", SessionSecret: "test-client-session-secret"},
		db:   db,
		pkg:  unavailablePackageManager{},
		mux:  http.NewServeMux(),
		auth: &clientAuth{secret: []byte("test-client-session-secret")},
	}
	s.routes()
	return s
}

func (s *Server) Close() error {
	if s.syncScheduler != nil {
		_ = s.syncScheduler.Close()
	}
	return s.db.Close()
}

func (s *Server) Handler() http.Handler {
	return s.mux
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/client/v1/auth/me", s.handleClientAuthMe)
	s.mux.HandleFunc("POST /api/client/v1/auth/logout", s.handleClientAuthLogout)
	s.mux.HandleFunc("GET /auth/oidc/start", s.handleClientOIDCStart)
	s.mux.HandleFunc("GET /auth/oidc/callback", s.handleClientOIDCCallback)
	s.mux.HandleFunc("GET /api/client/v1/assets/{id}", s.clientAPI(s.handleClientAsset))
	s.mux.HandleFunc("HEAD /api/client/v1/assets/{id}", s.clientAPI(s.handleClientAsset))
	s.mux.HandleFunc("GET /api/client/v1/sources", s.clientAPI(s.handleListSources))
	s.mux.HandleFunc("POST /api/client/v1/sources", s.clientAPI(s.handleCreateSource))
	s.mux.HandleFunc("PATCH /api/client/v1/sources/{id}", s.clientAPI(s.handleUpdateSource))
	s.mux.HandleFunc("DELETE /api/client/v1/sources/{id}", s.clientAPI(s.handleDeleteSource))
	s.mux.HandleFunc("POST /api/client/v1/sources/{id}/sync", s.clientAPI(s.handleSyncSource))
	s.mux.HandleFunc("POST /api/client/v1/sources/sync", s.clientAPI(s.handleSyncAllSources))
	s.mux.HandleFunc("GET /api/client/v1/settings", s.clientAPI(s.handleGetSettings))
	s.mux.HandleFunc("PATCH /api/client/v1/settings", s.clientAPI(s.handleUpdateSettings))
	s.mux.HandleFunc("GET /api/client/v1/apps", s.clientAPI(s.handleListApps))
	s.mux.HandleFunc("GET /api/client/v1/apps/{id}", s.clientAPI(s.handleGetApp))
	s.mux.HandleFunc("GET /api/client/v1/apps/{id}/versions", s.clientAPI(s.handleGetAppVersions))
	s.mux.HandleFunc("GET /api/client/v1/apps/{id}/comments", s.clientAPI(s.handleListSourceAppComments))
	s.mux.HandleFunc("POST /api/client/v1/apps/{id}/comments", s.clientAPI(s.handleCreateSourceAppComment))
	s.mux.HandleFunc("DELETE /api/client/v1/apps/{id}/comments/{commentId}", s.clientAPI(s.handleDeleteSourceAppComment))
	s.mux.HandleFunc("POST /api/client/v1/apps/{id}/outdated-marks", s.clientAPI(s.handleMarkSourceAppOutdated))
	s.mux.HandleFunc("DELETE /api/client/v1/apps/{id}/outdated-marks", s.clientAPI(s.handleClearSourceAppOutdated))
	s.mux.HandleFunc("POST /api/client/v1/apps/{id}/chat", s.clientAPI(s.handleCreateAppChatConversation))
	s.mux.HandleFunc("GET /api/client/v1/chat/conversations", s.clientAPI(s.handleListChatConversations))
	s.mux.HandleFunc("GET /api/client/v1/chat/conversations/{id}/messages", s.clientAPI(s.handleListChatMessages))
	s.mux.HandleFunc("POST /api/client/v1/chat/conversations/{id}/messages", s.clientAPI(s.handleCreateChatMessage))
	s.mux.HandleFunc("POST /api/client/v1/chat/conversations/{id}/read", s.clientAPI(s.handleReadChatConversation))
	s.mux.HandleFunc("DELETE /api/client/v1/chat/conversations/{id}", s.clientAPI(s.handleDeleteChatConversation))
	s.mux.HandleFunc("GET /api/client/v1/chat/events", s.clientAPI(s.handleChatEvents))
	s.mux.HandleFunc("GET /api/client/v1/installed", s.clientAPI(s.handleInstalled))
	s.mux.HandleFunc("POST /api/client/v1/install", s.clientAPI(s.handleInstall))
	s.mux.HandleFunc("GET /api/client/v1/history", s.clientAPI(s.handleInstallHistory))
	s.mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "service": "lazycat-private-store-client"})
	})
	s.mux.Handle("/", embeddedClientHandler())
}

func (s *Server) clientAPI(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, ok := s.clientIdentity(r)
		if !ok {
			writeError(w, http.StatusUnauthorized, "CLIENT_AUTH_REQUIRED", "LazyCat OIDC login is required")
			return
		}
		next(w, r.WithContext(withClientIdentity(r.Context(), identity)))
	}
}

func (s *Server) clientIdentity(r *http.Request) (clientIdentity, bool) {
	if identity, ok := identityFromHeader(r); ok {
		return identity, true
	}
	if s.auth != nil {
		if session, ok := s.auth.session(r); ok {
			displayName := strings.TrimSpace(session.DisplayName)
			if displayName == "" {
				displayName = session.UserID
			}
			return clientIdentity{
				UserID:      session.UserID,
				DisplayName: displayName,
				Email:       session.Email,
				AvatarURL:   session.AvatarURL,
				Source:      clientIdentityOIDC,
				Issuer:      session.Issuer,
				Subject:     session.Subject,
			}, true
		}
		if s.auth.OIDCEnabled() {
			return clientIdentity{}, false
		}
	}
	return clientIdentity{UserID: clientIdentityLocal, DisplayName: clientIdentityLocal, Source: clientIdentityLocal}, true
}

func embeddedClientHandler() http.Handler {
	dist, err := fs.Sub(clientembed.Dist, "dist")
	if err != nil {
		return http.NotFoundHandler()
	}
	fileServer := http.FileServer(http.FS(dist))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.NotFound(w, r)
			return
		}
		requestPath := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if requestPath == "." || requestPath == "" {
			fileServer.ServeHTTP(w, r)
			return
		}
		if info, err := fs.Stat(dist, requestPath); err == nil && !info.IsDir() {
			fileServer.ServeHTTP(w, r)
			return
		}
		if _, err := fs.Stat(dist, "index.html"); err != nil {
			http.NotFound(w, r)
			return
		}
		clone := r.Clone(r.Context())
		clone.URL.Path = "/"
		fileServer.ServeHTTP(w, clone)
	})
}
