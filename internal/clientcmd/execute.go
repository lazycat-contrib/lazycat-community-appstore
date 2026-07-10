package clientcmd

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"lazycat.community/appstore/internal/clientserver"
	"lazycat.community/appstore/internal/httpserve"
)

var run = Run

func Execute() int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx); err != nil {
		log.Printf("Store client stopped: %v", err)
		return 1
	}
	return 0
}

func Run(ctx context.Context) error {
	cfg := clientserver.LoadConfig()
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("validate client config: %w", err)
	}
	app, err := clientserver.New(cfg)
	if err != nil {
		return fmt.Errorf("create client: %w", err)
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
	log.Printf("MiaoMiao Private Store client listening on %s", cfg.Addr)
	return httpserve.Run(ctx, httpServer, httpserve.Options{
		ShutdownTimeout: cfg.ShutdownTimeout,
		Stop:            app.Stop,
		Close:           app.CloseContext,
	})
}
