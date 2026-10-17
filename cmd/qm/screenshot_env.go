package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	canonserver "github.com/lavantien/quotes-manager/internal/canon/server"
	"github.com/lavantien/quotes-manager/internal/canon/source/dpd"
	"github.com/lavantien/quotes-manager/internal/cli"
	"github.com/lavantien/quotes-manager/internal/config"
	"github.com/lavantien/quotes-manager/internal/seed"
	"github.com/lavantien/quotes-manager/internal/server"
	"github.com/lavantien/quotes-manager/internal/store"
)

func init() {
	cli.SetScreenshotEnv(buildScreenshotEnv)
}

func buildScreenshotEnv(_ *config.Config) (string, func() error, error) {
	tmp, err := os.MkdirTemp("", "qm-screenshot-*")
	if err != nil {
		return "", nil, fmt.Errorf("temp dir: %w", err)
	}
	cleanup := func() error { return os.RemoveAll(tmp) }

	canonCfg, err := hermeticCanonConfig(tmp)
	if err != nil {
		_ = cleanup()
		return "", nil, err
	}
	for _, st := range []struct {
		name string
		args []string
	}{
		{"sync", nil},
		{"parse", nil},
		{"load", nil},
	} {
		if err := cli.RunStage(context.Background(), canonCfg, st.name, st.args, io.Discard); err != nil {
			_ = cleanup()
			return "", nil, fmt.Errorf("hermetic canon %s: %w", st.name, err)
		}
	}

	st, err := store.Open(filepath.Join(tmp, "quotes.db"))
	if err != nil {
		_ = cleanup()
		return "", nil, fmt.Errorf("open quotes store: %w", err)
	}
	if err := seed.EnsureSeeded(st.DB()); err != nil {
		_ = errors.Join(st.Close(), cleanup())
		return "", nil, fmt.Errorf("seed quotes: %w", err)
	}
	canonSrv, err := canonserver.New(canonCfg)
	if err != nil {
		_ = errors.Join(st.Close(), cleanup())
		return "", nil, fmt.Errorf("canon server: %w", err)
	}
	root := http.NewServeMux()
	server.New(st, server.WithLimits(canonCfg.Server.DefaultLimit, canonCfg.Server.MaxLimit)).RegisterRoutes(root)
	canonSrv.RegisterRoutes(root)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		_ = errors.Join(canonSrv.Close(), st.Close(), cleanup())
		return "", nil, fmt.Errorf("listen: %w", err)
	}
	srv := &http.Server{Handler: canonSrv.Chain(root)}
	go func() { _ = srv.Serve(ln) }()
	close := func() error {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return errors.Join(srv.Shutdown(shutdownCtx), canonSrv.Close(), st.Close(), cleanup())
	}
	return "http://" + ln.Addr().String(), close, nil
}

func hermeticCanonConfig(tmp string) (*config.Config, error) {
	bilaraSrc := filepath.Join(tmp, "bilara-src")
	editionsSrc := filepath.Join(tmp, "editions-src")
	if err := writeJourneyBilara(bilaraSrc); err != nil {
		return nil, err
	}
	if err := writeJourneyEditions(editionsSrc); err != nil {
		return nil, err
	}
	_, dpdTar, err := dpd.WriteFixture(filepath.Join(tmp, "dpd-fixture"))
	if err != nil {
		return nil, fmt.Errorf("dpd fixture: %w", err)
	}
	work := filepath.Join(tmp, "work")
	cfg := config.Defaults()
	cfg.Canon.Repos.Bilara = config.Repo{URL: fileURL(bilaraSrc), Branch: "main", Dir: "bilara-data"}
	cfg.Canon.Repos.Editions = config.Repo{URL: fileURL(editionsSrc), Branch: "main", Dir: "editions"}
	cfg.Canon.Paths.DataDir = work
	cfg.Canon.Paths.DBDir = filepath.Join(work, "db")
	cfg.Canon.Paths.DistDir = filepath.Join(work, "dist")
	cfg.Canon.Paths.ManifestsDir = filepath.Join(work, "docs", "manifests")
	cfg.Canon.Paths.ReportsDir = filepath.Join(work, "docs", "reports")
	cfg.Canon.Dpd.URL = fileURL(dpdTar)
	cfg.Canon.E2E.Muids = []string{}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("hermetic canon config: %w", err)
	}
	return cfg, nil
}

