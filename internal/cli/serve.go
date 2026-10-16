package cli

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/lavantien/quotes-manager/internal/config"
)

var serveStarter func(ctx context.Context, cfg *config.Config) error

func SetServeStarter(f func(ctx context.Context, cfg *config.Config) error) {
	serveStarter = f
}

func init() {
	Register(Command{
		Name:  "serve",
		Usage: "serve the api and frontend: --addr --db overrides",
		Run:   runServe,
	})
}

func resolveServe(cfg *config.Config, args []string) (*config.Config, error) {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	addr := fs.String("addr", cfg.Server.Addr, "listen address, host:port")
	db := fs.String("db", cfg.Quotes.DBPath, "SQLite database path")
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	if fs.NArg() > 0 {
		return nil, fmt.Errorf("serve: unexpected argument %q", fs.Arg(0))
	}
	resolved := *cfg
	resolved.Server.Addr = *addr
	resolved.Quotes.DBPath = *db
	if err := resolved.Validate(); err != nil {
		return nil, fmt.Errorf("serve: %w", err)
	}
	return &resolved, nil
}

func runServe(ctx context.Context, cfg *config.Config, args []string) error {
	if serveStarter == nil {
		return fmt.Errorf("serve: server not wired (run via cmd/qm)")
	}
	cfg, err := resolveServe(cfg, args)
	if err != nil {
		return err
	}
	printLine("serve: listening on http://%s\n", cfg.Server.Addr)
	sctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()
	return serveStarter(sctx, cfg)
}
