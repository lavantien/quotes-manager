package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/chromedp/chromedp"

	"github.com/lavantien/quotes-manager/internal/config"
	"github.com/lavantien/quotes-manager/internal/seed"
	"github.com/lavantien/quotes-manager/internal/server"
	"github.com/lavantien/quotes-manager/internal/store"
)

const (
	viewportW = 1440
	viewportH = 900
)

func init() {
	Register(Command{Name: "screenshot", Usage: "capture the home page: --out", Run: runScreenshot})
}

var screenshotCapture = captureScreenshot

func runScreenshot(_ context.Context, cfg *config.Config, args []string) error {
	fs := flag.NewFlagSet("screenshot", flag.ContinueOnError)
	out := fs.String("out", filepath.Join(cfg.Quotes.PicturesDir, "home.png"), "output PNG path")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("screenshot: unexpected argument %q", fs.Arg(0))
	}
	if err := screenshotRun(stdout, *out, screenshotCapture); err != nil {
		return fmt.Errorf("screenshot: %w", err)
	}
	return nil
}

func screenshotRun(out io.Writer, target string, capture func(url string) ([]byte, error)) error {
	tmp, err := os.CreateTemp("", "qm-screenshot-*.db")
	if err != nil {
		return fmt.Errorf("temp db: %w", err)
	}
	tmpPath := tmp.Name()
	if err := tmp.Close(); err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmpPath) }()

	st, err := store.Open(tmpPath)
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer func() { _ = st.Close() }()
	if err := seed.EnsureSeeded(st.DB()); err != nil {
		return fmt.Errorf("seed: %w", err)
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	srv := &http.Server{Handler: server.New(st)}
	go func() { _ = srv.Serve(ln) }()
	defer func() { _ = srv.Shutdown(context.Background()) }()
	url := "http://" + ln.Addr().String() + "/?col=1"

	buf, err := capture(url)
	if err != nil {
		return fmt.Errorf("capture: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(target), err)
	}
	if err := os.WriteFile(target, buf, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", target, err)
	}
	_, err = fmt.Fprintf(out, "wrote %s (%d bytes)\n", filepath.ToSlash(target), len(buf))
	return err
}

func captureScreenshot(url string) ([]byte, error) {
	ctx, cancel := browserContext()
	defer cancel()
	return capturePage(ctx,
		func(ctx context.Context) error {
			return chromedp.Do(ctx,
				chromedp.EmulateViewport(viewportW, viewportH),
				chromedp.Navigate(url),
				chromedp.WaitVisible(chromedp.ID("quote-list")),
				chromedp.Sleep(300*time.Millisecond),
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
	ctx, cancelTimeout := context.WithTimeout(ctx, 60*time.Second)
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
