package server_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/lavantien/quotes-manager/internal/server"
	"github.com/lavantien/quotes-manager/internal/store"
	"github.com/lavantien/quotes-manager/internal/store/storetest"
)

func TestBackupJSON(t *testing.T) {
	srv := server.New(storetest.CloneFixture(t))

	rec := do(t, srv, "GET", "/backup.json", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	if cd := rec.Header().Get("Content-Disposition"); !strings.Contains(cd, "attachment") || !strings.Contains(cd, "quotes-backup.json") {
		t.Errorf("Content-Disposition = %q, want attachment with quotes-backup.json", cd)
	}
	var dump store.Dump
	if err := json.Unmarshal(rec.Body.Bytes(), &dump); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if dump.Version != store.DumpVersion {
		t.Errorf("Version = %d, want %d", dump.Version, store.DumpVersion)
	}
	if len(dump.Quotes) != 109 {
		t.Errorf("Quotes = %d, want 109", len(dump.Quotes))
	}
	// The fixture seeds one sample collection and three sample categories.
	if len(dump.Collections) != 1 {
		t.Errorf("Collections = %d, want 1", len(dump.Collections))
	}
	if len(dump.Categories) != 3 {
		t.Errorf("Categories = %d, want 3", len(dump.Categories))
	}
}

func TestRestoreReplacesAll(t *testing.T) {
	srv := server.New(storetest.CloneFixture(t))

	// Download the backup, trim to the first 3 quotes (and drop memberships
	// that reference removed quotes), then upload it back.
	rec := do(t, srv, "GET", "/backup.json", "")
	var dump store.Dump
	if err := json.Unmarshal(rec.Body.Bytes(), &dump); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	keep := map[int64]bool{}
	for _, q := range dump.Quotes[:3] {
		keep[q.ID] = true
	}
	dump.Quotes = dump.Quotes[:3]
	dump.Collections = filterDumpCollections(dump.Collections, keep)
	dump.Categories = filterDumpCategories(dump.Categories, keep)
	body, err := json.Marshal(dump)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	rec2 := do(t, srv, "POST", "/restore", string(body), "Content-Type", "application/json")
	if rec2.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec2.Code)
	}

	after := do(t, srv, "GET", "/", "").Body.String()
	containsBody(t, after, ">3 blocks<")
}

func TestRestoreRejectsBadJSON(t *testing.T) {
	srv := server.New(storetest.CloneFixture(t))
	rec := do(t, srv, "POST", "/restore", "not json", "Content-Type", "application/json")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestRestoreRejectsBadVersion(t *testing.T) {
	srv := server.New(storetest.CloneFixture(t))
	rec := do(t, srv, "POST", "/restore", `{"version":99}`, "Content-Type", "application/json")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func filterDumpCollections(in []store.CollectionDump, keep map[int64]bool) []store.CollectionDump {
	var out []store.CollectionDump
	for _, c := range in {
		var items []int64
		for _, id := range c.Items {
			if keep[id] {
				items = append(items, id)
			}
		}
		if len(items) > 0 {
			out = append(out, store.CollectionDump{ID: c.ID, Name: c.Name, Items: items})
		}
	}
	return out
}

func filterDumpCategories(in []store.CategoryDump, keep map[int64]bool) []store.CategoryDump {
	var out []store.CategoryDump
	for _, c := range in {
		var items []int64
		for _, id := range c.Items {
			if keep[id] {
				items = append(items, id)
			}
		}
		if len(items) > 0 {
			out = append(out, store.CategoryDump{ID: c.ID, Name: c.Name, Items: items})
		}
	}
	return out
}
