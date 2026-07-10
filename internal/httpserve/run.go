package httpserve

import (
	"context"
	"errors"
	"net"
	"net/http"
	"time"
)

type Options struct {
	ShutdownTimeout time.Duration
	Restart         <-chan struct{}
	Stop            func(context.Context) error
	Close           func(context.Context) error
}

func Run(ctx context.Context, server *http.Server, options Options) error {
	if options.ShutdownTimeout <= 0 {
		return errors.New("ShutdownTimeout must be positive")
	}
	listener, err := (&net.ListenConfig{}).Listen(ctx, "tcp", server.Addr)
	if err != nil {
		return errors.Join(err, closeAfterListenFailure(options))
	}
	return RunListener(ctx, server, listener, options)
}

func closeAfterListenFailure(options Options) error {
	ctx, cancel := context.WithTimeout(context.Background(), options.ShutdownTimeout)
	defer cancel()
	var stopErr, closeErr error
	if options.Stop != nil {
		stopErr = options.Stop(ctx)
	}
	if options.Close != nil {
		closeErr = options.Close(ctx)
	}
	return errors.Join(stopErr, closeErr)
}

func RunListener(ctx context.Context, server *http.Server, listener net.Listener, options Options) error {
	if options.ShutdownTimeout <= 0 {
		_ = listener.Close()
		return errors.New("ShutdownTimeout must be positive")
	}
	serveErrCh := make(chan error, 1)
	go func() {
		serveErrCh <- normalizeServeError(server.Serve(listener))
	}()

	var (
		runErr        error
		serveFinished bool
	)
	select {
	case runErr = <-serveErrCh:
		serveFinished = true
	case <-ctx.Done():
	case <-options.Restart:
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), options.ShutdownTimeout)
	defer cancel()

	runErr = errors.Join(runErr, normalizeCloseError(listener.Close()))
	if options.Stop != nil {
		runErr = errors.Join(runErr, normalizeCloseError(options.Stop(shutdownCtx)))
	}
	if err := normalizeCloseError(server.Shutdown(shutdownCtx)); err != nil {
		runErr = errors.Join(runErr, err, normalizeCloseError(server.Close()))
	}
	if !serveFinished {
		select {
		case err := <-serveErrCh:
			runErr = errors.Join(runErr, err)
		case <-shutdownCtx.Done():
			runErr = errors.Join(runErr, shutdownCtx.Err(), normalizeCloseError(server.Close()))
			runErr = errors.Join(runErr, <-serveErrCh)
		}
	}
	if options.Close != nil {
		runErr = errors.Join(runErr, normalizeCloseError(options.Close(shutdownCtx)))
	}
	return runErr
}

func normalizeServeError(err error) error {
	if errors.Is(err, http.ErrServerClosed) || errors.Is(err, net.ErrClosed) {
		return nil
	}
	return err
}

func normalizeCloseError(err error) error {
	if errors.Is(err, http.ErrServerClosed) || errors.Is(err, net.ErrClosed) {
		return nil
	}
	return err
}
