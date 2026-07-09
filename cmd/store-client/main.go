package main

import (
	"errors"
	"log"
	"net/http"

	"lazycat.community/appstore/internal/clientserver"
)

func main() {
	cfg := clientserver.LoadConfig()
	app, err := clientserver.New(cfg)
	if err != nil {
		log.Fatalf("start appstore client: %v", err)
	}
	defer app.Close()

	server := &http.Server{
		Addr:    cfg.Addr,
		Handler: app.Handler(),
	}
	log.Printf("MiaoMiao Private Store client listening on %s", cfg.Addr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
