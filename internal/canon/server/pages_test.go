package server

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/lavantien/quotes-manager/internal/canon/jobs"
)

func getPage(t *testing.T, h http.Handler, target string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func wantHTML(t *testing.T, rec *httptest.ResponseRecorder, status int, markers ...string) *httptest.ResponseRecorder {
	t.Helper()
	if rec.Code != status {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("content type = %q", ct)
	}
	for _, want := range markers {
		if !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("body missing %q:\n%s", want, rec.Body.String())
		}
	}
	return rec
}

func wantPlain(t *testing.T, rec *httptest.ResponseRecorder, status int, marker string) {
	t.Helper()
	if rec.Code != status {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), marker) {
		t.Fatalf("body missing %q:\n%s", marker, rec.Body.String())
	}
}

func TestPageDashboard(t *testing.T) {
	srv, _ := newFixtureServer(t)
	h := srv.Routes()

	rec := wantHTML(t, getPage(t, h, "/"), http.StatusOK,
		"node:text", ">2<", "mn-div", "run 1", "stats-grid")
	if strings.Contains(rec.Body.String(), "id=\"job-status\"") {
		t.Fatal("no job ran; the dashboard must not render a job section")
	}

	withFakeExec(t, srv, func(context.Context, io.Writer) error { return nil })
	if rec := postBody(t, h, "/api/v1/jobs", `{"stage":"sync"}`); rec.Code != http.StatusAccepted {
		t.Fatalf("start job = %d", rec.Code)
	}
	wantHTML(t, getPage(t, h, "/"), http.StatusOK, "id=\"job-status\"", `data-state="`)
}

func TestPageTextsTree(t *testing.T) {
	srv, _ := newFixtureServer(t)
	h := srv.Routes()

	wantHTML(t, getPage(t, h, "/texts"), http.StatusOK,
		`hx-get="/texts/branch?node=mn-div"`, "Middle Length Discourses")

	rec := wantHTML(t, getPage(t, h, "/texts/branch?node=mn-div"), http.StatusOK,
		`href="/texts/mn1"`, "Mūlapariyāya")
	if strings.Contains(rec.Body.String(), "<html") {
		t.Fatal("branch must render the partial, not the layout")
	}
	if strings.Contains(rec.Body.String(), "hx-get") {
		t.Fatal("expanded branch must not carry the lazy-load attributes")
	}

	wantPlain(t, getPage(t, h, "/texts/branch"), http.StatusBadRequest, "node")
}

func TestPageTextReading(t *testing.T) {
	srv, _ := newFixtureServer(t)
	h := srv.Routes()

	wantHTML(t, getPage(t, h, "/texts/mn1"), http.StatusOK,
		"Mūlapariyāya", `href="/words?form=pariy%C4%81ya"`,
		`href="/words?form=ekodh%C4%81na"`, `href="/words?form=%C4%81sav%C4%81"`,
		"the relational", "(sujato)", "showing 1 to 3 of 3")

	wantHTML(t, getPage(t, h, "/texts/mn1?limit=2&offset=2"), http.StatusOK,
		"showing 3 to 3 of 3", "prev")
	wantPlain(t, getPage(t, h, "/texts/mn1?limit=x"), http.StatusBadRequest, "limit")
	wantPlain(t, getPage(t, h, "/texts/mn1?offset=-1"), http.StatusBadRequest, "offset")
}

func TestPageTextWindowHonorsLimit(t *testing.T) {
	srv, _ := newFixtureServer(t)
	h := srv.Routes()

	rec := wantHTML(t, getPage(t, h, "/texts/mn1?limit=2"), http.StatusOK,
		`href="/words?form=pariy%C4%81ya"`, `href="/words?form=ekodh%C4%81na"`,
		"showing 1 to 2 of 3", "next")
	if strings.Contains(rec.Body.String(), "āsavā") {
		t.Fatalf("limit 2 must cut the third segment:\n%s", rec.Body)
	}
}

