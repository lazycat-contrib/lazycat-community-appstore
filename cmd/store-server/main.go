package main

import (
	"errors"
	"log"
	"net/http"

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
	log.Printf("LazyCat Private Store server listening on %s", cfg.Addr)
	if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
