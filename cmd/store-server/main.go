package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"time"

	"lazycat.community/appstore/internal/config"
	"lazycat.community/appstore/internal/server"
)

func main() {
	cfg := config.Load()
	app, err := server.New(cfg)
	if err != nil {
		log.Fatalf("start appstore server: %v", err)
	}
	defer app.Close()

	httpServer := &http.Server{
		Addr:         cfg.Addr,
		Handler:      app.Handler(),
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
	}
	app.SetRestartAfterImport(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(ctx); err != nil {
			log.Printf("shutdown after migration import: %v", err)
		}
	})
	log.Printf("MiaoMiao Private Store server listening on %s", cfg.Addr)
	if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
