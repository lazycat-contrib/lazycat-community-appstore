package server

import (
	"context"
	"net/http"
)

func (s *Server) sharedFirstLoad(ctx context.Context, key string, load func(context.Context) (any, error)) (any, error) {
	value, err, _ := s.firstLoadGroup.Do(key, func() (any, error) {
		return load(context.WithoutCancel(ctx))
	})
	return value, err
}

func firstLoadKey(r *http.Request, scope string) string {
	key := scope
	if r != nil && r.URL != nil {
		if query := r.URL.Query().Encode(); query != "" {
			key += "?" + query
		}
	}
	return key
}
