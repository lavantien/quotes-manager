package graph

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/lavantien/quotes-manager/internal/config"
	_ "github.com/mattn/go-sqlite3"
)

func TestLatestVersionMatchesConfig(t *testing.T) {
	if got := LatestVersion(); got != config.SchemaVersion {
		t.Fatalf("LatestVersion() = %d, want config.SchemaVersion %d", got, config.SchemaVersion)
	}
}

func TestMigrationsContiguous(t *testing.T) {
	for i, m := range migrations {
		want := i + 1
		if m.Version != want {
			t.Fatalf("migrations[%d].Version = %d, want %d", i, m.Version, want)
		}
	}
}

func TestMigrateAppliesSchema(t *testing.T) {
	s := openTempStore(t)
	var v int
	if err := s.DB().QueryRow(`PRAGMA user_version`).Scan(&v); err != nil {
		t.Fatal(err)
	}
	if v != LatestVersion() {
		t.Fatalf("user_version = %d, want %d", v, LatestVersion())
	}
	for _, table := range []string{"node", "edge", "run", "discrepancy", "fts_segment"} {
		var n int
		err := s.DB().QueryRow(
			`SELECT COUNT(*) FROM sqlite_master WHERE name = ?`, table,
		).Scan(&n)
		if err != nil {
			t.Fatal(err)
		}
		if n != 1 {
			t.Fatalf("table %s present %d times, want 1", table, n)
		}
	}
}

func TestMigrateTamperedVersionErrors(t *testing.T) {
	s := openTempStore(t)
	if _, err := s.DB().Exec(`PRAGMA user_version = 0`); err != nil {
		t.Fatal(err)
	}
	if err := s.Migrate(); err == nil {
		t.Fatal("rewound user_version must make migration fail on existing tables")
	}
	if _, err := s.DB().Exec(fmt.Sprintf(`PRAGMA user_version = %d`, LatestVersion()+1)); err != nil {
		t.Fatal(err)
	}
	if err := s.Migrate(); err == nil {
		t.Fatal("user_version from a future binary must be rejected")
	}
}

func TestMigrateV1RunGainsDpdTag(t *testing.T) {
	path := filepath.Join(t.TempDir(), "v1.db")
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	for _, stmt := range []string{schemaV1, `PRAGMA user_version = 1`} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("seed v1 db: %v", err)
		}
	}
	if _, err := db.Exec(
		`INSERT INTO run (bilara_commit, editions_commit, started) VALUES ('bc', 'ec', '2026-01-01T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	if err := s.Migrate(); err != nil {
		t.Fatalf("migrate v1 -> v2: %v", err)
	}
	var v int
	if err := s.DB().QueryRow(`PRAGMA user_version`).Scan(&v); err != nil {
		t.Fatal(err)
	}
	if v != 2 {
		t.Fatalf("user_version = %d; want 2", v)
	}
	var hasTag int
	if err := s.DB().QueryRow(
		`SELECT COUNT(*) FROM pragma_table_info('run') WHERE name = 'dpd_tag'`).Scan(&hasTag); err != nil {
		t.Fatal(err)
	}
	if hasTag != 1 {
		t.Fatal("run.dpd_tag column missing after migration")
	}
	var tag any
	if err := s.DB().QueryRow(`SELECT dpd_tag FROM run WHERE bilara_commit = 'bc'`).Scan(&tag); err != nil {
		t.Fatal(err)
	}
	if tag != nil {
		t.Fatalf("pre-migration run dpd_tag = %v; want NULL", tag)
	}
}

func TestMigrateIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "idem.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	if err := s.Migrate(); err != nil {
		t.Fatalf("first migrate: %v", err)
	}
	if err := s.Migrate(); err != nil {
		t.Fatalf("second migrate: %v", err)
	}
	var tables int
	if err := s.DB().QueryRow(
		`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table'`,
	).Scan(&tables); err != nil {
		t.Fatal(err)
	}
	if tables < 5 {
		t.Fatalf("table count = %d, want at least 5", tables)
	}
}

func TestOpenPragmas(t *testing.T) {
	s := openTempStore(t)
	for _, tc := range []struct{ pragma, want string }{
		{"journal_mode", "wal"},
		{"foreign_keys", "1"},
		{"synchronous", "1"},
	} {
		var got string
		if err := s.DB().QueryRow("PRAGMA " + tc.pragma).Scan(&got); err != nil {
			t.Fatal(err)
		}
		if got != tc.want {
			t.Fatalf("PRAGMA %s = %q, want %q", tc.pragma, got, tc.want)
		}
	}
}

func openTempStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	if err := s.Migrate(); err != nil {
		t.Fatal(err)
	}
	return s
}
