package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

const seedTotal = 109

const kbdEnter = "\r"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", err)
		os.Exit(1)
	}
	fmt.Println("serve-smoke: all checks passed")
}

func run() error {
	root, bin, err := locate()
	if err != nil {
		return err
	}
	if err := httpChecks(root, bin); err != nil {
		return fmt.Errorf("http: %w", err)
	}
	if err := chromedpChecks(root, bin, ""); err != nil {
		return fmt.Errorf("chromedp: %w", err)
	}
	if err := hermeticCanonChecks(root, bin); err != nil {
		return fmt.Errorf("hermetic canon: %w", err)
	}
	return nil
}

func locate() (root, bin string, err error) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		return "", "", errors.New("runtime.Caller failed")
	}
	root = filepath.Dir(filepath.Dir(filepath.Dir(thisFile)))
	exe := ""
	if runtime.GOOS == "windows" {
		exe = ".exe"
	}
	bin = filepath.Join(root, "dist", "qm"+exe)
	if _, err := os.Stat(bin); err != nil {
		return "", "", fmt.Errorf("binary %s: %w (run make build)", bin, err)
	}
	return root, bin, nil
}

type proc struct {
	cmd    *exec.Cmd
	stdout bytes.Buffer
	base   string
}

func start(root, bin string, env map[string]string) (*proc, error) {
	dir, err := os.MkdirTemp("", "qm-smoke-*")
	if err != nil {
		return nil, err
	}
	probe, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	addr := probe.Addr().String()
	if err := probe.Close(); err != nil {
		return nil, err
	}

	cmd := exec.Command(bin, "serve", "--addr", addr, "--db", filepath.Join(dir, "quotes.db"))
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "QM_SEED_FILE="+filepath.Join(root, "database", "seed.sql"))
	for k, v := range env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	p := &proc{cmd: cmd, base: "http://" + addr}
	cmd.Stdout = &p.stdout
	cmd.Stderr = &p.stdout
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	client := &http.Client{Timeout: 2 * time.Second}
	deadline := time.Now().Add(20 * time.Second)
	for {
		resp, err := client.Get(p.base + "/")
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return p, nil
			}
		}
		if time.Now().After(deadline) {
			_ = cmd.Process.Kill()
			return nil, fmt.Errorf("server never came up on %s: %v; output: %s", p.base, err, p.stdout.String())
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func (p *proc) stop() {
	_ = p.cmd.Process.Kill()
	_ = p.cmd.Wait()
}

func get(p *proc, path string) (int, http.Header, string, error) {
	resp, err := http.Get(p.base + path)
	if err != nil {
		return 0, nil, "", err
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	return resp.StatusCode, resp.Header, string(raw), err
}

func wantStatus(p *proc, path string, want int, markers ...string) error {
	status, _, body, err := get(p, path)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	if status != want {
		return fmt.Errorf("%s: status = %d, want %d; body: %s", path, status, want, clip(body))
	}
	for _, m := range markers {
		if !strings.Contains(body, m) {
			return fmt.Errorf("%s: missing marker %q; body: %s", path, m, clip(body))
		}
	}
	return nil
}

func clip(s string) string {
	if len(s) > 500 {
		return s[:500] + "..."
	}
	return s
}

func httpChecks(root, bin string) error {
	p, err := start(root, bin, nil)
	if err != nil {
		return err
	}
	defer p.stop()

	checks := []struct {
		path    string
		status  int
		markers []string
	}{
		{"/", http.StatusOK, []string{`<nav class="nav">`, `class="nav-brand" href="/">qm</a>`, "/texts/mn"}},
		{"/dashboard", http.StatusOK, []string{"dashboard"}},
		{"/texts/mn1", http.StatusServiceUnavailable, []string{"no published database yet"}},
		{"/static/canon.css", http.StatusOK, []string{"--bg"}},
		{"/static/htmx.min.js", http.StatusOK, []string{`this.version="4.0.0"`}},
		{"/static/nav.css", http.StatusOK, []string{".nav"}},
		{"/static/cytoscape.min.js", http.StatusOK, []string{"cytoscape"}},
		{"/static/canon.js", http.StatusOK, []string{"job"}},
	}
	for _, c := range checks {
		if err := wantStatus(p, c.path, c.status, c.markers...); err != nil {
			return err
		}
	}

	status, hdr, body, err := get(p, "/api/v1/quotes")
	if err != nil {
		return err
	}
	if status != http.StatusOK {
		return fmt.Errorf("/api/v1/quotes: status = %d", status)
	}
	if ct := hdr.Get("Content-Type"); ct != "application/json" {
		return fmt.Errorf("/api/v1/quotes: content-type = %q", ct)
	}
	var list struct {
		Rows  []map[string]any `json:"rows"`
		Total int              `json:"total"`
	}
	if err := json.Unmarshal([]byte(body), &list); err != nil {
		return fmt.Errorf("/api/v1/quotes: %v (%s)", err, clip(body))
	}
	if list.Total != seedTotal {
		return fmt.Errorf("/api/v1/quotes: total = %d, want %d (seed corpus)", list.Total, seedTotal)
	}
	if len(list.Rows) != 50 {
		return fmt.Errorf("/api/v1/quotes: rows = %d, want 50 (DefaultLimit)", len(list.Rows))
	}

	status, _, body, err = get(p, "/api/v1/quotes/1")
	if err != nil {
		return err
	}
	if status != http.StatusOK || !strings.Contains(body, `"sutta_id"`) {
		return fmt.Errorf("/api/v1/quotes/1: status = %d, body %s", status, clip(body))
	}

	status, hdr, _, err = get(p, "/static/canon.css")
	if err != nil {
		return err
	}
	if cc := hdr.Get("Cache-Control"); status == http.StatusOK && cc != "public, max-age=86400" {
		return fmt.Errorf("/static/canon.css: cache-control = %q", cc)
	}
	fmt.Println("http checks passed: /, /dashboard, /texts/mn1 not-ready, 5 static assets, /api/v1/quotes seed corpus")
	return nil
}

func hermeticCanonChecks(root, bin string) error {
	src := filepath.Join(root, "data", "e2e-live", "db")
	entries, err := os.ReadDir(src)
	if err != nil {
		return fmt.Errorf("no local canon db at %s, skipping hermetic /texts/mn1 check: %w", src, err)
	}
	tmp, err := os.MkdirTemp("", "qm-canon-*")
	if err != nil {
		return err
	}
	dbDir := filepath.Join(tmp, "db")
	if err := os.MkdirAll(dbDir, 0o755); err != nil {
		return err
	}
	var dbFile string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".db") {
			dbFile = e.Name()
			raw, err := os.ReadFile(filepath.Join(src, e.Name()))
			if err != nil {
				return err
			}
			if err := os.WriteFile(filepath.Join(dbDir, e.Name()), raw, 0o644); err != nil {
				return err
			}
		}
	}
	if dbFile == "" {
		return errors.New("no canon db file found in " + src)
	}
	pointer := fmt.Sprintf(`{"current":%q,"path":%q,"updated":%q}`,
		strings.TrimSuffix(dbFile, ".db"), filepath.Join(dbDir, dbFile), time.Now().UTC().Format(time.RFC3339))
	if err := os.WriteFile(filepath.Join(dbDir, "pointer.json"), []byte(pointer), 0o644); err != nil {
		return err
	}

	p, err := start(root, bin, map[string]string{"QM_CANON_DB_DIR": dbDir})
	if err != nil {
		return err
	}
	defer p.stop()

	if err := wantStatus(p, "/texts/mn1", http.StatusOK, "/words?form=", "mn1"); err != nil {
		return err
	}
	fmt.Println("hermetic canon check passed: /texts/mn1 renders segments with word links")
	return nil
}

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

