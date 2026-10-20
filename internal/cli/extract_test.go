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

const tableFixture = "**I. Test Group.**\n\n1. Entry ([MN 26](https://suttacentral.net/mn26/en/sujato)).\n"

func writeTableDumps(t *testing.T, quoteBody, tableBody string) string {
	t.Helper()
	dumps := filepath.Join(t.TempDir(), "dumps")
	if err := os.MkdirAll(dumps, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dumps, "a.txt"), []byte(quoteBody), 0o644); err != nil {
		t.Fatal(err)
	}
	if tableBody != "" {
		if err := os.WriteFile(filepath.Join(dumps, "a.md"), []byte(tableBody), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dumps
}

func TestExtractGenerateSkipsZeroGroupTables(t *testing.T) {
	root := t.TempDir()
	dumps := writeTableDumps(t, "\"Quote A.\" - MN 1\n", "# stray notes\nno sutta tables here\n")
	seed := filepath.Join(root, "database", "seed.sql")
	export := filepath.Join(root, "exports", "shortest-first.md")

	if _, err := extractGenerate(dumps, seed, export, "TestCollection"); err != nil {
		t.Fatalf("extractGenerate: %v", err)
	}
	seedBytes, err := os.ReadFile(seed)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(seedBytes, []byte("TestCollection")) {
		t.Error("a zero-group markdown must not seed a collection shell")
	}
	if bytes.Contains(seedBytes, []byte("suttas")) {
		t.Error("a zero-group markdown must not emit the sutta table deletes")
	}
}

func TestExtractGenerateAppendsSuttaSection(t *testing.T) {
	root := t.TempDir()
	dumps := writeTableDumps(t, "\"Quote A.\" - MN 1\n", tableFixture)
	seed := filepath.Join(root, "database", "seed.sql")
	export := filepath.Join(root, "exports", "shortest-first.md")

	if _, err := extractGenerate(dumps, seed, export, "TestCollection"); err != nil {
		t.Fatalf("extractGenerate: %v", err)
	}
	seedBytes, err := os.ReadFile(seed)
	if err != nil {
		t.Fatal(err)
	}
	got := string(seedBytes)
	for _, want := range []string{
		"INSERT INTO quotes",
		"INSERT INTO suttas (muid, display, link) VALUES ('mn26', 'MN 26', 'https://suttacentral.net/mn26/en/sujato');",
		"INSERT INTO collections (id, name) VALUES (1, 'TestCollection');",
		"INSERT INTO collection_groups (id, collection_id, name, position) VALUES (1, 1, 'Test Group', 1);",
		"INSERT INTO collection_sutta_items (collection_id, muid, position, group_id) VALUES (1, 'mn26', 1, 1);",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("seed missing %q", want)
		}
	}
	if idxQuotes := strings.Index(got, "INSERT INTO quotes"); idxQuotes < 0 {
		t.Fatalf("no quote inserts")
	} else if idxSuttas := strings.Index(got, "DELETE FROM suttas;"); idxSuttas < idxQuotes {
		t.Errorf("sutta section must follow the quote section")
	}
	exportBytes, err := os.ReadFile(export)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(exportBytes), "suttacentral") {
		t.Errorf("export must stay quotes-only, got sutta content:\n%s", exportBytes)
	}
}

func TestExtractGenerateNoTablesKeepsQuoteSeed(t *testing.T) {
	root := t.TempDir()
	dumps := writeTableDumps(t, "\"Quote A.\" - MN 1\n", "")
	seed := filepath.Join(root, "database", "seed.sql")

	if _, err := extractGenerate(dumps, seed, filepath.Join(root, "exports", "e.md"), "TestCollection"); err != nil {
		t.Fatalf("extractGenerate: %v", err)
	}
	seedBytes, _ := os.ReadFile(seed)
	if !strings.Contains(string(seedBytes), "INSERT INTO quotes") {
		t.Errorf("seed missing quote INSERT")
	}
	if strings.Contains(string(seedBytes), "INSERT INTO suttas") {
		t.Errorf("no tables must mean no sutta section:\n%s", seedBytes)
	}
}

func TestExtractGenerateTableParseError(t *testing.T) {
	root := t.TempDir()
	dumps := writeTableDumps(t, "", "**I. G.**\n\n1. ([MN 26](https://suttacentral.net/xx1)).\n")
	_, err := extractGenerate(dumps, filepath.Join(root, "s.sql"), filepath.Join(root, "e.md"), "C")
	if err == nil {
		t.Error("extractGenerate must fail on an unparseable table dump")
	}
}

func TestExtractGenerateRealArtifact(t *testing.T) {
	root := t.TempDir()
	seed := filepath.Join(root, "seed.sql")

	if _, err := extractGenerate("../../dumps", seed, filepath.Join(root, "e.md"), "AriyaSavaka"); err != nil {
		t.Fatalf("extractGenerate: %v", err)
	}
	seedBytes, err := os.ReadFile(seed)
	if err != nil {
		t.Fatal(err)
	}
	got := string(seedBytes)
	for name, want := range map[string]int{
		"suttas registry rows":   strings.Count(got, "INSERT INTO suttas"),
		"collection group rows":  strings.Count(got, "INSERT INTO collection_groups"),
		"sutta membership rows":  strings.Count(got, "INSERT INTO collection_sutta_items"),
		"ariyasavaka collection": strings.Count(got, "INSERT INTO collections (id, name) VALUES (1, 'AriyaSavaka');"),
	} {
		if want == 0 {
			t.Errorf("%s: none found", name)
		}
	}
	if n := strings.Count(got, "INSERT INTO suttas"); n != 471 {
		t.Errorf("suttas rows = %d, want 471", n)
	}
	if n := strings.Count(got, "INSERT INTO collection_groups"); n != 16 {
		t.Errorf("collection group rows = %d, want 16", n)
	}
	if n := strings.Count(got, "INSERT INTO collection_sutta_items"); n != 471 {
		t.Errorf("membership rows = %d, want 471", n)
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

	count, err := extractGenerate(dumps, seed, export, "TestCollection")
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
	_, err := extractGenerate(filepath.Join(root, "nope"), filepath.Join(root, "s.sql"), filepath.Join(root, "e.md"), "TestCollection")
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
	_, err := extractGenerate(t.TempDir(), filepath.Join(filePath, "seed.sql"), filepath.Join(root, "e.md"), "TestCollection")
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
	_, err := extractGenerate(t.TempDir(), dirAsFile, filepath.Join(root, "e.md"), "TestCollection")
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
	_, err := extractGenerate(dumps, filepath.Join(root, "database", "seed.sql"), exportDir, "TestCollection")
	if err == nil {
		t.Error("extractGenerate should fail to write the export into a directory")
	}
}
