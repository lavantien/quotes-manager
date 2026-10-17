package server_test

import (
	"net/http"
	"strings"
	"testing"
)

func TestUnknownGetPathServesNotFoundPage(t *testing.T) {
	srv := newServer(t, newFake(sampleQuote(1)))
	for _, path := range []string{"/no-such-page", "/quotes/7", "/quotes/import", "/texts/mn1/deep/nested"} {
		rec := do(t, srv, "GET", path, "")
		if rec.Code != http.StatusNotFound {
			t.Errorf("GET %s: status = %d, want 404", path, rec.Code)
			continue
		}
		body := rec.Body.String()
		for _, marker := range []string{"<h1>not found</h1>", "does not exist.", `href="/dashboard"`} {
			if !strings.Contains(body, marker) {
				t.Errorf("GET %s: missing notfound marker %q", path, marker)
			}
		}
		if strings.Contains(body, `id="quote-list"`) {
			t.Errorf("GET %s: served the quotes home instead of the notfound page", path)
		}
	}
}

func TestRootPathStillServesQuotesHome(t *testing.T) {
	srv := newServer(t, newFake(sampleQuote(1)))
	for _, target := range []string{"/", "/?col=2&rq=x"} {
		rec := do(t, srv, "GET", target, "")
		if rec.Code != http.StatusOK {
			t.Errorf("GET %s: status = %d, want 200", target, rec.Code)
			continue
		}
		if !strings.Contains(rec.Body.String(), `id="quote-list"`) {
			t.Errorf("GET %s: missing quotes home marker", target)
		}
	}
	if rec := do(t, srv, "GET", "/", ""); !strings.Contains(rec.Body.String(), "Human beings are shady") {
		t.Error("GET /: missing quote body")
	}
}

func TestFaviconProbeIsNoContent(t *testing.T) {
	srv := newServer(t, newFake())
	rec := do(t, srv, "GET", "/favicon.ico", "")
	if rec.Code != http.StatusNoContent {
		t.Errorf("GET /favicon.ico: status = %d, want 204", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("GET /favicon.ico: body = %q, want empty", rec.Body.String())
	}
}

func TestPostUnknownPathStaysMethodNotAllowed(t *testing.T) {
	srv := newServer(t, newFake())
	rec := do(t, srv, "POST", "/no-such-page", "")
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST /no-such-page: status = %d, want 405", rec.Code)
	}
	if allow := rec.Header().Get("Allow"); !strings.Contains(allow, "GET") {
		t.Errorf("POST /no-such-page: Allow = %q, want GET listed", allow)
	}
}
