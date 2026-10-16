package e2e

import (
	"net/http"
	"strings"
	"testing"
)

func TestContentSecurityPolicyLint(t *testing.T) {
	app := mergedShared(t)
	pages := []string{
		"/",
		"/collections/1",
		"/rail/left",
		"/search/root",
		"/dashboard",
		"/texts",
		"/words",
	}
	for _, page := range pages {
		f := app.get(t, page)
		if f.status != http.StatusOK {
			t.Fatalf("%s: status = %d, want 200 (%s)", page, f.status, clip(f.body))
		}
		if ct := f.header.Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
			t.Fatalf("%s: content-type = %q, want text/html", page, ct)
		}
		csp := f.header.Get("Content-Security-Policy")
		if csp != "default-src 'self'" {
			t.Errorf("%s: Content-Security-Policy = %q, want exactly default-src 'self'", page, csp)
		}
		lintInlineScripts(t, page, f.body)
		lintEventAttributes(t, page, f.body)
	}
}

func lintInlineScripts(t *testing.T, page, body string) {
	t.Helper()
	for _, tg := range scanHTMLTags(body) {
		if tg.name != "script" {
			continue
		}
		if _, ok := tg.attrs["src"]; !ok {
			t.Errorf("%s: inline <script> without src: %v", page, tg.attrs)
		}
	}
}

func lintEventAttributes(t *testing.T, page, body string) {
	t.Helper()
	for _, tg := range scanHTMLTags(body) {
		for key := range tg.attrs {
			if len(key) > 2 && strings.HasPrefix(key, "on") {
				t.Errorf("%s: <%s> carries event attribute %q", page, tg.name, key)
			}
		}
	}
}
