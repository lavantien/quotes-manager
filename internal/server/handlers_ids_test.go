package server_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func TestCorpusIDsExport(t *testing.T) {
	srv := newServer(t, newFake(
		mkQuote(1, "MN 22", "alpha beta"),
		mkQuote(2, "AN 5.34", "gamma delta"),
	))
	rec := do(t, srv, "GET", "/ids.txt", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") {
		t.Errorf("Content-Type = %q, want text/plain", ct)
	}
	if want := "AN 5.34\nMN 22\n"; rec.Body.String() != want {
		t.Errorf("body = %q, want %q", rec.Body.String(), want)
	}
}

func TestCorpusIDsExportDedupesAndSorts(t *testing.T) {
	srv := newServer(t, newFake(
		mkQuote(1, "MN 22", "a"),
		mkQuote(2, "AN 5", "b"),
		mkQuote(3, "MN 22", "c"),
		mkQuote(4, "SN 12", "d"),
	))
	rec := do(t, srv, "GET", "/ids.txt", "")
	if want := "AN 5\nMN 22\nSN 12\n"; rec.Body.String() != want {
		t.Errorf("dedup/sort body = %q, want %q", rec.Body.String(), want)
	}
}

func TestCorpusIDsExportSkipsEmptySuttaIDs(t *testing.T) {
	srv := newServer(t, newFake(
		mkQuote(1, "", "no id here"),
		mkQuote(2, "MN 22", "has id"),
	))
	rec := do(t, srv, "GET", "/ids.txt", "")
	if want := "MN 22\n"; rec.Body.String() != want {
		t.Errorf("empty-skipping body = %q, want %q", rec.Body.String(), want)
	}
}

func TestCorpusIDsExportEmptyCorpus(t *testing.T) {
	srv := newServer(t, newFake())
	rec := do(t, srv, "GET", "/ids.txt", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if rec.Body.String() != "" {
		t.Errorf("empty corpus body = %q, want empty", rec.Body.String())
	}
}

func TestCorpusIDsExportStoreError(t *testing.T) {
	assert500(t, newServer(t, failList{newFake(sampleQuote(1))}), "GET", "/ids.txt", "")
}

func TestCollectionIDsExport(t *testing.T) {
	fs := newFake(
		mkQuote(1, "MN 22", "a"),
		mkQuote(2, "AN 5", "b"),
		mkQuote(3, "SN 12", "c"),
	)
	cid, err := fs.CreateCollection([]int64{1, 2, 2, 3}) // repeated 2 exercises dedup
	if err != nil {
		t.Fatal(err)
	}
	srv := newServer(t, fs)
	rec := do(t, srv, "GET", fmt.Sprintf("/collections/%d/ids.txt", cid), "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if want := "AN 5\nMN 22\nSN 12\n"; rec.Body.String() != want {
		t.Errorf("collection ids body = %q, want %q", rec.Body.String(), want)
	}
}

func TestCollectionIDsExportUnknownCollection(t *testing.T) {
	// Mirrors /collections/{id}/export.txt: an unknown collection yields an empty
	// 200, not a 404.
	srv := newServer(t, newFake(sampleQuote(1)))
	rec := do(t, srv, "GET", "/collections/999/ids.txt", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if rec.Body.String() != "" {
		t.Errorf("unknown collection body = %q, want empty", rec.Body.String())
	}
}

func TestCollectionIDsExportStoreError(t *testing.T) {
	assert500(t, newServer(t, failingStore{}), "GET", "/collections/1/ids.txt", "")
}

func TestLeftRailCopyIDsButton(t *testing.T) {
	srv := newServer(t, newFake(sampleQuote(1)))
	rec := do(t, srv, "GET", "/rail/left", "")
	body := rec.Body.String()
	if !strings.Contains(body, ">Copy ids<") {
		t.Error("left rail missing a Copy ids button")
	}
	if !strings.Contains(body, `data-export="/ids.txt"`) {
		t.Error(`left rail Copy ids should point at /ids.txt`)
	}
}

func TestRightRailCopyIDsButton(t *testing.T) {
	// No active collection: Copy ids is disabled and carries no data-export.
	srv := newServer(t, newFake(sampleQuote(1)))
	body := do(t, srv, "GET", "/rail/right", "").Body.String()
	if !strings.Contains(body, ">Copy ids<") {
		t.Error("right rail missing a Copy ids button")
	}
	if !strings.Contains(body, "disabled>Copy ids<") {
		t.Error("right rail Copy ids should be disabled when no collection is active")
	}

	// Active collection: enabled, pointing at the collection's ids endpoint.
	fs, cid := fakeWithCollection(t)
	srv2 := newServer(t, fs)
	body2 := do(t, srv2, "GET", fmt.Sprintf("/rail/right?col=%d", cid), "").Body.String()
	if strings.Contains(body2, "disabled>Copy ids<") {
		t.Error("right rail Copy ids should be enabled when a collection is active")
	}
	want := fmt.Sprintf(`data-export="/collections/%d/ids.txt"`, cid)
	if !strings.Contains(body2, want) {
		t.Errorf("right rail Copy ids missing %q", want)
	}
}
