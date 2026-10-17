package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/lavantien/quotes-manager/internal/config"
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

func TestScreenshotRunWithFakeCapture(t *testing.T) {
	out := filepath.Join(t.TempDir(), "docs", "home.png")
	var capturedURL string
	capture := func(url string) ([]byte, error) {
		capturedURL = url
		resp, err := http.Get(url)
		if err != nil {
			return nil, err
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("status %d", resp.StatusCode)
		}
		return []byte("PNG-FAKE"), nil
	}
	if err := screenshotRun(io.Discard, out, capture); err != nil {
		t.Fatalf("screenshotRun: %v", err)
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

func TestScreenshotRunWithCaptureError(t *testing.T) {
	out := filepath.Join(t.TempDir(), "docs", "home.png")
	fail := func(string) ([]byte, error) { return nil, errors.New("no browser") }
	if err := screenshotRun(io.Discard, out, fail); err == nil {
		t.Error("screenshotRun should surface the capture error")
	}
}

func TestRunScreenshotRejectsUnexpectedArg(t *testing.T) {
	if err := runScreenshot(context.Background(), config.Defaults(), []string{"extra"}); err == nil {
		t.Error("screenshot must reject positional arguments")
	}
}

func TestRunScreenshotOutDefaultFromPicturesDir(t *testing.T) {
	cfg := config.Defaults()
	cfg.Quotes.PicturesDir = filepath.Join(t.TempDir(), "pics")
	oldStarter := screenshotCapture
	screenshotCapture = func(url string) ([]byte, error) { return []byte("PNG-FAKE"), nil }
	defer func() { screenshotCapture = oldStarter }()
	oldOut := stdout
	stdout = io.Discard
	defer func() { stdout = oldOut }()
	if err := runScreenshot(context.Background(), cfg, nil); err != nil {
		t.Fatalf("runScreenshot: %v", err)
	}
	target := filepath.Join(cfg.Quotes.PicturesDir, "home.png")
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("default target %s not written: %v", target, err)
	}
	if string(got) != "PNG-FAKE" {
		t.Errorf("output = %q, want PNG-FAKE", got)
	}
}

func TestCapturePageRunsNavigateThenShot(t *testing.T) {
	var order []string
	got, err := capturePage(context.Background(),
		func(context.Context) error {
			order = append(order, "navigate")
			return nil
		},
		func(context.Context) ([]byte, error) {
			order = append(order, "shot")
			return []byte("png"), nil
		},
	)
	if err != nil {
		t.Fatalf("capturePage: %v", err)
	}
	if string(got) != "png" {
		t.Errorf("capturePage = %q, want png", got)
	}
	if len(order) != 2 || order[0] != "navigate" || order[1] != "shot" {
		t.Errorf("order = %v, want [navigate shot]", order)
	}
}

func TestCapturePageNavigateErrorSkipsShot(t *testing.T) {
	boom := errors.New("navigate failed")
	shotCalled := false
	got, err := capturePage(context.Background(),
		func(context.Context) error { return boom },
		func(context.Context) ([]byte, error) {
			shotCalled = true
			return []byte("png"), nil
		},
	)
	if !errors.Is(err, boom) {
		t.Fatalf("capturePage err = %v, want %v", err, boom)
	}
	if got != nil {
		t.Errorf("capturePage = %q, want nil", got)
	}
	if shotCalled {
		t.Error("shot ran despite navigate failure")
	}
}

func TestFindBrowserPathScansWindowsInstallPaths(t *testing.T) {
	t.Setenv("QUOTES_BROWSER", "")
	t.Setenv("PATH", "")
	edge := `C:\Program Files\Microsoft\Edge\Application\msedge.exe`
	stat := func(p string) (fs.FileInfo, error) {
		if p == edge {
			return nil, nil
		}
		return nil, os.ErrNotExist
	}
	if got := findBrowserPath("windows", stat); got != edge {
		t.Errorf("findBrowserPath(windows) = %q, want %q", got, edge)
	}
	if got := findBrowserPath("linux", stat); got != "" {
		t.Errorf("findBrowserPath(linux) = %q, want empty", got)
	}
}
