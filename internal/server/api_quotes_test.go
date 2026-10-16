package server_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/lavantien/quotes-manager/internal/store"
)

type apiQuotesListJSON struct {
	Rows  []store.Quote `json:"rows"`
	Total int           `json:"total"`
}

type apiErrorEnvelopeJSON struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func decodeAPIQuotesList(t *testing.T, rec *httptest.ResponseRecorder) apiQuotesListJSON {
	t.Helper()
	var list apiQuotesListJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("body is not json: %v (%s)", err, rec.Body.String())
	}
	return list
}

func manyQuotes(n int) []store.Quote {
	qs := make([]store.Quote, n)
	for i := range qs {
		qs[i] = sampleQuote(int64(i + 1))
	}
	return qs
}

func TestAPIQuotesListClamps(t *testing.T) {
	srv := newServer(t, newFake(manyQuotes(501)...))

	list := decodeAPIQuotesList(t, do(t, srv, "GET", "/api/v1/quotes", ""))
	if list.Total != 501 || len(list.Rows) != 50 {
		t.Fatalf("default: total = %d, rows = %d, want 501/50 (DefaultLimit)", list.Total, len(list.Rows))
	}

	list = decodeAPIQuotesList(t, do(t, srv, "GET", "/api/v1/quotes?limit=99999", ""))
	if list.Total != 501 || len(list.Rows) != 500 {
		t.Fatalf("limit=99999: total = %d, rows = %d, want 501/500 (MaxLimit)", list.Total, len(list.Rows))
	}

	list = decodeAPIQuotesList(t, do(t, srv, "GET", "/api/v1/quotes?limit=0", ""))
	if list.Total != 501 || len(list.Rows) != 50 {
		t.Fatalf("limit=0: total = %d, rows = %d, want 501/50 (default)", list.Total, len(list.Rows))
	}
}

func TestAPIQuotesListPaging(t *testing.T) {
	srv := newServer(t, newFake(sampleQuote(1), sampleQuote(2), sampleQuote(3)))

	list := decodeAPIQuotesList(t, do(t, srv, "GET", "/api/v1/quotes?limit=2", ""))
	if list.Total != 3 || len(list.Rows) != 2 {
		t.Fatalf("limit=2: total = %d, rows = %d, want 3/2", list.Total, len(list.Rows))
	}

	list = decodeAPIQuotesList(t, do(t, srv, "GET", "/api/v1/quotes?limit=2&offset=2", ""))
	if list.Total != 3 || len(list.Rows) != 1 || list.Rows[0].ID != 3 {
		t.Fatalf("limit=2&offset=2: total = %d, rows = %d, want 3/1 ending at id 3", list.Total, len(list.Rows))
	}

	list = decodeAPIQuotesList(t, do(t, srv, "GET", "/api/v1/quotes?offset=99", ""))
	if list.Total != 3 || len(list.Rows) != 0 || list.Rows == nil {
		t.Fatalf("offset=99: total = %d, rows = %v, want 3/empty-non-nil", list.Total, list.Rows)
	}

	rec := do(t, srv, "GET", "/api/v1/quotes", "")
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("content-type = %q", ct)
	}
}

func TestAPIQuotesListBadParams(t *testing.T) {
	srv := newServer(t, newFake(sampleQuote(1)))
	for _, q := range []string{"limit=x", "offset=-1", "offset=y"} {
		rec := do(t, srv, "GET", "/api/v1/quotes?"+q, "")
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%s: status = %d, want 400", q, rec.Code)
		}
		var env apiErrorEnvelopeJSON
		if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
			t.Fatalf("%s: body is not json: %v", q, err)
		}
		if env.Error.Code != "invalid_param" {
			t.Fatalf("%s: code = %s, want invalid_param", q, env.Error.Code)
		}
	}
}

func TestAPIQuoteGet(t *testing.T) {
	srv := newServer(t, newFake(sampleQuote(1)))

	rec := do(t, srv, "GET", "/api/v1/quotes/1", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var q store.Quote
	if err := json.Unmarshal(rec.Body.Bytes(), &q); err != nil {
		t.Fatalf("body is not json: %v", err)
	}
	if q.ID != 1 || q.SuttaID != "MN 22" || q.Citation != "the Buddha, MN 22" {
		t.Fatalf("quote = %+v", q)
	}

	for _, p := range []string{"/api/v1/quotes/999", "/api/v1/quotes/abc"} {
		rec := do(t, srv, "GET", p, "")
		if rec.Code != http.StatusNotFound {
			t.Fatalf("%s: status = %d, want 404", p, rec.Code)
		}
		var env apiErrorEnvelopeJSON
		if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
			t.Fatalf("%s: body is not json: %v", p, err)
		}
		if env.Error.Code != "not_found" {
			t.Fatalf("%s: code = %s, want not_found", p, env.Error.Code)
		}
	}
}

func TestAPIQuotesOnSharedMux(t *testing.T) {
	srv := newServer(t, newFake(sampleQuote(1)))
	m := http.NewServeMux()
	srv.RegisterRoutes(m)
	rec := httptest.NewRecorder()
	m.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/quotes", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
}
