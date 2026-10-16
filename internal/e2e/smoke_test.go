package e2e

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

const smokeEnterKey = "\r"

const smokeTimeout = 120 * time.Second

var smokeIDRe = regexp.MustCompile(`quote-[0-9]+`)

func smokeBrowserPath(t *testing.T) string {
	t.Helper()
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

func TestChromedpUISmoke(t *testing.T) {
	bin := smokeBrowserPath(t)
	if bin == "" {
		t.Skip("no chrome/chromium/edge binary found; set QUOTES_BROWSER to force one")
	}
	app := mergedAppFor(t, true)
	base := app.server().URL

	allocOpts := append(chromedp.DefaultExecAllocatorOptions[:], chromedp.Flag("hide-scrollbars", "true"))
	allocOpts = append(allocOpts, chromedp.ExecPath(bin))
	ctx, cancel := chromedp.NewExecAllocator(context.Background(), allocOpts...)
	defer cancel()
	ctx, cancel = chromedp.NewContext(ctx)
	defer cancel()
	ctx, cancel = context.WithTimeout(ctx, smokeTimeout)
	defer cancel()

	messages := chromedp.Console(ctx)
	var flowErr error
	run := func(name string, acts ...chromedp.Action[chromedp.Void]) {
		if flowErr != nil {
			return
		}
		if err := chromedp.Do(ctx, acts...); err != nil {
			flowErr = fmt.Errorf("%s: %w", name, err)
		}
	}
	poll := func(name, expr string) bool {
		if flowErr != nil {
			return false
		}
		ok, err := chromedp.Run(ctx, chromedp.Poll[bool](expr, chromedp.WithPollingTimeout(10*time.Second)))
		if err != nil {
			flowErr = fmt.Errorf("poll %s: %w", name, err)
			return false
		}
		return ok
	}

	run("navigate", chromedp.EmulateViewport(1440, 900), chromedp.Navigate(base+"/"), chromedp.WaitVisible("#quote-list"))
	htmxOn := poll("htmx loaded", `typeof htmx !== 'undefined' && htmx.version === '4.0.0'`)

	run("open form", chromedp.Click(`button[hx-get="/quotes/new"]`), chromedp.WaitVisible("#form-slot .quote-form"))
	run("fill form",
		chromedp.SetValue(`textarea[name="content"]`, `"Smoke passage for the merged shell."`),
		chromedp.SetValue(`input[name="attribution"]`, "the Buddha"),
		chromedp.SetValue(`input[name="text_id"]`, "MN 22"),
		chromedp.SetValue(`input[name="new_category"]`, "smoke-cat"),
	)
	run("submit form", chromedp.Click(`.quote-form button[type="submit"]`))
	created := poll("quote rendered",
		`document.getElementById('quote-110') !== null && document.body.innerText.includes('Smoke passage for the merged shell.')`)

	run("open rename", chromedp.Click(`button[data-action="rename-category"]`), chromedp.WaitVisible("input.rail__rename"))
	run("type rename", chromedp.Clear(`input.rail__rename`), chromedp.SendKeys(`input.rail__rename`, "smoke-renamed"))
	run("commit rename", chromedp.KeyEvent(smokeEnterKey))
	renamed := poll("rename rendered", `document.body.innerText.includes('smoke-renamed')`)

	if flowErr == nil {
		_, flowErr = chromedp.Run(ctx, chromedp.Evaluate[string]("window.smokeMarker = 1; 'ok'"))
	}
	run("open category", chromedp.Click(`a.rail__link[href="/categories/1"]`))
	sameDoc := poll("stayed on document",
		`typeof window.smokeMarker !== 'undefined' && document.getElementById('root-zone') && document.getElementById('root-zone').dataset.cat === '1'`)

	run("open home", chromedp.Click(`a.rail__link[href="/"]`))
	homeOn := poll("home pane", `document.getElementById('root-zone') && document.getElementById('root-zone').dataset.cat === '0'`)

	run("type search", chromedp.SendKeys(`input.zone__search`, "consciousness"))
	searched := poll("search filtered",
		fmt.Sprintf(`(function(){var n = document.querySelectorAll('#quote-list [id^="quote-"]').length; return n > 0 && n < %d;})()`, seedCorpusTotal+1))

	cancel()
	var consoleErrs []string
	for m, merr := range messages {
		if merr != nil {
			break
		}
		if m.Type == chromedp.ConsoleError || m.IsException() {
			consoleErrs = append(consoleErrs, m.String())
		}
		if len(consoleErrs) > 20 {
			break
		}
	}

	switch {
	case flowErr != nil:
		t.Fatalf("flow: %v; console errors: %v", flowErr, consoleErrs)
	case !htmxOn || !created || !renamed || !sameDoc || !homeOn || !searched:
		t.Fatalf("flow never settled: htmx=%v created=%v renamed=%v paneToggle=%v home=%v searched=%v; console errors: %v",
			htmxOn, created, renamed, sameDoc, homeOn, searched, consoleErrs)
	case len(consoleErrs) > 0:
		t.Fatalf("console errors: %v", consoleErrs)
	}
	if !smokeIDRe.MatchString(app.get(t, "/").body) {
		t.Fatal("index lost its quote blocks after the smoke flow")
	}
}
