package server

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/lavantien/quotes-manager/internal/store"
)

// White-box unit tests for the unexported pure helpers and error writers.

func TestCollectionLabel(t *testing.T) {
	if got := collectionLabel(store.Collection{ID: 3, Name: ""}); got != "Collection 3" {
		t.Errorf("empty name = %q, want %q", got, "Collection 3")
	}
	if got := collectionLabel(store.Collection{ID: 3, Name: "Keepsakes"}); got != "Keepsakes" {
		t.Errorf("named = %q, want %q", got, "Keepsakes")
	}
}

func TestAttributionOf(t *testing.T) {
	cases := []struct {
		cit, sutta, want string
	}{
		{"the Buddha, MN 22", "MN 22", "the Buddha"},
		{"the Buddha to Pessa, MN 51", "MN 51", "the Buddha to Pessa"},
		{"MN 22", "MN 22", ""}, // citation is just the id
		{"some text", "", ""},  // no sutta id
	}
	for _, c := range cases {
		if got := attributionOf(c.cit, c.sutta); got != c.want {
			t.Errorf("attributionOf(%q, %q) = %q, want %q", c.cit, c.sutta, got, c.want)
		}
	}
}

func TestBuildQuote(t *testing.T) {
	t.Run("defaults attribution", func(t *testing.T) {
		q := buildQuote(url.Values{"content": {"line"}, "text_id": {"MN 22"}})
		if q.Citation != "the Buddha, MN 22" {
			t.Errorf("Citation = %q", q.Citation)
		}
		if q.SuttaID != "MN 22" {
			t.Errorf("SuttaID = %q", q.SuttaID)
		}
	})
	t.Run("preserves attribution", func(t *testing.T) {
		q := buildQuote(url.Values{"content": {"x"}, "attribution": {"Pessa"}, "text_id": {"MN 51"}})
		if q.Citation != "Pessa, MN 51" {
			t.Errorf("Citation = %q", q.Citation)
		}
	})
	t.Run("no text id yields empty citation", func(t *testing.T) {
		q := buildQuote(url.Values{"content": {"x"}})
		if q.SuttaID != "" || q.Citation != "" {
			t.Errorf("SuttaID=%q Citation=%q, want empty", q.SuttaID, q.Citation)
		}
	})
	t.Run("non-canonical text id kept as sutta", func(t *testing.T) {
		q := buildQuote(url.Values{"content": {"x"}, "text_id": {"mystery id"}})
		if q.SuttaID != "mystery id" {
			t.Errorf("SuttaID = %q, want mystery id", q.SuttaID)
		}
	})
}

func TestSplitPassages(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"a\nb", []string{"a", "b"}},
		{"a\n\nb", []string{"a", "b"}},
		{"  spaced  \n\t\n", []string{"spaced"}},
		{"", nil},
		{"   ", nil},
	}
	for _, c := range cases {
		got := splitPassages(c.in)
		if len(got) != len(c.want) {
			t.Errorf("splitPassages(%q) = %#v, want %#v", c.in, got, c.want)
			continue
		}
		for i := range c.want {
			if got[i] != c.want[i] {
				t.Errorf("splitPassages(%q)[%d] = %q, want %q", c.in, i, got[i], c.want[i])
			}
		}
	}
}

func TestParseQueryID(t *testing.T) {
	cases := []struct {
		target, key string
		want        int64
	}{
		{"/?cat=5", "cat", 5},
		{"/?col=7", "col", 7},
		{"/?cat=abc", "cat", 0},
		{"/?cat=-3", "cat", 0},
		{"/?cat=", "cat", 0},
		{"/", "cat", 0},
	}
	for _, c := range cases {
		req := httptest.NewRequest(http.MethodGet, c.target, nil)
		if got := parseQueryID(req, c.key); got != c.want {
			t.Errorf("parseQueryID(%q, %q) = %d, want %d", c.target, c.key, got, c.want)
		}
	}
}

func TestParseIDs(t *testing.T) {
	got := parseIDs([]string{"1", "abc", "-2", "3", "  4  "})
	want := []int64{1, 3, 4}
	if len(got) != len(want) {
		t.Fatalf("parseIDs = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("parseIDs[%d] = %d, want %d", i, got[i], want[i])
		}
	}
}

func TestParseIDBadRequest(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/quotes/abc", nil)
	req.SetPathValue("id", "abc")
	if _, ok := parseID(rec, req, "id"); ok {
		t.Error("non-numeric id should not parse")
	}
	if rec.Code != http.StatusBadRequest {
		t.Errorf("code = %d, want 400", rec.Code)
	}
}

func TestIsHTMX(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if isHTMX(req) {
		t.Error("plain request should not be HTMX")
	}
	req.Header.Set("HX-Request", "true")
	if !isHTMX(req) {
		t.Error("HX-Request: true should be HTMX")
	}
}

func TestHandleStoreErr(t *testing.T) {
	for _, c := range []struct {
		err  error
		code int
	}{
		{store.ErrNotFound, http.StatusNotFound},
		{store.ErrDuplicate, http.StatusConflict},
		{errors.New("boom"), http.StatusInternalServerError},
	} {
		rec := httptest.NewRecorder()
		handleStoreErr(rec, c.err)
		if rec.Code != c.code {
			t.Errorf("err=%v: code = %d, want %d", c.err, rec.Code, c.code)
		}
	}
}
