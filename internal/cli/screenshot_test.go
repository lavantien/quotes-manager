package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
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

func TestJourneyShotsNumberingAndCoverage(t *testing.T) {
	if len(journeyShots) == 0 {
		t.Fatal("journeyShots must define the canonical journey")
	}
	files := map[string]bool{}
	paths := map[string]bool{}
	for i, s := range journeyShots {
		prefix := fmt.Sprintf("%02d-", i+1)
		if !strings.HasPrefix(s.File, prefix) {
			t.Errorf("journeyShots[%d].File = %q, want %s* prefix", i, s.File, prefix)
		}
		if !strings.HasSuffix(s.File, ".png") {
			t.Errorf("journeyShots[%d].File = %q, want .png suffix", i, s.File)
		}
		if !strings.HasPrefix(s.Path, "/") {
			t.Errorf("journeyShots[%d].Path = %q, want an absolute site path", i, s.Path)
		}
		if s.Wait == "" {
			t.Errorf("journeyShots[%d] has no wait selector", i)
		}
		if files[s.File] {
			t.Errorf("duplicate journey file %q", s.File)
		}
		files[s.File] = true
		if paths[s.Path] {
			t.Errorf("duplicate journey path %q", s.Path)
		}
		paths[s.Path] = true
	}
	var home, collection, search, dashboard, texts, text, words, graph bool
	for _, s := range journeyShots {
		switch {
		case s.Path == "/":
			home = true
		case s.Path == "/?col=1":
			collection = true
		case strings.HasPrefix(s.Path, "/?rq="):
			search = true
		case s.Path == "/dashboard":
			dashboard = true
		case s.Path == "/texts":
			texts = true
		case strings.HasPrefix(s.Path, "/texts/"):
			text = true
		case strings.HasPrefix(s.Path, "/words"):
			words = true
		case strings.HasPrefix(s.Path, "/graph"):
			graph = true
		}
	}
	for name, ok := range map[string]bool{
		"workspace home": home, "workspace collection": collection, "workspace search": search,
		"canon dashboard": dashboard, "canon texts": texts, "canon text page": text,
		"canon words": words, "canon graph": graph,
	} {
		if !ok {
			t.Errorf("journey missing the %s surface", name)
		}
	}
}

func installScreenshotDeps(t *testing.T, baseURL string, capture func(url, wait string) ([]byte, error)) {
	t.Helper()
	oldEnv := screenshotEnv
	screenshotEnv = func(*config.Config) (string, func() error, error) {
		return baseURL, func() error { return nil }, nil
	}
	oldCapture := screenshotCapture
	screenshotCapture = capture
	oldOut := stdout
	stdout = io.Discard
	t.Cleanup(func() {
		screenshotEnv = oldEnv
		screenshotCapture = oldCapture
		stdout = oldOut
	})
}

func TestRunScreenshotWritesNumberedJourney(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "pics")
	cfg := config.Defaults()
	cfg.Quotes.PicturesDir = dir
	var seen []string
	installScreenshotDeps(t, "http://127.0.0.1:1", func(url, wait string) ([]byte, error) {
		seen = append(seen, url+" "+wait)
		return []byte("PNG-" + wait), nil
	})
	if err := runScreenshot(context.Background(), cfg, nil); err != nil {
		t.Fatalf("runScreenshot: %v", err)
	}
	if len(seen) != len(journeyShots) {
		t.Fatalf("captures = %d, want %d", len(seen), len(journeyShots))
	}
	for i, s := range journeyShots {
		want := "http://127.0.0.1:1" + s.Path + " " + s.Wait
		if seen[i] != want {
			t.Errorf("capture %d = %q, want %q", i, seen[i], want)
		}
		target := filepath.Join(dir, s.File)
		raw, err := os.ReadFile(target)
		if err != nil {
			t.Errorf("%s not written: %v", s.File, err)
			continue
		}
		if string(raw) != "PNG-"+s.Wait {
			t.Errorf("%s = %q, want PNG-%s", s.File, raw, s.Wait)
		}
	}
}

