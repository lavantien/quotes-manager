package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"time"

	"github.com/chromedp/chromedp"

	"github.com/lavantien/quotes-manager/internal/config"
)

const (
	viewportW     = 1440
	viewportH     = 900
	captureBound  = 120 * time.Second
	captureSettle = 300 * time.Millisecond
)

type ShotStep struct {
	Action string
	Sel    string
	Value  string
}

type Shot struct {
	File  string
	Path  string
	Wait  string
	Steps []ShotStep
}

type captureFunc func(url, wait string, steps []ShotStep) ([]byte, error)

func Journey() []Shot { return slices.Clone(journeyShots) }

var journeyShots = []Shot{
	{File: "01-workspace-home.png", Path: "/", Wait: "#quote-list"},
	{File: "02-workspace-category.png", Path: "/?cat=1", Wait: ".quote"},
	{File: "03-workspace-collection.png", Path: "/?col=1", Wait: "#collection-zone"},
	{File: "04-workspace-search.png", Path: "/?rq=buddha", Wait: "#quote-list"},
	{File: "05-workspace-new-form.png", Path: "/", Wait: ".quote-form", Steps: []ShotStep{
		{Action: "click", Sel: `button[hx-get="/quotes/new"]`},
	}},
	{File: "06-workspace-import-form.png", Path: "/", Wait: "#import-form", Steps: []ShotStep{
		{Action: "click", Sel: `button[hx-get="/quotes/import/form"]`},
	}},
	{File: "07-workspace-check-ids.png", Path: "/", Wait: ".check__list", Steps: []ShotStep{
		{Action: "click", Sel: `button[hx-get^="/pane/check"]`},
		{Action: "fill", Sel: "#check-input", Value: "MN 22\nMN 51\nXX 9.9"},
		{Action: "click", Sel: `.check-form button[type="submit"]`},
	}},
	{File: "08-workspace-edit-form.png", Path: "/", Wait: ".quote-form", Steps: []ShotStep{
		{Action: "click", Sel: `button[hx-get$="/edit"]`},
	}},
	{File: "09-canon-dashboard.png", Path: "/dashboard", Wait: "section.stats"},
	{File: "10-canon-texts.png", Path: "/texts", Wait: "ul.tree"},
	{File: "11-canon-text.png", Path: "/texts/mn1", Wait: ".segment"},
	{File: "12-canon-search.png", Path: "/search?text=deep&lang=en", Wait: ".hit"},
	{File: "13-canon-words.png", Path: "/words?form=dhamma", Wait: ".entry"},
	{File: "14-canon-word.png", Path: "/words/eva", Wait: ".entry"},
	{File: "15-canon-graph-word.png", Path: "/graph?dataset=word&param=dhamma", Wait: "#graph-canvas canvas"},
	{File: "16-canon-graph-coverage.png", Path: "/graph?dataset=coverage&param=mn", Wait: "#graph-canvas canvas"},
	{File: "17-canon-analytics.png", Path: "/analytics?aggregate=top-headwords", Wait: ".table"},
	{File: "18-canon-pipeline.png", Path: "/pipeline", Wait: "tr.log-line", Steps: []ShotStep{
		{Action: "click", Sel: ".stage-button"},
	}},
	{File: "19-canon-discrepancies.png", Path: "/discrepancies", Wait: ".table"},
}

func init() {
	Register(Command{Name: "screenshot", Usage: "capture the numbered UI journey into project-pictures: --out dir", Run: runScreenshot})
}

var (
	screenshotEnv     func(cfg *config.Config) (string, func() error, error)
	screenshotCapture captureFunc = captureScreenshot
)

func SetScreenshotEnv(f func(cfg *config.Config) (string, func() error, error)) {
	screenshotEnv = f
}

func runScreenshot(_ context.Context, cfg *config.Config, args []string) error {
	fs := flag.NewFlagSet("screenshot", flag.ContinueOnError)
	out := fs.String("out", cfg.Quotes.PicturesDir, "output directory for the numbered journey PNG files")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("screenshot: unexpected argument %q", fs.Arg(0))
	}
	if screenshotEnv == nil {
		return fmt.Errorf("screenshot: merged server not wired (run via cmd/qm)")
	}
	baseURL, closeEnv, err := screenshotEnv(cfg)
	if err != nil {
		return fmt.Errorf("screenshot: %w", err)
	}
	defer func() { _ = closeEnv() }()
	if err := writeJourney(stdout, *out, baseURL, screenshotCapture); err != nil {
		return fmt.Errorf("screenshot: %w", err)
	}
	return nil
}

