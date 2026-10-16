package e2e

import (
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
)

var quotesPatterns = []string{
	"GET /",
	"GET /quotes",
	"GET /quotes/new",
	"GET /quotes/import/form",
	"POST /quotes/import",
	"POST /quotes",
	"GET /quotes/{id}/edit",
	"POST /quotes/{id}",
	"DELETE /quotes/{id}",
	"POST /quotes/delete",
	"GET /quotes/{id}/copy",
	"GET /quotes/{id}/categories",
	"GET /quotes/{id}/categories/edit",
	"POST /quotes/{id}/categories",
	"GET /export.txt",
	"GET /backup.json",
	"POST /restore",
	"GET /collections/{cid}",
	"POST /collections",
	"POST /collections/{cid}/items",
	"POST /collections/{cid}/insert",
	"POST /collections/{cid}/rename",
	"DELETE /collections/{cid}",
	"POST /collections/{cid}/reorder",
	"GET /collections/{cid}/export.txt",
	"GET /ids.txt",
	"GET /collections/{cid}/ids.txt",
	"GET /api/v1/quotes",
	"GET /api/v1/quotes/{id}",
	"GET /pane/root",
	"GET /pane/collection",
	"GET /pane/check",
	"POST /check",
	"GET /search/root",
	"GET /search/collection",
	"GET /rail/left",
	"GET /rail/right",
	"POST /duplicates/{repID}/merge",
	"GET /categories/{ctid}",
	"GET /categories/{ctid}/export.txt",
	"POST /categories",
	"POST /categories/{ctid}/rename",
	"DELETE /categories/{ctid}",
	"GET /static/",
}

var canonPatterns = []string{
	"GET /api/v1/version",
	"GET /api/v1/stats",
	"GET /api/v1/segments",
	"GET /api/v1/search",
	"GET /api/v1/tree",
	"GET /api/v1/word/form",
	"GET /api/v1/word/headword",
	"GET /api/v1/runs",
	"GET /api/v1/discrepancies",
	"GET /api/v1/graph/word",
	"GET /api/v1/graph/root",
	"GET /api/v1/graph/tree",
	"GET /api/v1/graph/coverage",
	"POST /api/v1/jobs",
	"GET /api/v1/jobs",
	"GET /api/v1/jobs/{id}",
	"GET /api/v1/jobs/{id}/logs",
	"POST /api/v1/jobs/{id}/cancel",
	"GET /api/v1/analytics/catalog",
	"GET /api/v1/analytics/{name}",
	"GET /api/v1/analytics/{name}/csv",
	"POST /api/v1/analytics/query",
	"GET /api/v1/export",
	"GET /api/v1/export/parquet/{table}",
	"GET /api/v1/report",
	"GET /api/v1/report/{date}/{file}",
	"GET /dashboard",
	"GET /texts",
	"GET /texts/branch",
	"GET /texts/{muid}",
	"GET /search",
	"GET /words",
	"GET /words/{lemma}",
	"GET /graph",
	"GET /analytics",
	"POST /analytics",
	"GET /pipeline",
	"POST /pipeline",
	"GET /discrepancies",
	"GET /partials/job/{id}",
	"GET /api/",
	"POST /api/",
}

func TestMergedRouteInventory(t *testing.T) {
	app := mergedAppFor(t, false)
	got, err := muxPatterns(app.mux())
	if err != nil {
		t.Fatalf("walk mux patterns: %v", err)
	}
	sort.Strings(got)
	want := append(append([]string{}, quotesPatterns...), canonPatterns...)
	sort.Strings(want)
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("merged route inventory drifted (got %d, want %d):\nmissing: %v\nextra: %v",
			len(got), len(want), diff(want, got), diff(got, want))
	}
}

func TestMergedRouteProbes(t *testing.T) {
	app := mergedAppFor(t, false)
	m := app.mux()
	for _, probe := range []struct {
		method, target, want string
	}{
		{http.MethodGet, "/", "GET /"},
		{http.MethodGet, "/quotes/7", "GET /"},
		{http.MethodGet, "/api/v1/quotes/7", "GET /api/v1/quotes/{id}"},
		{http.MethodGet, "/api/v1/quotes", "GET /api/v1/quotes"},
		{http.MethodGet, "/texts/mn1", "GET /texts/{muid}"},
		{http.MethodGet, "/texts/branch", "GET /texts/branch"},
		{http.MethodGet, "/words/suta", "GET /words/{lemma}"},
		{http.MethodGet, "/static/app.css", "GET /static/"},
		{http.MethodGet, "/api/v1/nope", "GET /api/"},
		{http.MethodPost, "/api/nope", "POST /api/"},
		{http.MethodGet, "/dashboard", "GET /dashboard"},
		{http.MethodGet, "/categories/3", "GET /categories/{ctid}"},
		{http.MethodPost, "/collections/5/reorder", "POST /collections/{cid}/reorder"},
	} {
		req := httptest.NewRequest(probe.method, probe.target, nil)
		if _, pattern := m.Handler(req); pattern != probe.want {
			t.Errorf("%s %s matched %q, want %q", probe.method, probe.target, pattern, probe.want)
		}
	}
}

func diff(a, b []string) []string {
	set := map[string]bool{}
	for _, s := range b {
		set[s] = true
	}
	var out []string
	for _, s := range a {
		if !set[s] {
			out = append(out, s)
		}
	}
	return out
}

func (a *mergedApp) mux() *http.ServeMux {
	return a.root
}
