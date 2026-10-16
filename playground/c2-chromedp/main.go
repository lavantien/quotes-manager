package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/chromedp/chromedp"
)

const (
	viewportW = 1440
	viewportH = 900
)

func browserPath() string {
	for _, name := range []string{"chrome", "chromium", "chromium-browser", "google-chrome"} {
		if path, err := execLookPath(name); err == nil {
			return path
		}
	}
	return ""
}

func main() {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, "<html><body><ul id='quote-list'><li>one</li></ul></body></html>")
	})
	srv := &http.Server{Handler: mux}
	go func() { _ = srv.Serve(ln) }()
	defer func() { _ = srv.Shutdown(context.Background()) }()
	url := "http://" + ln.Addr().String() + "/"

	allocOpts := append(chromedp.DefaultExecAllocatorOptions[:], chromedp.Flag("hide-scrollbars", "true"))
	ctx, cancel := chromedp.NewExecAllocator(context.Background(), allocOpts...)
	defer cancel()
	ctx, cancel = chromedp.NewContext(ctx)
	defer cancel()
	ctx, cancel = context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	if err := chromedp.Do(ctx,
		chromedp.EmulateViewport(viewportW, viewportH),
		chromedp.Navigate(url),
		chromedp.WaitVisible(chromedp.ID("quote-list")),
		chromedp.Sleep(300*time.Millisecond),
	); err != nil {
		fmt.Println("setup Do failed:", err)
		os.Exit(1)
	}
	buf, err := chromedp.Run(ctx, chromedp.CaptureScreenshot())
	if err != nil {
		fmt.Println("capture Run failed:", err)
		os.Exit(1)
	}
	if len(buf) < 8 || string(buf[:8]) != "\x89PNG\r\n\x1a\n" {
		fmt.Println("capture is not a PNG")
		os.Exit(1)
	}
	fmt.Println("ok: captured PNG", len(buf), "bytes via Do setup + Run capture")
}
