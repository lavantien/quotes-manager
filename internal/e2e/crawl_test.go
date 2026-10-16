package e2e

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

const crawlMaxPages = 500

func TestNoDeadLinksCrawl(t *testing.T) {
	app := mergedShared(t)
	base := app.server().URL
	seeds := []string{
		"/",
		"/dashboard",
		"/texts",
		"/words",
		"/graph",
		"/analytics",
		"/pipeline",
		"/discrepancies",
		"/rail/left",
		"/rail/right",
		"/pane/root",
	}
	type queued struct {
		path, from string
	}
	visited := map[string]bool{}
	queue := make([]queued, 0, len(seeds))
	for _, s := range seeds {
		queue = append(queue, queued{s, "seed"})
	}
	fetched := 0
	for len(queue) > 0 {
		q := queue[0]
		queue = queue[1:]
		if visited[q.path] {
			continue
		}
		visited[q.path] = true
		fetched++
		if fetched > crawlMaxPages {
			t.Fatalf("crawl exceeded %d pages, refusing to continue", crawlMaxPages)
		}
		f, err := app.fetch(http.MethodGet, q.path, "", "", "")
		if err != nil {
			t.Fatalf("%s (from %s): %v", q.path, q.from, err)
		}
		if f.status == http.StatusNotFound || f.status >= http.StatusInternalServerError {
			t.Errorf("%s (from %s): status = %d", q.path, q.from, f.status)
			continue
		}
		if !strings.HasPrefix(f.header.Get("Content-Type"), "text/html") {
			continue
		}
		for _, tg := range scanHTMLTags(f.body) {
			for _, attr := range []string{"href", "src"} {
				ref := tg.attrs[attr]
				if ref == "" {
					continue
				}
				if next, ok := sameOriginRef(base, ref); ok && !visited[next] {
					queue = append(queue, queued{next, q.path})
				}
			}
		}
	}
	for _, must := range []string{"/texts/mn1", "/static/app.css", "/static/htmx.min.js", "/collections/1", "/words/eva"} {
		if !visited[must] {
			t.Errorf("crawl never reached %s", must)
		}
	}
	t.Logf("crawl fetched %d pages, %d distinct targets", fetched, len(visited))
}

func sameOriginRef(base, ref string) (string, bool) {
	if i := strings.IndexByte(ref, '#'); i >= 0 {
		ref = ref[:i]
	}
	if ref == "" {
		return "", false
	}
	if strings.HasPrefix(ref, "/") {
		return ref, true
	}
	if !strings.HasPrefix(ref, "http://") && !strings.HasPrefix(ref, "https://") {
		return "", false
	}
	u, err := url.Parse(ref)
	if err != nil {
		return "", false
	}
	b, err := url.Parse(base)
	if err != nil || u.Host != b.Host {
		return "", false
	}
	return u.RequestURI(), true
}
