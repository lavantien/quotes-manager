package cli

import (
	"bytes"
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lavantien/quotes-manager/internal/config"
	"github.com/lavantien/quotes-manager/internal/seed"
	"github.com/lavantien/quotes-manager/internal/store"
)

func TestFixtureRunRoundTripsAllTables(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "main.db")
	outPath := filepath.Join(dir, "fixture.sql")
	clonePath := filepath.Join(dir, "clone.db")

	src, err := store.Open(srcPath)
	if err != nil {
		t.Fatalf("open source: %v", err)
	}
	if err := seed.EnsureSeeded(src.DB()); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := src.Close(); err != nil {
		t.Fatalf("close source: %v", err)
	}

	if err := fixtureRun(srcPath, outPath); err != nil {
		t.Fatalf("fixtureRun: %v", err)
	}
	dump, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	if len(dump) == 0 {
		t.Fatal("fixture is empty")
	}

	clone, err := store.Open(clonePath)
	if err != nil {
		t.Fatalf("open clone: %v", err)
	}
	defer func() { _ = clone.Close() }()
	if _, err := clone.DB().Exec(string(dump)); err != nil {
		t.Fatalf("load dump into clone: %v", err)
	}

	ref, err := store.Open(srcPath)
	if err != nil {
		t.Fatalf("reopen source: %v", err)
	}
	defer func() { _ = ref.Close() }()

	for _, tb := range fixtureTables {
		want := countRows(t, ref.DB(), tb.name)
		got := countRows(t, clone.DB(), tb.name)
		if got != want {
			t.Errorf("table %s: clone has %d rows, source has %d", tb.name, got, want)
		}
	}
}

func countRows(t *testing.T, db *sql.DB, table string) int {
	t.Helper()
	var n int
	if err := db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&n); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return n
}

func TestRunFixtureDefaultsFromConfigAndReports(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Defaults()
	cfg.Quotes.DBPath = filepath.Join(dir, "main.db")
	oldOut := stdout
	stdout = new(bytes.Buffer)
	err := runFixture(context.Background(), cfg, []string{"-out", filepath.Join(dir, "fixture.sql")})
	out := stdout.(*bytes.Buffer).String()
	stdout = oldOut
	if err != nil {
		t.Fatalf("runFixture: %v", err)
	}
	if want := "wrote " + filepath.Join(dir, "fixture.sql"); !strings.Contains(out, want) {
		t.Errorf("output = %q, want it to contain %q", out, want)
	}
	if _, err := os.Stat(filepath.Join(dir, "fixture.sql")); err != nil {
		t.Errorf("fixture not written: %v", err)
	}
}

func TestRunFixtureRejectsUnexpectedArg(t *testing.T) {
	if err := runFixture(context.Background(), config.Defaults(), []string{"extra"}); err == nil {
		t.Error("fixture must reject positional arguments")
	}
}