func TestPageTextWindowHonorsOffset(t *testing.T) {
	srv, _ := newFixtureServer(t)
	h := srv.Routes()

	rec := wantHTML(t, getPage(t, h, "/texts/mn1?limit=1&offset=1"), http.StatusOK,
		`href="/words?form=ekodh%C4%81na"`, "showing 2 to 2 of 3")
	body := rec.Body.String()
	if strings.Contains(body, "pariy%C4%81ya") {
		t.Fatalf("offset 1 must skip the first segment:\n%s", body)
	}
	if strings.Contains(body, "āsavā") {
		t.Fatalf("limit 1 must cut the third segment:\n%s", body)
	}

	rec = wantHTML(t, getPage(t, h, "/texts/mn1?limit=1&offset=9"), http.StatusOK, "of 3")
	body = rec.Body.String()
	for _, gone := range []string{"pariy%C4%81ya", "ekodh%C4%81na", "%C4%81sav%C4%81"} {
		if strings.Contains(body, gone) {
			t.Fatalf("offset 9 must render no segments, found %s:\n%s", gone, body)
		}
	}
}

func TestPageHugeOffset(t *testing.T) {
	srv, _ := newFixtureServer(t)
	h := srv.Routes()

	rec := wantHTML(t, getPage(t, h, "/texts/mn1?offset=9223372036854775807"), http.StatusOK, "of 3")
	body := rec.Body.String()
	for _, gone := range []string{"pariy%C4%81ya", "ekodh%C4%81na", "%C4%81sav%C4%81"} {
		if strings.Contains(body, gone) {
			t.Fatalf("huge offset must render no segments, found %s:\n%s", gone, body)
		}
	}

	rec = wantHTML(t, getPage(t, h, "/words/dhamma?offset=9223372036854775807"), http.StatusOK, "of 3")
	if strings.Contains(rec.Body.String(), `href="/texts/mn`) {
		t.Fatalf("huge offset must render no usage refs:\n%s", rec.Body.String())
	}
}

func TestTokenizeSegmentRules(t *testing.T) {
	toks := tokenizeSegment("evaṃ, dhamma pe", 2)
	want := []struct {
		text   string
		isWord bool
	}{
		{"evaṃ", true}, {", ", false}, {"dhamma", true}, {" ", false}, {"pe", false},
	}
	if len(toks) != len(want) {
		t.Fatalf("tokens = %+v", toks)
	}
	for i, w := range want {
		if toks[i].Text != w.text || toks[i].IsWord != w.isWord {
			t.Fatalf("token %d = %+v, want %+v (all: %+v)", i, toks[i], w, toks)
		}
	}
	if got := tokenizeSegment("", 2); len(got) != 0 {
		t.Fatalf("empty segment = %+v", got)
	}
}

func TestPageSearch(t *testing.T) {
	srv, _ := newFixtureServer(t)
	h := srv.Routes()

	target := "/search?text=" + url.QueryEscape("the relational") + "&lang=en"
	wantHTML(t, getPage(t, h, target), http.StatusOK,
		`href="/texts/mn1#0.1"`, "pariyāya", "the relational")

	req := httptest.NewRequest(http.MethodGet, target, nil)
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), "<html") {
		t.Fatalf("htmx search = %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "mn1") {
		t.Fatalf("htmx partial missing results:\n%s", rec.Body.String())
	}

	wantPlain(t, getPage(t, h, "/search?text=x&lang=fr"), http.StatusBadRequest, "lang")
	wantHTML(t, getPage(t, h, "/search"), http.StatusOK, "no results")
}

func TestPageWordsAndWord(t *testing.T) {
	srv, _ := newFixtureServer(t)
	h := srv.Routes()

	wantHTML(t, getPage(t, h, "/words?form=dhamma"), http.StatusOK,
		`href="/words/dhamma"`, "the teaching; nature", "dhṛ", ">3<")
	wantHTML(t, getPage(t, h, "/words?form=unknown"), http.StatusOK, "no entries")

	wantHTML(t, getPage(t, h, "/words/dhamma"), http.StatusOK,
		"3 segments", `href="/texts/mn1#0.1"`, `href="/texts/mn1#0.2"`,
		`href="/texts/mn2#0.1"`,
		`data-graph-url="/api/v1/graph/word?token=dhamma"`, `data-layout="concentric"`,
		"showing 1 to 3 of 3")
	wantHTML(t, getPage(t, h, "/words/dhamma?limit=2"), http.StatusOK,
		`href="/words/dhamma?limit=2&amp;offset=2"`)
	wantHTML(t, getPage(t, h, "/words/nope"), http.StatusOK, "no entries")
}

