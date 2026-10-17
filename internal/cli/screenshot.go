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

type Shot struct {
	File string
	Path string
	Wait string
}

func Journey() []Shot { return slices.Clone(journeyShots) }

var journeyShots = []Shot{
	{File: "01-workspace-home.png", Path: "/", Wait: "#quote-list"},
	{File: "02-workspace-collection.png", Path: "/?col=1", Wait: "#collection-zone"},
	{File: "03-workspace-search.png", Path: "/?rq=buddha", Wait: "#quote-list"},
	{File: "04-canon-dashboard.png", Path: "/dashboard", Wait: "section.stats"},
	{File: "05-canon-texts.png", Path: "/texts", Wait: "ul.tree"},
	{File: "06-canon-text.png", Path: "/texts/mn1", Wait: ".segment"},
	{File: "07-canon-words.png", Path: "/words?form=dhamma", Wait: ".entry"},
	{File: "08-canon-graph.png", Path: "/graph?dataset=word&param=dhamma", Wait: "#graph-canvas canvas"},
}

func init() {
	Register(Command{Name: "screenshot", Usage: "capture the numbered UI journey into project-pictures: --out dir", Run: runScreenshot})
}

var (
	screenshotEnv     func(cfg *config.Config) (string, func() error, error)
	screenshotCapture = captureScreenshot
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

func writeJourney(out io.Writer, dir, baseURL string, capture func(url, wait string) ([]byte, error)) error {
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

func captureRetry(out io.Writer, shot Shot, url string, capture func(url, wait string) ([]byte, error)) ([]byte, error) {
	buf, err := capture(url, shot.Wait)
	if err == nil {
		return buf, nil
	}
	_, _ = fmt.Fprintf(out, "capture %s (%s) attempt 1 failed: %v; retrying once\n", shot.File, url, err)
	buf, err = capture(url, shot.Wait)
	if err != nil {
		return nil, fmt.Errorf("capture %s (%s) failed after retry: %w", shot.File, url, err)
	}
	return buf, nil
}

func captureScreenshot(url, wait string) ([]byte, error) {
	ctx, cancel := browserContext()
	defer cancel()
	return capturePage(ctx,
		func(ctx context.Context) error {
			return chromedp.Do(ctx,
				chromedp.EmulateViewport(viewportW, viewportH),
				chromedp.Navigate(url),
				chromedp.WaitVisible(chromedp.CSS(wait)),
				chromedp.Sleep(captureSettle),
			)
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
