// Command screenshot serves the seeded quotes-manager web app in-process on an
// ephemeral port and captures a viewport screenshot of the home page to
// docs/home.png for the README. A Chromium binary is required: set QUOTES_BROWSER
// to its path if auto-detection does not find Chrome or Edge. Run via
// `make screenshot` (CGO is needed for the SQLite driver).
package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/chromedp/chromedp"

	"github.com/lavantien/quotes-manager/internal/seed"
	"github.com/lavantien/quotes-manager/internal/server"
	"github.com/lavantien/quotes-manager/internal/store"
)

const (
	viewportW = 1440
	viewportH = 900
	outFile   = "docs/home.png"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("screenshot: %v", err)
	}
}

func run() error {
	return runWith(os.Stdout, outFile, captureScreenshot)
}

// runWith serves the canonical seed in-process so the screenshot is reproducible
// and independent of any user edits in database/quotes.db, captures the home page
// via capture, and writes it to outFile. capture is injected so the non-browser
// flow is testable.
func runWith(out io.Writer, target string, capture func(url string) ([]byte, error)) error {
	tmp, err := os.CreateTemp("", "qm-screenshot-*.db")
	if err != nil {
		return fmt.Errorf("temp db: %w", err)
	}
	tmpPath := tmp.Name()
	tmp.Close()
	defer os.Remove(tmpPath)

	st, err := store.Open(tmpPath)
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer st.Close()
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
	// The seed provisions one sample collection (id 1); capture the page with it
	// active so the dual-pane screenshot shows a populated collection column.
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
	fmt.Fprintf(out, "wrote %s (%d bytes)\n", filepath.ToSlash(target), len(buf))
	return nil
}

// captureScreenshot drives Chromium to capture a viewport screenshot of url.
func captureScreenshot(url string) ([]byte, error) {
	allocOpts := append(chromedp.DefaultExecAllocatorOptions[:], chromedp.Flag("hide-scrollbars", "true"))
	if bin := browserPath(); bin != "" {
		allocOpts = append(allocOpts, chromedp.ExecPath(bin))
	}
	ctx, cancel := chromedp.NewExecAllocator(context.Background(), allocOpts...)
	defer cancel()
	ctx, cancel = chromedp.NewContext(ctx)
	defer cancel()
	ctx, cancel = context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	var buf []byte
	if err := chromedp.Run(ctx,
		chromedp.EmulateViewport(viewportW, viewportH),
		chromedp.Navigate(url),
		chromedp.WaitVisible("#quote-list", chromedp.ByID),
		chromedp.Sleep(300*time.Millisecond), // let CSS/layout settle
		chromedp.CaptureScreenshot(&buf),
	); err != nil {
		return nil, err
	}
	return buf, nil
}

// browserPath resolves a Chromium binary: the QUOTES_BROWSER env var, then
// chrome/chromium on PATH, then common Windows Chrome/Edge install locations.
// Returns "" to let chromedp fall back to its own lookup.
func browserPath() string {
	if bin := os.Getenv("QUOTES_BROWSER"); bin != "" {
		return bin
	}
	for _, name := range []string{"chrome", "chromium", "chromium-browser", "google-chrome"} {
		if path, err := exec.LookPath(name); err == nil {
			return path
		}
	}
	if runtime.GOOS == "windows" {
		for _, p := range []string{
			`C:\Program Files\Google\Chrome\Application\chrome.exe`,
			`C:\Program Files (x86)\Google\Chrome\Application\chrome.exe`,
			`C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe`,
			`C:\Program Files\Microsoft\Edge\Application\msedge.exe`,
		} {
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}
	}
	return ""
}
