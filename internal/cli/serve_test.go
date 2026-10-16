package cli

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/lavantien/quotes-manager/internal/config"
)

func TestServeRegistered(t *testing.T) {
	cmd, ok := Lookup("serve")
	if !ok {
		t.Fatal("serve not registered")
	}
	if cmd.Run == nil {
		t.Error("serve command has no Run")
	}
}

func TestResolveServeAddrOverride(t *testing.T) {
	cfg := config.Defaults()
	got, err := resolveServe(cfg, []string{"--addr", "127.0.0.1:9787"})
	if err != nil {
		t.Fatalf("resolveServe: %v", err)
	}
	if got.Server.Addr != "127.0.0.1:9787" {
		t.Errorf("resolved addr = %q; want 127.0.0.1:9787", got.Server.Addr)
	}
	if cfg.Server.Addr != "127.0.0.1:8080" {
		t.Errorf("caller config mutated: addr = %q", cfg.Server.Addr)
	}
}

func TestResolveServeDefaultKeepsCfgAddr(t *testing.T) {
	cfg := config.Defaults()
	cfg.Server.Addr = "localhost:9000"
	got, err := resolveServe(cfg, nil)
	if err != nil {
		t.Fatalf("resolveServe: %v", err)
	}
	if got.Server.Addr != "localhost:9000" {
		t.Errorf("resolved addr = %q; want localhost:9000", got.Server.Addr)
	}
}

func TestResolveServeRejectsInvalidAddr(t *testing.T) {
	if _, err := resolveServe(config.Defaults(), []string{"--addr", "nohostport"}); err == nil {
		t.Error("addr without host:port must fail validation")
	}
}

func TestResolveServeRejectsUnexpectedArg(t *testing.T) {
	if _, err := resolveServe(config.Defaults(), []string{"extra"}); err == nil {
		t.Error("unexpected positional argument must fail")
	}
}

func TestServeRequiresWiring(t *testing.T) {
	old := serveStarter
	serveStarter = nil
	defer func() { serveStarter = old }()
	err := runServe(context.Background(), config.Defaults(), nil)
	if err == nil || !strings.Contains(err.Error(), "not wired") {
		t.Fatalf("unwired serve error = %v; want not-wired failure", err)
	}
}

func TestServeRunWiresResolvedConfig(t *testing.T) {
	old := serveStarter
	defer func() { serveStarter = old }()

	started := make(chan struct{})
	serveStarter = func(ctx context.Context, c *config.Config) error {
		if c.Server.Addr != "127.0.0.1:9787" {
			t.Errorf("starter addr = %q; want 127.0.0.1:9787", c.Server.Addr)
		}
		close(started)
		<-ctx.Done()
		return ctx.Err()
	}

	cfg := config.Defaults()
	oldOut := stdout
	stdout = new(bytes.Buffer)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- runServe(ctx, cfg, []string{"--addr", "127.0.0.1:9787"}) }()

	<-started
	cancel()
	err := <-done
	out := stdout.(*bytes.Buffer).String()
	stdout = oldOut

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("runServe error = %v; want context.Canceled", err)
	}
	if !strings.Contains(out, "127.0.0.1:9787") {
		t.Errorf("listen log = %q; want the resolved address", out)
	}
	if cfg.Server.Addr != "127.0.0.1:8080" {
		t.Errorf("caller config mutated: addr = %q", cfg.Server.Addr)
	}
}
