package e2e

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/lavantien/quotes-manager/internal/canon/source/dpd"
)

func writeFiles(path string, files map[string]string) error {
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

func gitIn(dir string, args ...string) error {
	cmd := exec.Command("git", append([]string{
		"-c", "user.email=e2e@quotes-manager", "-c", "user.name=e2e",
	}, args...)...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git %s in %s: %w: %s", strings.Join(args, " "), dir, err, out)
	}
	return nil
}

func fileURL(path string) string {
	return "file:///" + strings.TrimPrefix(filepath.ToSlash(path), "/")
}

func buildBilara(path string) error {
	files := map[string]string{
		"root/pli/ms/sutta/mn/mn1_root-pli-ms.json": `{
	"mn1:0.1": "Middle Discourses 1",
	"mn1:1.1": "Evaṁ me sutaṁ.",
	"mn1:2.1": "Deep indeed."
}`,
		"root/pli/ms/sutta/mn/mn2_root-pli-ms.json": `{
	"mn2:1.1": "All the defilements."
}`,
		"translation/en/sujato/sutta/mn/mn1_translation-en-sujato.json": `{
	"mn1:0.1": "Middle Discourses 1",
	"mn1:1.1": "So I have heard.",
	"mn1:2.1": "Deep indeed."
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
	if err := writeFiles(path, files); err != nil {
		return err
	}
	if err := gitIn(path, "init", "-b", "main"); err != nil {
		return err
	}
	if err := gitIn(path, "add", "-A"); err != nil {
		return err
	}
	return gitIn(path, "commit", "-m", "chore: synthetic bilara corpus")
}

func buildEditions(path string) error {
	zipPath := filepath.Join(path, "en/sujato/mn/paperback/Test-sujato-2026-01-02-tex.zip")
	if err := os.MkdirAll(filepath.Dir(zipPath), 0o755); err != nil {
		return err
	}
	f, err := os.Create(zipPath)
	if err != nil {
		return err
	}
	zw := zip.NewWriter(f)
	w, err := zw.Create("vol-1.tex")
	if err != nil {
		return err
	}
	if _, err := w.Write([]byte(`\chapter*{Preface}
Front matter.
\section*{{\suttatitleacronym MN 1}{\suttatitletranslation Title}{\suttatitleroot Root}}
\addcontentsline{toc}{section}{MN 1}
\scevam{Middle Discourses\marginnote{0.1} 1 So\marginnote{1.1} I have heard.}
Deep indeed.
\section*{{\suttatitleacronym MN 2}{\suttatitletranslation T2}{\suttatitleroot R2}}
All the defilements.
`)); err != nil {
		return err
	}
	if err := zw.Close(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
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
	if err := writeFiles(path, files); err != nil {
		return err
	}
	if err := gitIn(path, "init", "-b", "main"); err != nil {
		return err
	}
	if err := gitIn(path, "add", "-A"); err != nil {
		return err
	}
	return gitIn(path, "commit", "-m", "chore: synthetic editions corpus")
}

func writeMergedConfig(tmp, quotesDB string) (string, error) {
	if err := buildBilara(filepath.Join(tmp, "bilara-src")); err != nil {
		return "", err
	}
	if err := buildEditions(filepath.Join(tmp, "editions-src")); err != nil {
		return "", err
	}
	_, dpdTar, err := dpd.WriteFixture(filepath.Join(tmp, "dpd-fixture"))
	if err != nil {
		return "", err
	}
	work := filepath.Join(tmp, "work")
	cfg := map[string]any{
		"quotes": map[string]any{
			"db_path":      filepath.ToSlash(quotesDB),
			"dumps_dir":    filepath.ToSlash(filepath.Join(tmp, "dumps")),
			"seed_file":    "database/seed.sql",
			"export_file":  filepath.ToSlash(filepath.Join(tmp, "exports", "e2e.md")),
			"pictures_dir": filepath.ToSlash(filepath.Join(tmp, "pictures")),
		},
		"server": map[string]any{"addr": "127.0.0.1:0", "sql_console": true},
		"canon": map[string]any{
			"repos": map[string]any{
				"bilara": map[string]any{
					"url": fileURL(filepath.Join(tmp, "bilara-src")), "branch": "main", "dir": "bilara-data",
				},
				"editions": map[string]any{
					"url": fileURL(filepath.Join(tmp, "editions-src")), "branch": "main", "dir": "editions",
				},
			},
			"paths": map[string]any{
				"data_dir":      filepath.ToSlash(work),
				"db_dir":        filepath.ToSlash(filepath.Join(work, "db")),
				"dist_dir":      filepath.ToSlash(filepath.Join(work, "dist")),
				"manifests_dir": filepath.ToSlash(filepath.Join(work, "docs", "manifests")),
				"reports_dir":   filepath.ToSlash(filepath.Join(work, "docs", "reports")),
			},
			"dpd": map[string]any{"url": fileURL(dpdTar)},
			"e2e": map[string]any{"muids": []string{}},
		},
	}
	raw, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return "", err
	}
	p := filepath.Join(tmp, "config.json")
	if err := os.WriteFile(p, raw, 0o644); err != nil {
		return "", err
	}
	return p, nil
}