func TestPageWordWindowHonorsLimit(t *testing.T) {
	srv, _ := newFixtureServer(t)
	h := srv.Routes()

	rec := wantHTML(t, getPage(t, h, "/words/dhamma?limit=2"), http.StatusOK,
		`href="/texts/mn1#0.1"`, `href="/texts/mn1#0.2"`, "showing 1 to 2 of 3")
	if strings.Contains(rec.Body.String(), `href="/texts/mn2#0.1"`) {
		t.Fatalf("limit 2 must cut the third usage ref:\n%s", rec.Body)
	}
}

func TestPageGraphDatasets(t *testing.T) {
	srv, _ := newFixtureServer(t)
	h := srv.Routes()

	wantHTML(t, getPage(t, h, "/graph"), http.StatusOK,
		`data-api="/api/v1/graph/word"`, `data-param="token"`,
		`data-api="/api/v1/graph/root"`, `data-param="root"`,
		`data-api="/api/v1/graph/tree"`, `data-param="division"`,
		`data-layout="breadthfirst"`, `data-layout="concentric"`,
		`<option value="mn-div">`, `<option value="dhṛ">`)
	wantHTML(t, getPage(t, h, "/graph?dataset=root&param=dh%E1%B9%9B"), http.StatusOK,
		`data-list="graph-options-root" selected>`, `value="dhṛ"`)
}

func TestPageAnalytics(t *testing.T) {
	srv, _ := fixtureSidecarServer(t)
	h := srv.Routes()

	rec := wantHTML(t, getPage(t, h, "/analytics"), http.StatusOK, `value="top-headwords"`)
	if strings.Contains(rec.Body.String(), "console-form") {
		t.Fatal("console form must stay hidden while disabled")
	}

	wantHTML(t, getPage(t, h, "/analytics?aggregate=top-headwords"), http.StatusOK,
		"<th>headword</th>", "dhamma", "download csv",
		`href="/api/v1/analytics/top-headwords/csv?limit=50"`, "<svg")
	wantHTML(t, getPage(t, h, "/analytics?aggregate=coverage-by-division"), http.StatusOK,
		"mn-div")
	wantHTML(t, getPage(t, h,
		"/analytics?aggregate=top-headwords&division=mn-div"), http.StatusOK,
		`href="/api/v1/analytics/top-headwords/csv?division=mn-div&amp;limit=50"`)
}

func TestPageAnalyticsConsoleForm(t *testing.T) {
	srv, _ := fixtureSidecarServer(t)
	srv.cfg.Server.SQLConsole = true
	h := srv.Routes()

	wantHTML(t, getPage(t, h, "/analytics"), http.StatusOK, "console-form")

	form := url.Values{"console": {"1"}, "sql": {"SELECT 41 AS x"}}
	wantHTML(t, postForm(t, h, "/analytics", form), http.StatusOK, "<th>x</th>", "41")

	form.Set("sql", "DELETE FROM node")
	wantHTML(t, postForm(t, h, "/analytics", form), http.StatusOK, "SELECT or WITH")

	srv.cfg.Server.SQLConsole = false
	rec := postForm(t, h, "/analytics", form)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/analytics" {
		t.Fatalf("disabled console post = %d %s", rec.Code, rec.Header().Get("Location"))
	}
}

