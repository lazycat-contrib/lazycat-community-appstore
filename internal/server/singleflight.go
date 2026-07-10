package server

import (
	"context"
	"net/http"
	"time"
)

func (s *Server) sharedFirstLoad(ctx context.Context, key string, load func(context.Context) (any, error)) (any, error) {
	serverCtx := s.ctx
	if serverCtx == nil {
		serverCtx = context.Background()
	}
	result := s.firstLoadGroup.DoChan(key, func() (any, error) {
		if !s.beginBackground() {
			return nil, context.Canceled
		}
		defer s.endBackground()
		loadCtx, cancel := context.WithTimeout(serverCtx, 30*time.Second)
		defer cancel()
		return load(loadCtx)
	})
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case item := <-result:
		return item.Val, item.Err
	}
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
