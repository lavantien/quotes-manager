package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"time"

	"github.com/lavantien/quotes-manager/internal/cli"
	"github.com/lavantien/quotes-manager/internal/config"
	"github.com/lavantien/quotes-manager/internal/seed"
	"github.com/lavantien/quotes-manager/internal/server"
	"github.com/lavantien/quotes-manager/internal/store"
)

func main() {
	cli.SetServeStarter(serveQuotes)
	os.Exit(cli.Main(os.Args[1:]))
}

func serveQuotes(ctx context.Context, cfg *config.Config) error {
	st, err := store.Open(cfg.Quotes.DBPath)
	if err != nil {
		return err
	}
	defer func() { _ = st.Close() }()
	if err := seed.EnsureSeeded(st.DB()); err != nil {
		return err
	}
	srv := &http.Server{
		Addr:         cfg.Server.Addr,
		Handler:      server.New(st),
		ReadTimeout:  time.Duration(cfg.Server.ReadTimeoutSeconds) * time.Second,
		WriteTimeout: time.Duration(cfg.Server.WriteTimeoutSeconds) * time.Second,
		IdleTimeout:  time.Duration(cfg.Server.IdleTimeoutSeconds) * time.Second,
	}
	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()
	select {
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.Server.ShutdownTimeoutSeconds)*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}
}