func TestRunScreenshotOutFlagOverridesPicturesDir(t *testing.T) {
	cfg := config.Defaults()
	cfg.Quotes.PicturesDir = filepath.Join(t.TempDir(), "default-pics")
	dir := filepath.Join(t.TempDir(), "override-pics")
	installScreenshotDeps(t, "http://127.0.0.1:1", func(_, wait string) ([]byte, error) {
		return []byte("PNG-" + wait), nil
	})
	if err := runScreenshot(context.Background(), cfg, []string{"--out", dir}); err != nil {
		t.Fatalf("runScreenshot: %v", err)
	}
	for _, s := range journeyShots {
		if _, err := os.Stat(filepath.Join(dir, s.File)); err != nil {
			t.Errorf("--out %s missing %s: %v", dir, s.File, err)
		}
		if _, err := os.Stat(filepath.Join(cfg.Quotes.PicturesDir, s.File)); !os.IsNotExist(err) {
			t.Errorf("pictures dir must stay untouched, stat err = %v", err)
		}
	}
}

func TestRunScreenshotUnwiredEnvFails(t *testing.T) {
	oldEnv := screenshotEnv
	screenshotEnv = nil
	defer func() { screenshotEnv = oldEnv }()
	err := runScreenshot(context.Background(), config.Defaults(), nil)
	if err == nil || !strings.Contains(err.Error(), "not wired") {
		t.Errorf("runScreenshot without an env builder = %v, want a not-wired error", err)
	}
}

func TestRunScreenshotCaptureErrorStopsJourney(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "pics")
	installScreenshotDeps(t, "http://127.0.0.1:1", func(string, string) ([]byte, error) {
		return nil, errors.New("no browser")
	})
	err := runScreenshot(context.Background(), &config.Config{Quotes: config.Quotes{PicturesDir: dir}}, nil)
	if err == nil {
		t.Fatal("runScreenshot must fail when every capture fails")
	}
	if !strings.Contains(err.Error(), journeyShots[0].File) {
		t.Errorf("error %v must name the failing shot %s", err, journeyShots[0].File)
	}
	if _, statErr := os.Stat(filepath.Join(dir, journeyShots[0].File)); !os.IsNotExist(statErr) {
		t.Errorf("a failed capture must not write %s", journeyShots[0].File)
	}
}

func TestRunScreenshotRejectsUnexpectedArg(t *testing.T) {
	if err := runScreenshot(context.Background(), config.Defaults(), []string{"extra"}); err == nil {
		t.Error("screenshot must reject positional arguments")
	}
}

func TestCaptureRetrySucceedsOnSecondAttempt(t *testing.T) {
	calls := 0
	capture := func(string, string) ([]byte, error) {
		calls++
		if calls == 1 {
			return nil, errors.New("browser hiccup")
		}
		return []byte("png"), nil
	}
	buf, err := captureRetry(io.Discard, Shot{File: "01-x.png", Wait: "#a"}, "http://x/", capture)
	if err != nil {
		t.Fatalf("captureRetry: %v", err)
	}
	if calls != 2 || string(buf) != "png" {
		t.Errorf("captureRetry calls = %d buf = %q, want 2 and png", calls, buf)
	}
}

func TestCaptureRetryFailsAfterTwoAttempts(t *testing.T) {
	calls := 0
	capture := func(string, string) ([]byte, error) {
		calls++
		return nil, errors.New("no browser")
	}
	_, err := captureRetry(io.Discard, Shot{File: "02-y.png", Wait: "#b"}, "http://x/", capture)
	if err == nil || !strings.Contains(err.Error(), "02-y.png") {
		t.Fatalf("captureRetry err = %v, want a failure naming 02-y.png", err)
	}
	if calls != 2 {
		t.Errorf("captureRetry calls = %d, want exactly 2", calls)
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
