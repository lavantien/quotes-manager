package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"time"

	canonserver "github.com/lavantien/quotes-manager/internal/canon/server"
	"github.com/lavantien/quotes-manager/internal/cli"
	"github.com/lavantien/quotes-manager/internal/config"
	"github.com/lavantien/quotes-manager/internal/seed"
	"github.com/lavantien/quotes-manager/internal/server"
	"github.com/lavantien/quotes-manager/internal/store"
)

func main() {
	cli.SetServeStarter(serveMerged)
	os.Exit(cli.Main(os.Args[1:]))
}

func serveMerged(ctx context.Context, cfg *config.Config) error {
	st, err := store.Open(cfg.Quotes.DBPath)
	if err != nil {
		return err
	}
	defer func() { _ = st.Close() }()
	if err := seed.EnsureSeeded(st.DB()); err != nil {
		return err
	}
	canonSrv, err := canonserver.New(cfg)
	if err != nil {
		return err
	}
	root := http.NewServeMux()
	server.New(st).RegisterRoutes(root)
	canonSrv.RegisterRoutes(root)
	srv := &http.Server{
		Addr:         cfg.Server.Addr,
		Handler:      canonSrv.Chain(root),
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
		budget := time.Duration(cfg.Server.ShutdownTimeoutSeconds) * time.Second
		canonSrv.CancelActiveJobs(budget)
		shutdownCtx, cancel := context.WithTimeout(context.Background(), budget)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			return err
		}
		if err := <-errCh; err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return canonSrv.Close()
	}
}
