package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lavantien/quotes-manager/internal/config"
)

func TestRunExtractReportsResolvedPaths(t *testing.T) {
	root := t.TempDir()
	dumps := filepath.Join(root, "dumps")
	if err := os.MkdirAll(dumps, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dumps, "a.txt"), []byte("\"Quote A.\" - MN 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := config.Defaults()
	cfg.Quotes.DumpsDir = dumps
	cfg.Quotes.SeedFile = filepath.Join(root, "database", "seed.sql")
	cfg.Quotes.ExportFile = filepath.Join(root, "exports", "shortest-first.md")

	oldOut := stdout
	stdout = new(bytes.Buffer)
	err := runExtract(context.Background(), cfg, nil)
	out := stdout.(*bytes.Buffer).String()
	stdout = oldOut

	if err != nil {
		t.Fatalf("runExtract: %v", err)
	}
	for _, want := range []string{"extracted 1 unique quotes", cfg.Quotes.SeedFile, cfg.Quotes.ExportFile} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestRunExtractRejectsArgs(t *testing.T) {
	if err := runExtract(context.Background(), config.Defaults(), []string{"extra"}); err == nil {
		t.Error("extract must reject positional arguments")
	}
}

func TestLoadDumpsOrdersByName(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "b.txt"), []byte("B"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("A"), 0o644); err != nil {
		t.Fatal(err)
	}
	files, err := loadDumps(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 || files[0].Name != "a.txt" || files[1].Name != "b.txt" {
		t.Errorf("loadDumps = %+v", files)
	}
	if files[0].Content != "A" || files[1].Content != "B" {
		t.Errorf("contents = %q / %q", files[0].Content, files[1].Content)
	}
}

func TestLoadDumpsNoMatches(t *testing.T) {
	files, err := loadDumps(t.TempDir())
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if len(files) != 0 {
		t.Errorf("len = %d, want 0", len(files))
	}
}

func TestExtractGenerateWritesArtifacts(t *testing.T) {
	root := t.TempDir()
	dumps := filepath.Join(root, "dumps")
	if err := os.MkdirAll(dumps, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dumps, "a.txt"), []byte("\"Quote A.\" - MN 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dumps, "b.txt"), []byte("\"Quote B.\" - MN 2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	seed := filepath.Join(root, "database", "seed.sql")
	export := filepath.Join(root, "exports", "shortest-first.md")

	count, err := extractGenerate(dumps, seed, export)
	if err != nil {
		t.Fatalf("extractGenerate: %v", err)
	}
	if count != 2 {
		t.Errorf("count = %d, want 2", count)
	}
	if _, err := os.Stat(seed); err != nil {
		t.Errorf("seed.sql not written: %v", err)
	}
	if _, err := os.Stat(export); err != nil {
		t.Errorf("export not written: %v", err)
	}
	seedBytes, _ := os.ReadFile(seed)
	if !strings.Contains(string(seedBytes), "INSERT INTO quotes") {
		t.Errorf("seed missing INSERT:\n%s", seedBytes)
	}
}

func TestExtractGenerateMissingDumpsDir(t *testing.T) {
	root := t.TempDir()
	_, err := extractGenerate(filepath.Join(root, "nope"), filepath.Join(root, "s.sql"), filepath.Join(root, "e.md"))
	if err != nil {
		t.Errorf("extractGenerate on missing dumps dir: %v", err)
	}
}

func TestExtractGenerateMkdirFails(t *testing.T) {
	root := t.TempDir()
	filePath := filepath.Join(root, "afile")
	if err := os.WriteFile(filePath, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := extractGenerate(t.TempDir(), filepath.Join(filePath, "seed.sql"), filepath.Join(root, "e.md"))
	if err == nil {
		t.Error("extractGenerate should fail when the output dir cannot be created")
	}
}

func TestExtractGenerateWriteFails(t *testing.T) {
	root := t.TempDir()
	dirAsFile := filepath.Join(root, "seed.sql")
	if err := os.MkdirAll(dirAsFile, 0o755); err != nil {
		t.Fatal(err)
	}
	_, err := extractGenerate(t.TempDir(), dirAsFile, filepath.Join(root, "e.md"))
	if err == nil {
		t.Error("extractGenerate should fail to write seed.sql into a directory")
	}
}

func TestExtractGenerateExportWriteFails(t *testing.T) {
	root := t.TempDir()
	dumps := filepath.Join(root, "dumps")
	if err := os.MkdirAll(dumps, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dumps, "a.txt"), []byte("\"A.\" - MN 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	exportDir := filepath.Join(root, "exports", "shortest-first.md")
	if err := os.MkdirAll(exportDir, 0o755); err != nil {
		t.Fatal(err)
	}
	_, err := extractGenerate(dumps, filepath.Join(root, "database", "seed.sql"), exportDir)
	if err == nil {
		t.Error("extractGenerate should fail to write the export into a directory")
	}
}