func postForm(t *testing.T, h http.Handler, target string, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, target, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestPagePipeline(t *testing.T) {
	srv, _ := newFixtureServer(t)
	release := make(chan struct{})
	withFakeExec(t, srv, func(_ context.Context, out io.Writer) error {
		<-release
		_, _ = io.WriteString(out, "stage output line\n")
		return nil
	})
	h := srv.Routes()

	rec := wantHTML(t, getPage(t, h, "/pipeline"), http.StatusOK,
		`name="stage" value="sync"`, `value="triangulate"`, `value="all"`,
		"hx-post=\"/pipeline\"")
	if n := strings.Count(rec.Body.String(), "stage-button"); n != 8 {
		t.Fatalf("stage buttons = %d, want 8", n)
	}

	form := url.Values{"stage": {"export"}}
	resp := postForm(t, h, "/pipeline", form)
	if resp.Code != http.StatusSeeOther || resp.Header().Get("Location") != "/pipeline" {
		t.Fatalf("stage post = %d %s", resp.Code, resp.Header().Get("Location"))
	}

	resp = postForm(t, h, "/pipeline", url.Values{"stage": {"sync"}})
	if resp.Code != http.StatusSeeOther {
		t.Fatalf("busy post = %d: %s", resp.Code, resp.Body.String())
	}

	req := httptest.NewRequest(http.MethodPost, "/pipeline", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, req)
	wantHTML(t, rec2, http.StatusOK, `id="job-status"`, `data-state="`)

	close(release)
	waitJobState(t, h, 1, jobs.StateDone)

	wantHTML(t, getPage(t, h, "/pipeline"), http.StatusOK,
		`id="job-status"`, `id="job-log"`,
		`data-log-url="/api/v1/jobs/1/logs"`, `data-next-seq="2"`,
		`data-polling="false"`, `data-seq="1"`, "stage output line")

	wantHTML(t, getPage(t, h, "/partials/job/1"), http.StatusOK, `id="job-status"`)
	wantPlain(t, getPage(t, h, "/partials/job/99"), http.StatusNotFound, "no such job")

	if rec := postForm(t, h, "/pipeline", url.Values{"stage": {"nope"}}); rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid stage post = %d", rec.Code)
	}
}

func TestPageDiscrepancies(t *testing.T) {
	srv, _ := newFixtureServer(t)
	h := srv.Routes()

	wantHTML(t, getPage(t, h, "/discrepancies"), http.StatusOK,
		"badge-error", `href="/texts/mn2"`, "duplicate key", "showing 1 to 2 of 2")

	wantHTML(t, getPage(t, h, "/discrepancies?limit=1"), http.StatusOK,
		`href="/discrepancies?limit=1&amp;offset=1"`, "next")
	rec := wantHTML(t, getPage(t, h, "/discrepancies?severity=error"), http.StatusOK,
		"mn2", "badge-error")
	if strings.Contains(rec.Body.String(), "mn1") {
		t.Fatalf("severity filter leaked rows:\n%s", rec.Body)
	}
	wantHTML(t, getPage(t, h, "/discrepancies?run=99"), http.StatusOK, "no discrepancies")
	wantPlain(t, getPage(t, h, "/discrepancies?run=x"), http.StatusBadRequest, "run")
}

func TestPageNotFound(t *testing.T) {
	srv, _ := newFixtureServer(t)
	h := srv.Routes()

	wantHTML(t, getPage(t, h, "/nope"), http.StatusNotFound, "not found", "/nope")
}

func TestPagesBeforePublish(t *testing.T) {
	cfg := newFixtureConfig(t)
	srv, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = srv.Close() })
	h := srv.Routes()

	wantHTML(t, getPage(t, h, "/"), http.StatusOK, "no stats")
	wantHTML(t, getPage(t, h, "/texts"), http.StatusOK, "no divisions")
	wantHTML(t, getPage(t, h, "/search"), http.StatusOK, "no results")
	wantHTML(t, getPage(t, h, "/discrepancies"), http.StatusOK, "no discrepancies")
	wantHTML(t, getPage(t, h, "/graph"), http.StatusOK, "graph-canvas")
	wantHTML(t, getPage(t, h, "/pipeline"), http.StatusOK, "stage-button")

	for _, target := range []string{"/texts/mn1", "/texts/branch?node=mn", "/words?form=x"} {
		if rec := getPage(t, h, target); rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("%s before publish = %d", target, rec.Code)
		}
	}
}
