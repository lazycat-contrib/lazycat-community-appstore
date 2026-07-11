package clientserver

import (
	"context"
	"database/sql"
	"errors"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"sync"
	"time"

	"lazycat.community/appstore/clientembed"
	"lazycat.community/appstore/ent"
)

type Server struct {
	cfg           Config
	db            *ent.Client
	sqlDB         *sql.DB
	pkg           PackageManager
	mux           *http.ServeMux
	syncScheduler *sourceSyncScheduler
	auth          *clientAuth
	httpClient    *http.Client
	streamClient  *http.Client
	httpClientsMu sync.Mutex
	sourcePolicy  sourceURLPolicy
	stopOnce      sync.Once
	stopDone      chan struct{}
	stopErr       error
	closeOnce     sync.Once
	closeDone     chan struct{}
	closeErr      error
}

func New(cfg Config) (*Server, error) {
	db, sqlDB, err := openDB(cfg)
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
	httpClient, streamClient := newHTTPClients()
	s := &Server{
		cfg:          cfg,
		db:           db,
		sqlDB:        sqlDB,
		pkg:          NewLazyCatPackageManager(),
		mux:          http.NewServeMux(),
		auth:         auth,
		httpClient:   httpClient,
		streamClient: streamClient,
		sourcePolicy: allowSourceURLPolicy{},
		stopDone:     make(chan struct{}),
		closeDone:    make(chan struct{}),
	}
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
	httpClient, streamClient := newHTTPClients()
	s := &Server{
		cfg:          Config{DefaultSourceName: "喵喵私有商店", SessionSecret: "test-client-session-secret"},
		db:           db,
		pkg:          unavailablePackageManager{},
		mux:          http.NewServeMux(),
		auth:         &clientAuth{secret: []byte("test-client-session-secret")},
		httpClient:   httpClient,
		streamClient: streamClient,
		sourcePolicy: allowSourceURLPolicy{},
		stopDone:     make(chan struct{}),
		closeDone:    make(chan struct{}),
	}
	s.routes()
	return s
}

func (s *Server) ensureHTTPClients() {
	s.httpClientsMu.Lock()
	defer s.httpClientsMu.Unlock()
	if s.httpClient != nil && s.streamClient != nil {
		return
	}
	ordinary, stream := newHTTPClients()
	if s.httpClient == nil {
		s.httpClient = ordinary
	}
	if s.streamClient == nil {
		s.streamClient = stream
	}
}

func (s *Server) Close() error {
	timeout := s.cfg.ShutdownTimeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return s.CloseContext(ctx)
}

func (s *Server) CloseContext(ctx context.Context) error {
	s.startClose()
	select {
	case <-s.closeDone:
		return s.closeErr
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *Server) Stop(ctx context.Context) error {
	s.startStop()
	select {
	case <-s.stopDone:
		return s.stopErr
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *Server) startStop() {
	s.stopOnce.Do(func() {
		if s.stopDone == nil {
			s.stopDone = make(chan struct{})
		}
		if s.syncScheduler != nil {
			s.syncScheduler.Stop()
		}
		go func() {
			if s.syncScheduler != nil {
				s.stopErr = s.syncScheduler.CloseContext(context.Background())
			}
			close(s.stopDone)
		}()
	})
}

func (s *Server) startClose() {
	s.closeOnce.Do(func() {
		if s.closeDone == nil {
			s.closeDone = make(chan struct{})
		}
		go func() {
			stopErr := s.Stop(context.Background())
			var dbErr error
			if s.db != nil {
				dbErr = s.db.Close()
			}
			s.closeErr = errors.Join(stopErr, dbErr)
			close(s.closeDone)
		}()
	})
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
	s.mux.HandleFunc("GET /api/client/v1/install-tasks/{taskId}", s.clientAPI(s.handleGetInstallTask))
	s.mux.HandleFunc("DELETE /api/client/v1/install-tasks/{taskId}", s.clientAPI(s.handleCancelInstallTask))
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