func writeJourney(out io.Writer, dir, baseURL string, capture captureFunc) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}
	for _, shot := range journeyShots {
		buf, err := captureRetry(out, shot, baseURL+shot.Path, capture)
		if err != nil {
			return err
		}
		target := filepath.Join(dir, shot.File)
		if err := os.WriteFile(target, buf, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", target, err)
		}
		if _, err := fmt.Fprintf(out, "wrote %s (%d bytes)\n", filepath.ToSlash(target), len(buf)); err != nil {
			return err
		}
	}
	return nil
}

func captureRetry(out io.Writer, shot Shot, url string, capture captureFunc) ([]byte, error) {
	buf, err := capture(url, shot.Wait, shot.Steps)
	if err == nil {
		return buf, nil
	}
	_, _ = fmt.Fprintf(out, "capture %s (%s) attempt 1 failed: %v; retrying once\n", shot.File, url, err)
	buf, err = capture(url, shot.Wait, shot.Steps)
	if err != nil {
		return nil, fmt.Errorf("capture %s (%s) failed after retry: %w", shot.File, url, err)
	}
	return buf, nil
}

func stepActions(steps []ShotStep) ([]chromedp.Action[chromedp.Void], error) {
	acts := make([]chromedp.Action[chromedp.Void], 0, len(steps))
	for _, st := range steps {
		switch st.Action {
		case "click":
			acts = append(acts, chromedp.Click(chromedp.CSS(st.Sel)))
		case "fill":
			acts = append(acts, chromedp.SendKeys(chromedp.CSS(st.Sel), st.Value))
		default:
			return nil, fmt.Errorf("screenshot step %q: unsupported action", st.Action)
		}
	}
	return acts, nil
}

func captureScreenshot(url, wait string, steps []ShotStep) ([]byte, error) {
	ctx, cancel := browserContext()
	defer cancel()
	acts, err := stepActions(steps)
	if err != nil {
		return nil, err
	}
	return capturePage(ctx,
		func(ctx context.Context) error {
			run := []chromedp.Action[chromedp.Void]{chromedp.EmulateViewport(viewportW, viewportH), chromedp.Navigate(url)}
			run = append(run, acts...)
			run = append(run, chromedp.WaitVisible(chromedp.CSS(wait)), chromedp.Sleep(captureSettle))
			return chromedp.Do(ctx, run...)
		},
		func(ctx context.Context) ([]byte, error) {
			return chromedp.Run(ctx, chromedp.CaptureScreenshot())
		},
	)
}

func capturePage(ctx context.Context, navigate func(context.Context) error, shot func(context.Context) ([]byte, error)) ([]byte, error) {
	if err := navigate(ctx); err != nil {
		return nil, err
	}
	return shot(ctx)
}

func browserContext() (context.Context, context.CancelFunc) {
	allocOpts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("hide-scrollbars", "true"),
		chromedp.ExecPath(browserPath()),
	)
	ctx, cancelAlloc := chromedp.NewExecAllocator(context.Background(), allocOpts...)
	ctx, cancelBrowser := chromedp.NewContext(ctx)
	ctx, cancelTimeout := context.WithTimeout(ctx, captureBound)
	return ctx, func() {
		cancelTimeout()
		cancelBrowser()
		cancelAlloc()
	}
}

func browserPath() string {
	return findBrowserPath(runtime.GOOS, os.Stat)
}

var windowsBrowserPaths = []string{
	`C:\Program Files\Google\Chrome\Application\chrome.exe`,
	`C:\Program Files (x86)\Google\Chrome\Application\chrome.exe`,
	`C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe`,
	`C:\Program Files\Microsoft\Edge\Application\msedge.exe`,
}

func findBrowserPath(goos string, stat func(string) (fs.FileInfo, error)) string {
	if bin := os.Getenv("QUOTES_BROWSER"); bin != "" {
		return bin
	}
	for _, name := range []string{"chrome", "chromium", "chromium-browser", "google-chrome"} {
		if path, err := exec.LookPath(name); err == nil {
			return path
		}
	}
	if goos != "windows" {
		return ""
	}
	for _, p := range windowsBrowserPaths {
		if _, err := stat(p); err == nil {
			return p
		}
	}
	return ""
}
