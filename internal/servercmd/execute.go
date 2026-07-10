package servercmd

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"lazycat.community/appstore/internal/config"
	"lazycat.community/appstore/internal/httpserve"
	"lazycat.community/appstore/internal/server"
)

var run = Run

func Execute() int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx); err != nil {
		log.Printf("Store server stopped: %v", err)
		return 1
	}
	return 0
}

func Run(ctx context.Context) error {
	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("validate server config: %w", err)
	}
	app, err := server.New(cfg)
	if err != nil {
		return fmt.Errorf("create server: %w", err)
	}

	httpServer := &http.Server{
		Addr:              cfg.Addr,
		Handler:           app.Handler(),
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
		MaxHeaderBytes:    cfg.MaxHeaderBytes,
	}
	log.Printf("MiaoMiao Private Store server listening on %s", cfg.Addr)
	return httpserve.Run(ctx, httpServer, httpserve.Options{
		ShutdownTimeout: cfg.ShutdownTimeout,
		Restart:         app.RestartRequested(),
		Stop:            app.Stop,
		Close:           app.CloseContext,
	})
}
