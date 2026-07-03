package main

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestBrowserPathEnvOverride(t *testing.T) {
	t.Setenv("QUOTES_BROWSER", "/custom/browser")
	if got := browserPath(); got != "/custom/browser" {
		t.Errorf("browserPath = %q, want /custom/browser", got)
	}
}

func TestBrowserPathFindsOnPATH(t *testing.T) {
	t.Setenv("QUOTES_BROWSER", "")
	dir := t.TempDir()
	name := "chrome"
	if runtime.GOOS == "windows" {
		name = "chrome.exe"
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte("fake"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	got := browserPath()
	if got == "" {
		t.Fatal("browserPath should find chrome on PATH")
	}
	if base := strings.ToLower(filepath.Base(got)); !strings.HasPrefix(base, "chrome") {
		t.Errorf("browserPath = %q, want a chrome binary", got)
	}
}

func TestBrowserPathEmptyWhenAbsent(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows scans fixed install paths; not deterministic")
	}
	t.Setenv("QUOTES_BROWSER", "")
	t.Setenv("PATH", "")
	if got := browserPath(); got != "" {
		t.Errorf("browserPath = %q, want empty when nothing is installed", got)
	}
}

// TestBrowserPathWindowsScan exercises the Windows install-path scan. It makes
// no strict assertion (Chrome/Edge may or may not be installed) but verifies any
// discovered binary actually exists.
func TestBrowserPathWindowsScan(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows install-path scan only")
	}
	t.Setenv("QUOTES_BROWSER", "")
	t.Setenv("PATH", "")
	if got := browserPath(); got != "" {
		if _, err := os.Stat(got); err != nil {
			t.Errorf("browserPath returned %q which does not exist: %v", got, err)
		}
	}
}

// TestRunWithFakeCapture drives the full non-browser flow: a temp DB is opened
// and seeded, the app is served on an ephemeral port, the (faked) capture hits
// it over HTTP, and the bytes are written to the target file.
func TestRunWithFakeCapture(t *testing.T) {
	out := filepath.Join(t.TempDir(), "docs", "home.png")
	var capturedURL string
	capture := func(url string) ([]byte, error) {
		capturedURL = url
		resp, err := http.Get(url)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("status %d", resp.StatusCode)
		}
		return []byte("PNG-FAKE"), nil
	}
	if err := runWith(io.Discard, out, capture); err != nil {
		t.Fatalf("runWith: %v", err)
	}
	if !strings.Contains(capturedURL, "/?col=1") {
		t.Errorf("capture URL = %q, want /?col=1", capturedURL)
	}
	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("output not written: %v", err)
	}
	if string(got) != "PNG-FAKE" {
		t.Errorf("output = %q, want PNG-FAKE", got)
	}
}

func TestRunWithCaptureError(t *testing.T) {
	out := filepath.Join(t.TempDir(), "docs", "home.png")
	fail := func(string) ([]byte, error) { return nil, errors.New("no browser") }
	if err := runWith(io.Discard, out, fail); err == nil {
		t.Error("runWith should surface the capture error")
	}
}
