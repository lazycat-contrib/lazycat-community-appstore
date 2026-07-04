package clientserver

import (
	"io/fs"
	"net/http"
	"path"
	"strings"

	"lazycat.community/appstore/clientembed"
	"lazycat.community/appstore/ent"
)

type Server struct {
	cfg Config
	db  *ent.Client
	pkg PackageManager
	mux *http.ServeMux
}

func New(cfg Config) (*Server, error) {
	db, err := openDB(cfg)
	if err != nil {
		return nil, err
	}
	s := &Server{cfg: cfg, db: db, pkg: newLazyCatPackageManager(), mux: http.NewServeMux()}
	s.routes()
	return s, nil
}

func newTestServer(db *ent.Client) *Server {
	s := &Server{cfg: Config{DefaultSourceName: "Community Store"}, db: db, pkg: unavailablePackageManager{}, mux: http.NewServeMux()}
	s.routes()
	return s
}

func (s *Server) Close() error {
	return s.db.Close()
}

func (s *Server) Handler() http.Handler {
	return s.mux
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/client/v1/sources", s.handleListSources)
	s.mux.HandleFunc("POST /api/client/v1/sources", s.handleCreateSource)
	s.mux.HandleFunc("PATCH /api/client/v1/sources/{id}", s.handleUpdateSource)
	s.mux.HandleFunc("DELETE /api/client/v1/sources/{id}", s.handleDeleteSource)
	s.mux.HandleFunc("POST /api/client/v1/sources/{id}/sync", s.handleSyncSource)
	s.mux.HandleFunc("POST /api/client/v1/sources/sync", s.handleSyncAllSources)
	s.mux.HandleFunc("GET /api/client/v1/apps", s.handleListApps)
	s.mux.HandleFunc("GET /api/client/v1/apps/{id}", s.handleGetApp)
	s.mux.HandleFunc("GET /api/client/v1/installed", s.handleInstalled)
	s.mux.HandleFunc("POST /api/client/v1/install", s.handleInstall)
	s.mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "service": "lazycat-appstore-client"})
	})
	s.mux.Handle("/", embeddedClientHandler())
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
