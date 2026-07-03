package server_test

import (
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lavantien/quotes-manager/internal/seed"
	"github.com/lavantien/quotes-manager/internal/server"
	"github.com/lavantien/quotes-manager/internal/store"
)

// TestDuplicatesFromCanonicalSeed drives the full pipeline against the real
// SQLite store and the canonical seed: the seed contains the MN 22
// sexual/sensual trio (three near-identical passages that differ only in
// "Bhikkhus"/"Mendicants" and "sexual"/"sensual"), which must surface as one
// duplicate group of size 3 in the rendered left rail.
func TestDuplicatesFromCanonicalSeed(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "quotes.db")
	st, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := seed.EnsureSeeded(st.DB()); err != nil {
		t.Fatalf("seed: %v", err)
	}

	srv := server.New(st)
	rec := do(t, srv, "GET", "/", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()

	if !strings.Contains(body, `MN 22 <span class="rail__count">3</span>`) {
		t.Errorf("expected the seed's MN 22 trio as a duplicate group of 3")
	}
	if !strings.Contains(body, "engage in sexual pleasures without sexual desires") {
		t.Errorf("expected the trio passage text in the rendered page")
	}
	// Only duplicated text ids are surfaced: there must be at least one duplicate
	// group, confirming the feature activates on real data.
	if strings.Count(body, `data-action="focus-quote"`) == 0 {
		t.Errorf("expected duplicate links to be rendered from the seed")
	}
}
