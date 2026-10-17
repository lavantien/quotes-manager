package main

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lavantien/quotes-manager/internal/config"
)

func TestServeOpenFails(t *testing.T) {
	cfg := config.Defaults()
	cfg.Quotes.DBPath = filepath.Join(t.TempDir(), "no", "such", "dir", "db.sqlite")
	if err := serveMerged(context.Background(), cfg); err == nil {
		t.Error("serveMerged should fail when the DB path cannot be opened")
	}
}

func TestServeListenFails(t *testing.T) {
	cfg := config.Defaults()
	cfg.Quotes.DBPath = filepath.Join(t.TempDir(), "db.sqlite")
	cfg.Server.Addr = "not a valid:addr"
	if err := serveMerged(context.Background(), cfg); err == nil {
		t.Error("serveMerged should fail to listen on a bad address")
	}
}

func startMerged(t *testing.T) (addr string, stop func() error) {
	t.Helper()
	cfg := config.Defaults()
	cfg.Quotes.DBPath = filepath.Join(t.TempDir(), "db.sqlite")
	probe, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr = probe.Addr().String()
	if err := probe.Close(); err != nil {
		t.Fatal(err)
	}
	cfg.Server.Addr = addr

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- serveMerged(ctx, cfg) }()
	return addr, func() error {
		cancel()
		select {
		case err := <-done:
			return err
		case <-time.After(10 * time.Second):
			t.Fatal("serveMerged did not shut down after ctx cancel")
			return nil
		}
	}
}

func waitForServer(t *testing.T, client *http.Client, url string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		resp, err := client.Get(url)
		if err == nil {
			if err := resp.Body.Close(); err != nil {
				t.Fatal(err)
			}
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("%s status = %d, want 200", url, resp.StatusCode)
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("server never came up on %s: %v", url, err)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func TestServeServesAndShutsDown(t *testing.T) {
	addr, stop := startMerged(t)
	client := &http.Client{Timeout: 2 * time.Second}
	waitForServer(t, client, "http://"+addr+"/?col=1")
	if err := stop(); err != nil {
		t.Fatalf("serveMerged returned %v on graceful shutdown", err)
	}
}

func TestServeMergedUnknownPaths(t *testing.T) {
	addr, stop := startMerged(t)
	defer func() {
		if err := stop(); err != nil {
			t.Fatalf("serveMerged returned %v on graceful shutdown", err)
		}
	}()
	client := &http.Client{Timeout: 2 * time.Second}
	base := "http://" + addr
	waitForServer(t, client, base+"/")

	resp, err := client.Get(base + "/no-such-page")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("GET /no-such-page = %d, want 404", resp.StatusCode)
	}
	if !strings.Contains(string(raw), `href="/dashboard"`) {
		t.Errorf("404 body missing the dashboard back-link")
	}
	if strings.Contains(string(raw), `id="quote-list"`) {
		t.Errorf("404 body served the quotes home")
	}

	req, err := http.NewRequest(http.MethodPost, base+"/no-such-page", strings.NewReader(""))
	if err != nil {
		t.Fatal(err)
	}
	resp, err = client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("POST /no-such-page = %d, want 405", resp.StatusCode)
	}
	if allow := resp.Header.Get("Allow"); !strings.Contains(allow, "GET") {
		t.Errorf("POST /no-such-page: Allow = %q, want GET listed", allow)
	}
}

func TestServeMergedComposition(t *testing.T) {
	addr, stop := startMerged(t)
	defer func() {
		if err := stop(); err != nil {
			t.Fatalf("serveMerged returned %v on graceful shutdown", err)
		}
	}()
	client := &http.Client{Timeout: 2 * time.Second}
	base := "http://" + addr

	waitForServer(t, client, base+"/")

	for _, tc := range []struct{ path, marker string }{
		{"/dashboard", `class="nav-brand" href="/">qm</a>`},
		{"/texts", "texts"},
		{"/static/canon.css", "--bg"},
		{"/static/nav.css", ".nav"},
		{"/static/htmx.min.js", "htmx"},
		{"/static/cytoscape.min.js", "cytoscape"},
	} {
		resp, err := client.Get(base + tc.path)
		if err != nil {
			t.Fatalf("%s: %v", tc.path, err)
		}
		raw, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			t.Fatalf("%s: read body: %v", tc.path, err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("%s status = %d", tc.path, resp.StatusCode)
		}
		if !strings.Contains(string(raw), tc.marker) {
			t.Errorf("%s missing marker %q", tc.path, tc.marker)
		}
	}

	form := url.Values{"text_id": {"mn1"}, "content": {"test passage"}, "attribution": {"the Buddha"}}
	req, err := http.NewRequest(http.MethodPost, base+"/quotes", strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", "http://evil.example")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("cross-origin POST = %d, want 403", resp.StatusCode)
	}

	req, err = http.NewRequest(http.MethodPost, base+"/quotes", strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", "http://"+addr)
	resp, err = client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("same-origin POST = %d, want 200", resp.StatusCode)
	}
}
