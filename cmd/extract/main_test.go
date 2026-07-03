package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReport(t *testing.T) {
	var buf bytes.Buffer
	report(&buf, 7)
	got := buf.String()
	for _, want := range []string{"extracted 7 unique quotes", "database/seed.sql", "exports/shortest-first.md"} {
		if !strings.Contains(got, want) {
			t.Errorf("report missing %q:\n%s", want, got)
		}
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
	// A directory with no .txt files yields an empty slice, not an error.
	files, err := loadDumps(t.TempDir())
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if len(files) != 0 {
		t.Errorf("len = %d, want 0", len(files))
	}
}

func TestGenerateWritesArtifacts(t *testing.T) {
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

	count, err := generate(dumps, seed, export)
	if err != nil {
		t.Fatalf("generate: %v", err)
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

func TestGenerateMissingDumpsDir(t *testing.T) {
	root := t.TempDir()
	// loadDumps on a non-existent dir returns no error and no files, so generate
	// succeeds and writes empty artifacts.
	_, err := generate(filepath.Join(root, "nope"), filepath.Join(root, "s.sql"), filepath.Join(root, "e.md"))
	if err != nil {
		t.Errorf("generate on missing dumps dir: %v", err)
	}
}

func TestGenerateMkdirFails(t *testing.T) {
	root := t.TempDir()
	// Make the seed path's parent a regular file, so MkdirAll cannot create it.
	filePath := filepath.Join(root, "afile")
	if err := os.WriteFile(filePath, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := generate(t.TempDir(), filepath.Join(filePath, "seed.sql"), filepath.Join(root, "e.md"))
	if err == nil {
		t.Error("generate should fail when the output dir cannot be created")
	}
}

func TestGenerateWriteFails(t *testing.T) {
	root := t.TempDir()
	// The seed path is itself a directory, so WriteFile cannot create it.
	dirAsFile := filepath.Join(root, "seed.sql")
	if err := os.MkdirAll(dirAsFile, 0o755); err != nil {
		t.Fatal(err)
	}
	_, err := generate(t.TempDir(), dirAsFile, filepath.Join(root, "e.md"))
	if err == nil {
		t.Error("generate should fail to write seed.sql into a directory")
	}
}

func TestGenerateExportWriteFails(t *testing.T) {
	root := t.TempDir()
	dumps := filepath.Join(root, "dumps")
	if err := os.MkdirAll(dumps, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dumps, "a.txt"), []byte("\"A.\" - MN 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// The export path is itself a directory, so WriteFile fails after seed.sql.
	exportDir := filepath.Join(root, "exports", "shortest-first.md")
	if err := os.MkdirAll(exportDir, 0o755); err != nil {
		t.Fatal(err)
	}
	_, err := generate(dumps, filepath.Join(root, "database", "seed.sql"), exportDir)
	if err == nil {
		t.Error("generate should fail to write the export into a directory")
	}
}