func writeJourneyBilara(path string) error {
	files := map[string]string{
		"root/pli/ms/sutta/mn/mn1_root-pli-ms.json": `{
	"mn1:0.1": "Middle Discourses 1",
	"mn1:1.1": "Evaṁ me sutaṁ.",
	"mn1:2.1": "Deep indeed."
}`,
		"root/pli/ms/sutta/mn/mn2_root-pli-ms.json": `{
	"mn2:0.1": "Middle Discourses 2",
	"mn2:1.1": "All the defilements."
}`,
		"root/pli/ms/vinaya/pli-tv-bu-pm_root-pli-ms.json": `{
	"pli-tv-bu-pm:0.1": "The monks' rules."
}`,
		"translation/en/sujato/sutta/mn/mn1_translation-en-sujato.json": `{
	"mn1:0.1": "Middle Discourses 1",
	"mn1:1.1": "So I have heard.",
	"mn1:2.1": "Deep indeed."
}`,
		"translation/en/sujato/name/sutta/mn-name_translation-en-sujato.json": `{
	"mn-name:1.mn": "The Middle Discourses",
	"mn-name:2.mn1": "The Root of All Things",
	"mn-name:3.mn2": "All the Defilements"
}`,
		"html/pli/ms/sutta/mn/mn1_html.json": `{
	"mn1:0.1": "<p>",
	"mn1:1.1": "<p>"
}`,
		"reference/pli/ms/sutta/mn/mn1_reference.json": `{
	"mn1:0.1": "MN i 1",
	"mn1:1.1": "MN i 2",
	"mn1:2.1": "MN i 3"
}`,
	}
	if err := writeCorpusFiles(path, files); err != nil {
		return err
	}
	if err := gitCommitAll(path); err != nil {
		return err
	}
	return nil
}

func writeJourneyEditions(path string) error {
	files := map[string]string{
		"en/sujato/mn/html/Middle-Discourses-sujato-2026-01-02.html": `<!doctype html>
<html><body>
<article id="mn1">
<h3 class="sutta-title heading"><span class="sutta-heading acronym">MN 1</span></h3>
<p id="mn1:0.1">Middle Discourses 1</p>
<p id="mn1:1.1">So I have heard.<a epub:type="noteref" href="#note-1" id="noteref-1" role="doc-noteref">1</a></p>
<p id="mn1:2.1">Deep indeed.</p>
</article>
<article id="mn2">
<p id="mn2:1.1">All the defilements.</p>
</article>
</body></html>`,
		"last_run_date": "2026-01-02",
	}
	if err := writeCorpusFiles(path, files); err != nil {
		return err
	}
	return gitCommitAll(path)
}

func writeCorpusFiles(path string, files map[string]string) error {
	for rel, body := range files {
		p := filepath.Join(path, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func gitCommitAll(dir string) error {
	git := func(args ...string) error {
		cmd := exec.Command("git", append([]string{
			"-c", "user.email=screenshot@quotes-manager", "-c", "user.name=screenshot",
		}, args...)...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("git %s in %s: %w: %s", strings.Join(args, " "), dir, err, out)
		}
		return nil
	}
	if err := git("init", "-b", "main"); err != nil {
		return err
	}
	if err := git("add", "-A"); err != nil {
		return err
	}
	return git("commit", "-m", "chore: synthetic corpus")
}

func fileURL(path string) string {
	return "file:///" + strings.TrimPrefix(filepath.ToSlash(path), "/")
}
