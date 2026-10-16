package main

import (
	"context"
	"net"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/lavantien/quotes-manager/internal/config"
)

func TestServeQuotesOpenFails(t *testing.T) {
	cfg := config.Defaults()
	cfg.Quotes.DBPath = filepath.Join(t.TempDir(), "no", "such", "dir", "db.sqlite")
	if err := serveQuotes(context.Background(), cfg); err == nil {
		t.Error("serveQuotes should fail when the DB path cannot be opened")
	}
}

func TestServeQuotesListenFails(t *testing.T) {
	cfg := config.Defaults()
	cfg.Quotes.DBPath = filepath.Join(t.TempDir(), "db.sqlite")
	cfg.Server.Addr = "not a valid:addr"
	if err := serveQuotes(context.Background(), cfg); err == nil {
		t.Error("serveQuotes should fail to listen on a bad address")
	}
}

func TestServeQuotesServesAndShutsDown(t *testing.T) {
	cfg := config.Defaults()
	cfg.Quotes.DBPath = filepath.Join(t.TempDir(), "db.sqlite")
	probe, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := probe.Addr().String()
	if err := probe.Close(); err != nil {
		t.Fatal(err)
	}
	cfg.Server.Addr = addr

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- serveQuotes(ctx, cfg) }()

	client := &http.Client{Timeout: 2 * time.Second}
	deadline := time.Now().Add(5 * time.Second)
	for {
		resp, err := client.Get("http://" + addr + "/?col=1")
		if err == nil {
			if err := resp.Body.Close(); err != nil {
				t.Fatal(err)
			}
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("home status = %d, want 200", resp.StatusCode)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("server never came up on %s: %v", addr, err)
		}
		time.Sleep(50 * time.Millisecond)
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("serveQuotes returned %v on graceful shutdown", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("serveQuotes did not shut down after ctx cancel")
	}
}
