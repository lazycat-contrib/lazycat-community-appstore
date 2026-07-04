package clientserver

import (
	"net/http"

	"lazycat.community/appstore/ent"
)

type Server struct {
	cfg Config
	db  *ent.Client
	mux *http.ServeMux
}

func New(cfg Config) (*Server, error) {
	db, err := openDB(cfg)
	if err != nil {
		return nil, err
	}
	s := &Server{cfg: cfg, db: db, mux: http.NewServeMux()}
	s.routes()
	return s, nil
}

func newTestServer(db *ent.Client) *Server {
	s := &Server{cfg: Config{DefaultSourceName: "Community Store"}, db: db, mux: http.NewServeMux()}
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
}
