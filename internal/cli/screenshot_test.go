package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/lavantien/quotes-manager/internal/config"
)

var readmePictureRe = regexp.MustCompile(`project-pictures/([0-9]{2}-[a-z0-9-]+\.png)`)

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
		if len(s.Steps) > 0 {
			continue
		}
		if paths[s.Path] {
			t.Errorf("duplicate plain journey path %q", s.Path)
		}
		paths[s.Path] = true
	}
	var home, category, collection, search, dashboard, texts, text, canonSearch, words, word bool
	var wordGraph, coverageGraph, analytics, pipeline, discrepancies bool
	for _, s := range journeyShots {
		switch {
		case s.Path == "/":
			home = true
		case strings.HasPrefix(s.Path, "/?cat="):
			category = true
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
		case strings.HasPrefix(s.Path, "/search"):
			canonSearch = true
		case strings.HasPrefix(s.Path, "/words"):
			if strings.HasPrefix(s.Path, "/words/") {
				word = true
			} else {
				words = true
			}
		case strings.HasPrefix(s.Path, "/graph?dataset=word"):
			wordGraph = true
		case strings.HasPrefix(s.Path, "/graph?dataset=coverage"):
			coverageGraph = true
		case strings.HasPrefix(s.Path, "/analytics"):
			analytics = true
		case s.Path == "/pipeline":
			pipeline = true
		case s.Path == "/discrepancies":
			discrepancies = true
		}
	}
	for name, ok := range map[string]bool{
		"workspace home": home, "workspace category": category,
		"workspace collection": collection, "workspace search": search,
		"canon dashboard": dashboard, "canon texts": texts, "canon text page": text,
		"canon search": canonSearch, "canon words": words, "canon word page": word,
		"canon word graph": wordGraph, "canon coverage graph": coverageGraph,
		"canon analytics": analytics, "canon pipeline": pipeline,
		"canon discrepancies": discrepancies,
	} {
		if !ok {
			t.Errorf("journey missing the %s surface", name)
		}
	}
}

func TestJourneyShotsMatchReadmePictures(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "readme.md"))
	if err != nil {
		t.Fatalf("read readme: %v", err)
	}
	matches := readmePictureRe.FindAllStringSubmatch(string(raw), -1)
	if len(matches) == 0 {
		t.Fatal("readme references no project-pictures PNG")
	}
	if len(matches) != len(journeyShots) {
		t.Fatalf("readme pictures = %d, journeyShots = %d, want equal counts", len(matches), len(journeyShots))
	}
	for i, m := range matches {
		if journeyShots[i].File != m[1] {
			t.Errorf("journeyShots[%d].File = %q, readme wants %q", i, journeyShots[i].File, m[1])
		}
	}
}

func TestJourneyShotsMatchDiskPictures(t *testing.T) {
	entries, err := os.ReadDir(filepath.Join("..", "..", "project-pictures"))
	if err != nil {
		t.Fatalf("read project-pictures: %v", err)
	}
	want := map[string]bool{}
	for _, s := range journeyShots {
		want[s.File] = true
	}
	onDisk := map[string]bool{}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".png" {
			continue
		}
		info, err := e.Info()
		if err != nil {
			t.Fatalf("stat %s: %v", e.Name(), err)
		}
		if info.Size() == 0 {
			t.Errorf("project-pictures/%s is zero bytes", e.Name())
		}
		onDisk[e.Name()] = true
	}
	for name := range onDisk {
		if !want[name] {
			t.Errorf("project-pictures holds orphan %s not in journeyShots", name)
		}
	}
	for name := range want {
		if !onDisk[name] {
			t.Errorf("journeyShots wants %s missing from project-pictures", name)
		}
	}
}

func TestJourneyInteractiveShotsCarrySteps(t *testing.T) {
	interactive := map[string]bool{
		"05-workspace-new-form.png":    true,
		"06-workspace-import-form.png": true,
		"07-workspace-check-ids.png":   true,
		"08-workspace-edit-form.png":   true,
		"18-canon-pipeline.png":        true,
	}
	seen := 0
	for _, s := range journeyShots {
		if interactive[s.File] {
			seen++
			if len(s.Steps) == 0 {
				t.Errorf("%s must define interaction steps", s.File)
			}
			continue
		}
		if len(s.Steps) != 0 {
			t.Errorf("%s must navigate and wait only, got %d steps", s.File, len(s.Steps))
		}
	}
	if seen != len(interactive) {
		t.Errorf("interactive frames found = %d, want %d", seen, len(interactive))
	}
}

func TestJourneyCheckIdsSteps(t *testing.T) {
	var shot *Shot
	for i := range journeyShots {
		if journeyShots[i].File == "07-workspace-check-ids.png" {
			shot = &journeyShots[i]
		}
	}
	if shot == nil {
		t.Fatal("journey missing 07-workspace-check-ids.png")
	}
	want := []ShotStep{
		{Action: "click", Sel: `button[hx-get^="/pane/check"]`},
		{Action: "fill", Sel: "#check-input", Value: "MN 22\nMN 51\nXX 9.9"},
		{Action: "click", Sel: `.check-form button[type="submit"]`},
	}
	if !reflect.DeepEqual(shot.Steps, want) {
		t.Errorf("07-workspace-check-ids steps = %#v, want %#v", shot.Steps, want)
	}
}

func installScreenshotDeps(t *testing.T, baseURL string, capture captureFunc) {
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
	installScreenshotDeps(t, "http://127.0.0.1:1", func(url, wait string, steps []ShotStep) ([]byte, error) {
		seen = append(seen, url+" "+wait+" "+fmt.Sprint(len(steps)))
		return []byte("PNG-" + wait), nil
	})
	if err := runScreenshot(context.Background(), cfg, nil); err != nil {
		t.Fatalf("runScreenshot: %v", err)
	}
	if len(seen) != len(journeyShots) {
		t.Fatalf("captures = %d, want %d", len(seen), len(journeyShots))
	}
	for i, s := range journeyShots {
		want := "http://127.0.0.1:1" + s.Path + " " + s.Wait + " " + fmt.Sprint(len(s.Steps))
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
	installScreenshotDeps(t, "http://127.0.0.1:1", func(_, wait string, _ []ShotStep) ([]byte, error) {
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
	installScreenshotDeps(t, "http://127.0.0.1:1", func(string, string, []ShotStep) ([]byte, error) {
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
	capture := func(string, string, []ShotStep) ([]byte, error) {
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
	capture := func(string, string, []ShotStep) ([]byte, error) {
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

func TestStepActionsBuildsClickAndFill(t *testing.T) {
	acts, err := stepActions([]ShotStep{
		{Action: "click", Sel: "button.a"},
		{Action: "fill", Sel: "#b", Value: "MN 22\nMN 51"},
		{Action: "click", Sel: "button.c"},
	})
	if err != nil {
		t.Fatalf("stepActions: %v", err)
	}
	if len(acts) != 3 {
		t.Errorf("stepActions len = %d, want 3", len(acts))
	}
}

func TestStepActionsRejectsUnknownAction(t *testing.T) {
	_, err := stepActions([]ShotStep{{Action: "hover", Sel: "#a"}})
	if err == nil || !strings.Contains(err.Error(), "hover") {
		t.Errorf("stepActions unknown action err = %v, want an error naming hover", err)
	}
	acts, err := stepActions(nil)
	if err != nil || len(acts) != 0 {
		t.Errorf("stepActions(nil) = %v, %v, want empty and nil", acts, err)
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
