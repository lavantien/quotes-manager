// Package storetest provides a clone of the main quotes database as a test
// fixture, so integration tests run against the real corpus (quotes,
// collections, categories) and can mutate it freely. Import it only from
// _test.go files so the embedded fixture never ships in the server binary.
package storetest

import (
	_ "embed"
	"path/filepath"
	"testing"

	"github.com/lavantien/quotes-manager/internal/store"
)

//go:embed testdata/quotes_fixture.sql
var fixtureSQL string

// CloneFixture opens a temp SQLite database, loads the clone-of-main fixture
// into it, and returns a *store.SQLiteStore the caller can mutate freely. The
// database is closed automatically when the test ends.
func CloneFixture(t testing.TB) *store.SQLiteStore {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "quotes.db"))
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	if _, err := s.DB().Exec(fixtureSQL); err != nil {
		t.Fatalf("load fixture: %v", err)
	}
	return s
}
