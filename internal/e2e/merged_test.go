package e2e

import (
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"
)

const seedCorpusTotal = 109

var quoteIDRe = regexp.MustCompile(`id="quote-[0-9]+"`)

type quotesEnvelope struct {
	Rows []struct {
		ID      int64  `json:"id"`
		SuttaID string `json:"sutta_id"`
	} `json:"rows"`
	Total int `json:"total"`
}

func TestMergedHermeticE2E(t *testing.T) {
	app := mergedShared(t)

	home := app.get(t, "/")
	if home.status != http.StatusOK {
		t.Fatalf("/: status = %d (%s)", home.status, clip(home.body))
	}
	if !strings.Contains(home.body, `id="quote-list"`) {
		t.Errorf("/: missing #quote-list (%s)", clip(home.body))
	}
	if n := len(quoteIDRe.FindAllString(home.body, -1)); n != seedCorpusTotal {
		t.Errorf("/: quote blocks = %d, want %d (seed corpus)", n, seedCorpusTotal)
	}

	dash := app.get(t, "/dashboard")
	if dash.status != http.StatusOK || !strings.Contains(dash.body, "node:segment") {
		t.Errorf("/dashboard = %d, canon stats missing (%s)", dash.status, clip(dash.body))
	}

	text := app.get(t, "/texts/mn1")
	if text.status != http.StatusOK {
		t.Fatalf("/texts/mn1: status = %d (%s)", text.status, clip(text.body))
	}
	for _, marker := range []string{"/words?form=", "/words?form=suta%E1%B9%81"} {
		if !strings.Contains(text.body, marker) {
			t.Errorf("/texts/mn1: missing word-link marker %q", marker)
		}
	}

	quotes := quotesEnvelope{}
	decodeJSON(t, app.get(t, "/api/v1/quotes"), &quotes)
	if quotes.Total != seedCorpusTotal || len(quotes.Rows) != 50 {
		t.Errorf("/api/v1/quotes default = %d rows of total %d, want 50 of %d",
			len(quotes.Rows), quotes.Total, seedCorpusTotal)
	}
	decodeJSON(t, app.get(t, "/api/v1/quotes?limit=2"), &quotes)
	if len(quotes.Rows) != 2 || quotes.Total != seedCorpusTotal {
		t.Errorf("/api/v1/quotes?limit=2 = %d rows of total %d", len(quotes.Rows), quotes.Total)
	}
	decodeJSON(t, app.get(t, "/api/v1/quotes?limit=999"), &quotes)
	if len(quotes.Rows) != seedCorpusTotal {
		t.Errorf("/api/v1/quotes?limit=999 = %d rows, want the %d-quote corpus under MaxLimit",
			len(quotes.Rows), seedCorpusTotal)
	}
	wantStatusBody(t, app.get(t, "/api/v1/quotes?limit=abc"), http.StatusBadRequest, "invalid_param")
	wantStatusBody(t, app.get(t, "/api/v1/quotes?offset=-1"), http.StatusBadRequest, "invalid_param")

	one := app.get(t, "/api/v1/quotes/1")
	if one.status != http.StatusOK || !strings.Contains(one.body, `"sutta_id"`) {
		t.Errorf("/api/v1/quotes/1 = %d (%s)", one.status, clip(one.body))
	}
	wantStatusBody(t, app.get(t, "/api/v1/quotes/999999"), http.StatusNotFound, "not_found")
	wantStatusBody(t, app.get(t, "/api/v1/quotes/abc"), http.StatusNotFound, "not_found")

	citation := ""
	for _, tg := range scanHTMLTags(home.body) {
		if tg.name == "a" && strings.HasPrefix(tg.attrs["href"], "/texts/") {
			citation = tg.attrs["href"]
			break
		}
	}
	if citation == "" {
		t.Fatal("/: no /texts/ citation href rendered")
	}
	resolved := app.get(t, citation)
	if resolved.status != http.StatusOK {
		t.Errorf("citation %s: status = %d, want 200", citation, resolved.status)
	}

	static := map[string]string{
		"/static/app.css":          "--",
		"/static/layout.css":       ".",
		"/static/nav.css":          ".nav",
		"/static/htmx.min.js":      `this.version="4.0.0"`,
		"/static/app.js":           "htmx",
		"/static/canon.css":        "--bg",
		"/static/canon.js":         "job",
		"/static/cytoscape.min.js": "cytoscape",
	}
	for asset, marker := range static {
		f := app.get(t, asset)
		if f.status != http.StatusOK {
			t.Errorf("%s: status = %d", asset, f.status)
			continue
		}
		if cc := f.header.Get("Cache-Control"); cc != "public, max-age=86400" {
			t.Errorf("%s: cache-control = %q", asset, cc)
		}
		if !strings.Contains(f.body, marker) {
			t.Errorf("%s: missing marker %q", asset, marker)
		}
	}
}

func TestMergedOriginPost(t *testing.T) {
	app := mergedAppFor(t, true)
	form := url.Values{"content": {"Merged e2e origin post."}, "text_id": {"MN 22"}}
	same := app.postForm(t, "/quotes", form, app.server().URL)
	if same.status != http.StatusOK {
		t.Errorf("same-origin POST /quotes = %d, want 200 (%s)", same.status, clip(same.body))
	}
	cross := app.postForm(t, "/quotes", form, "http://evil.example")
	wantStatusBody(t, cross, http.StatusForbidden, "origin_rejected")
}
