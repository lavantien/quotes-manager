package sync

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lavantien/quotes-manager/internal/config"
)

func fileURL(path string) string {
	return "file:///" + filepath.ToSlash(filepath.Clean(path))
}

func initRemote(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	mustGit(t, dir, "init", "-b", "published")
	mustGit(t, dir, "config", "user.email", "test@example.com")
	mustGit(t, dir, "config", "user.name", "test")
	write(t, dir, filepath.Join("root", "pli", "ms", "sutta", "mn", "mn1_root-pli-ms.json"), `{"mn1:0.1":"a "}`)
	write(t, dir, "_edition.json", `{}`)
	write(t, dir, "stray.txt", "prune me")
	write(t, dir, ".gitignore", "ignored\n")
	mustGit(t, dir, "add", ".")
	mustGit(t, dir, "commit", "-m", "test: init fixture")
	return dir
}

func initEditionsRemote(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	mustGit(t, dir, "init", "-b", "main")
	mustGit(t, dir, "config", "user.email", "test@example.com")
	mustGit(t, dir, "config", "user.name", "test")
	write(t, dir, filepath.Join("en", "sujato", "an", "html", "an1.0.html"), "<p>a</p>")
	write(t, dir, filepath.Join("en", "sujato", "an", "tex", "an1.0.tex"), "tex body")
	write(t, dir, "last_run_date", "2026-10-17")
	write(t, dir, "README.md", "readme")
	write(t, dir, filepath.Join("en", "sujato", "notes.txt"), "prune me")
	mustGit(t, dir, "add", ".")
	mustGit(t, dir, "commit", "-m", "test: init editions fixture")
	return dir
}

func mustGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	out, err := git(dir, args...)
	if err != nil {
		t.Fatalf("git %v: %v (%s)", args, err, out)
	}
}

func write(t *testing.T, dir, rel, body string) {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func testCfg(t *testing.T, remote string) *config.Config {
	t.Helper()
	c := config.Defaults()
	c.Canon.Repos.Bilara.URL = fileURL(remote)
	c.Canon.Bilara.SparseDirs = []string{"root/pli/ms/sutta"}
	return c
}

func TestCloneSparseAndPrune(t *testing.T) {
	remote := initRemote(t)
	cfg := testCfg(t, remote)
	cfg.Canon.Paths.DataDir = t.TempDir()

	m, err := New(cfg, "bilara")
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Update(t.Context()); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(m.Dir(), "root", "pli", "ms", "sutta", "mn", "mn1_root-pli-ms.json")); err != nil {
		t.Fatalf("sutta file missing after clone: %v", err)
	}
	if _, err := os.Stat(filepath.Join(m.Dir(), "_edition.json")); err != nil {
		t.Fatalf("root metadata missing after clone: %v", err)
	}

	removed, err := m.Prune()
	if err != nil {
		t.Fatal(err)
	}
	if removed == 0 {
		t.Fatal("stray.txt must be pruned")
	}
	if _, err := os.Stat(filepath.Join(m.Dir(), "stray.txt")); !os.IsNotExist(err) {
		t.Fatal("stray.txt survived prune")
	}
	if _, err := os.Stat(filepath.Join(m.Dir(), "_edition.json")); err != nil {
		t.Fatalf("keep-listed metadata must survive prune: %v", err)
	}
	if _, err := os.Stat(filepath.Join(m.Dir(), ".git")); err != nil {
		t.Fatalf(".git must never be touched: %v", err)
	}

	if _, err := m.Commit(); err != nil {
		t.Fatalf("commit resolution failed: %v", err)
	}
}

func TestUpdateResetsToRemote(t *testing.T) {
	remote := initRemote(t)
	cfg := testCfg(t, remote)
	cfg.Canon.Paths.DataDir = t.TempDir()

	m, err := New(cfg, "bilara")
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Update(t.Context()); err != nil {
		t.Fatal(err)
	}

	write(t, m.Dir(), "root/pli/ms/sutta/mn/tampered.json", "{}")
	write(t, remote, filepath.Join("root", "pli", "ms", "sutta", "mn", "mn2_root-pli-ms.json"), `{"mn2:0.1":"b "}`)
	mustGit(t, remote, "add", ".")
	mustGit(t, remote, "commit", "-m", "test: add mn2")

	if err := m.Update(t.Context()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(m.Dir(), "root", "pli", "ms", "sutta", "mn", "mn2_root-pli-ms.json")); err != nil {
		t.Fatalf("new upstream file missing after update: %v", err)
	}
	if _, err := os.Stat(filepath.Join(m.Dir(), "root", "pli", "ms", "sutta", "mn", "tampered.json")); !os.IsNotExist(err) {
		t.Fatal("local-only file survived hard reset")
	}
}

func TestEditionsPruneKeepsHtmlOnly(t *testing.T) {
	remote := initEditionsRemote(t)
	cfg := config.Defaults()
	cfg.Canon.Repos.Editions.URL = fileURL(remote)
	cfg.Canon.Editions.SparseDirs = []string{"en/sujato"}
	cfg.Canon.Paths.DataDir = t.TempDir()

	m, err := New(cfg, "editions")
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Update(t.Context()); err != nil {
		t.Fatal(err)
	}

	removed, err := m.Prune()
	if err != nil {
		t.Fatal(err)
	}
	if removed == 0 {
		t.Fatal("off-keep-list editions files must be pruned")
	}
	if _, err := os.Stat(filepath.Join(m.Dir(), "en", "sujato", "an", "html", "an1.0.html")); err != nil {
		t.Fatalf("html edition must survive prune: %v", err)
	}
	if _, err := os.Stat(filepath.Join(m.Dir(), "en", "sujato", "an", "tex", "an1.0.tex")); !os.IsNotExist(err) {
		t.Fatal("tex file must be pruned under the html-only keep-list")
	}
	if _, err := os.Stat(filepath.Join(m.Dir(), "en", "sujato", "notes.txt")); !os.IsNotExist(err) {
		t.Fatal("stray editions file must be pruned")
	}
	if _, err := os.Stat(filepath.Join(m.Dir(), "last_run_date")); err != nil {
		t.Fatalf("last_run_date must survive prune: %v", err)
	}
	if _, err := os.Stat(filepath.Join(m.Dir(), "README.md")); err != nil {
		t.Fatalf("README must survive prune: %v", err)
	}
}

func TestUnknownKindRejected(t *testing.T) {
	if _, err := New(config.Defaults(), "nope"); err == nil {
		t.Fatal("unknown mirror kind must be rejected")
	}
}
