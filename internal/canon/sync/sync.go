package sync

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/lavantien/quotes-manager/internal/config"
)

type Mirror struct {
	Repo   config.Repo
	Dirs   []string
	Keeps  []*regexp.Regexp
	root   string
	branch string
}

func New(cfg *config.Config, kind string) (*Mirror, error) {
	m := &Mirror{root: cfg.Canon.MirrorsDir(), branch: "origin/" + cfg.Canon.Repos.Bilara.Branch}
	switch kind {
	case "bilara":
		m.Repo = cfg.Canon.Repos.Bilara
		m.Dirs = cfg.Canon.Bilara.SparseDirs
	case "editions":
		m.Repo = cfg.Canon.Repos.Editions
		m.Dirs = cfg.Canon.Editions.SparseDirs
	default:
		return nil, fmt.Errorf("unknown mirror kind %q", kind)
	}
	keeps, err := kindKeeps(cfg, kind)
	if err != nil {
		return nil, err
	}
	m.Keeps = keeps
	m.branch = "origin/" + m.Repo.Branch
	return m, nil
}

func kindKeeps(cfg *config.Config, kind string) ([]*regexp.Regexp, error) {
	pats := cfg.Canon.Bilara.KeepRegexes
	if kind == "editions" {
		pats = cfg.Canon.Editions.KeepRegexes
	}
	out := make([]*regexp.Regexp, 0, len(pats))
	for _, p := range pats {
		re, err := regexp.Compile(p)
		if err != nil {
			return nil, fmt.Errorf("bad keep regex %q: %w", p, err)
		}
		out = append(out, re)
	}
	return out, nil
}

func (m *Mirror) Dir() string { return filepath.Join(m.root, m.Repo.Dir) }

func (m *Mirror) Commit() (string, error) {
	return git(m.Dir(), "rev-parse", "HEAD")
}

func (m *Mirror) Update(ctx context.Context) error {
	if _, err := os.Stat(filepath.Join(m.Dir(), ".git")); err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		return m.clone()
	}
	if _, err := run(ctx, m.root, "git", "-C", m.Repo.Dir, "fetch", "origin", m.Repo.Branch); err != nil {
		return err
	}
	if _, err := run(ctx, m.root, "git", "-C", m.Repo.Dir, "reset", "--hard", m.branch); err != nil {
		return err
	}
	_, err := run(ctx, m.root, "git", "-C", m.Repo.Dir, "clean", "-fd")
	return err
}

func (m *Mirror) clone() error {
	if err := os.MkdirAll(m.root, 0o755); err != nil {
		return err
	}
	args := []string{"clone", "--filter=blob:none", "--no-checkout", "--branch", m.Repo.Branch, m.Repo.URL, m.Repo.Dir}
	if _, err := run(context.Background(), m.root, "git", args...); err != nil {
		return err
	}
	sparse := append([]string{"-C", m.Repo.Dir, "sparse-checkout", "set", "--cone"}, m.Dirs...)
	if _, err := run(context.Background(), m.root, "git", sparse...); err != nil {
		return err
	}
	_, err := run(context.Background(), m.root, "git", "-C", m.Repo.Dir, "checkout", m.Repo.Branch)
	return err
}

func (m *Mirror) Prune() (int, error) {
	removed := 0
	type dirEntry struct {
		path  string
		depth int
	}
	var dirs []dirEntry
	root := m.Dir()
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, rerr := filepath.Rel(root, path)
		if rerr != nil {
			return rerr
		}
		if rel == "." {
			return nil
		}
		slash := filepath.ToSlash(rel)
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			dirs = append(dirs, dirEntry{path: path, depth: strings.Count(slash, "/")})
			return nil
		}
		if m.keep(slash) {
			return nil
		}
		if err := os.Remove(path); err != nil {
			return err
		}
		removed++
		return nil
	})
	if err != nil {
		return removed, err
	}
	for _, d := range slices.Backward(dirs) {
		if d.depth == 0 {
			continue
		}
		_ = os.Remove(d.path)
	}
	return removed, nil
}

func (m *Mirror) keep(slashRel string) bool {
	for _, re := range m.Keeps {
		if re.MatchString(slashRel) {
			return true
		}
	}
	return false
}

func git(dir string, args ...string) (string, error) {
	return run(context.Background(), dir, "git", args...)
}

func run(ctx context.Context, dir string, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	var out strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, strings.TrimSpace(out.String()))
	}
	return strings.TrimSpace(out.String()), nil
}