func chromedpChecks(root, bin, canonDBDir string) error {
	env := map[string]string{}
	if canonDBDir != "" {
		env["QM_CANON_DB_DIR"] = canonDBDir
	}
	p, err := start(root, bin, env)
	if err != nil {
		return err
	}
	defer p.stop()

	allocOpts := append(chromedp.DefaultExecAllocatorOptions[:], chromedp.Flag("hide-scrollbars", "true"))
	if bin := browserPath(); bin != "" {
		allocOpts = append(allocOpts, chromedp.ExecPath(bin))
	}
	ctx, cancel := chromedp.NewExecAllocator(context.Background(), allocOpts...)
	defer cancel()
	ctx, cancel = chromedp.NewContext(ctx)
	defer cancel()
	ctx, cancel = context.WithTimeout(ctx, 120*time.Second)
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

	run("navigate", chromedp.EmulateViewport(1440, 900), chromedp.Navigate(p.base+"/"), chromedp.WaitVisible("#quote-list"))
	htmxOn := poll("htmx loaded", `typeof htmx !== 'undefined' && htmx.version === '4.0.0'`)

	run("open form", chromedp.Click(`button[hx-get="/quotes/new"]`), chromedp.WaitVisible("#form-slot .quote-form"))
	run("fill form",
		chromedp.SetValue(`textarea[name="content"]`, `"Smoke passage for the merged shell."`),
		chromedp.SetValue(`input[name="attribution"]`, "the Buddha"),
		chromedp.SetValue(`input[name="text_id"]`, "MN 22"),
		chromedp.SetValue(`input[name="new_category"]`, "smoke-cat"),
	)
	run("submit form", chromedp.Click(`.quote-form button[type="submit"]`))
	created := poll("quote rendered", `document.getElementById('quote-110') !== null && document.body.innerText.includes('Smoke passage for the merged shell.')`)

	run("open rename", chromedp.Click(`button[data-action="rename-category"]`), chromedp.WaitVisible("input.rail__rename"))
	run("type rename", chromedp.Clear(`input.rail__rename`), chromedp.SendKeys(`input.rail__rename`, "smoke-renamed"))
	run("commit rename", chromedp.KeyEvent(kbdEnter))
	renamed := poll("rename rendered", `document.body.innerText.includes('smoke-renamed')`)

	if flowErr == nil {
		_, flowErr = chromedp.Run(ctx, chromedp.Evaluate[string]("window.smokeMarker = 1; 'ok'"))
	}
	run("open category", chromedp.Click(`a.rail__link[href="/categories/1"]`))
	sameDoc := poll("stayed on document", `typeof window.smokeMarker !== 'undefined' && document.getElementById('root-zone') && document.getElementById('root-zone').dataset.cat === '1'`)
	catOn := sameDoc

	run("open home", chromedp.Click(`a.rail__link[href="/"]`))
	homeOn := poll("home pane", `document.getElementById('root-zone') && document.getElementById('root-zone').dataset.cat === '0'`)

	run("type search", chromedp.SendKeys(`input.zone__search`, "consciousness"))
	searched := poll("search filtered",
		`(function(){var n = document.querySelectorAll('#quote-list [id^="quote-"]').length; return n > 0 && n < 110;})()`)

	var consoleErrs []string
	for m, merr := range messages {
		if merr != nil {
			break
		}
		if m.Type == chromedp.ConsoleError || m.IsException() {
			consoleErrs = append(consoleErrs, m.String())
		}
	}

	switch {
	case flowErr != nil:
		return fmt.Errorf("flow: %w; console errors: %v", flowErr, consoleErrs)
	case !htmxOn || !created || !renamed || !catOn || !homeOn || !searched:
		return fmt.Errorf("flow never settled: htmx=%v created=%v renamed=%v catOn=%v homeOn=%v searched=%v; console errors: %v",
			htmxOn, created, renamed, catOn, homeOn, searched, consoleErrs)
	case len(consoleErrs) > 0:
		return fmt.Errorf("console errors under htmx 4.0.0: %v", consoleErrs)
	}
	fmt.Println("chromedp flows passed: htmx 4.0.0 active, create quote #110, rename category, toggle pane, live search; zero console errors")
	return nil
}
