package server

import (
	"net/http"
	"net/url"
	"path/filepath"
	"testing"

	"github.com/lavantien/quotes-manager/internal/config"
)

func TestAPIVersion(t *testing.T) {
	srv, _ := newFixtureServer(t)
	var got versionJSON
	getJSON(t, srv.Routes(), "/api/v1/version", &got)
	if got.Version == "" || got.Schema == 0 || got.Run != "run1" {
		t.Fatalf("version = %+v", got)
	}
}

func TestAPIVersionBeforePublishHasEmptyRun(t *testing.T) {
	_, cfg := newFixtureServer(t)
	cfg2 := *cfg
	cfg2.Canon.Paths.DataDir = t.TempDir()
	cfg2.Canon.Paths.DBDir = filepath.Join(cfg2.Canon.Paths.DataDir, "db")
	fresh, err := New(&cfg2)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = fresh.Close() }()
	var got versionJSON
	getJSON(t, fresh.Routes(), "/api/v1/version", &got)
	if got.Run != "" || got.Version == "" || got.Schema == 0 {
		t.Fatalf("version before publish = %+v, want empty run", got)
	}
}

func TestAPIStats(t *testing.T) {
	srv, _ := newFixtureServer(t)
	var got map[string]int64
	getJSON(t, srv.Routes(), "/api/v1/stats", &got)
	for k, want := range map[string]int64{
		"node:text":         2,
		"node:headword":     3,
		"node:dpd_root":     1,
		"node:edition":      1,
		"edge:HAS_WORD":     5,
		"edge:DERIVED_FROM": 2,
		"edge:COVERED_BY":   1,
	} {
		if got[k] != want {
			t.Fatalf("stats[%s] = %d, want %d (all %v)", k, got[k], want, got)
		}
	}
}

func TestAPISegments(t *testing.T) {
	srv, _ := newFixtureServer(t)
	h := srv.Routes()

	var got []segmentJSON
	getJSON(t, h, "/api/v1/segments?muid=mn1", &got)
	want := []segmentJSON{
		{Muid: "mn1", SegKey: "0.1", Pali: "pariyāya", En: "the relational", Author: "sujato"},
		{Muid: "mn1", SegKey: "0.2", Pali: "ekodhāna", En: "settled", Author: "sujato"},
		{Muid: "mn1", SegKey: "0.3", Pali: "āsavā"},
	}
	if len(got) != len(want) {
		t.Fatalf("segments = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("segments[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}

	getJSON(t, h, "/api/v1/segments?muid=mn1&with_en=1", &got)
	if len(got) != 2 {
		t.Fatalf("with_en segments = %+v, want 2 rows", got)
	}
	getJSON(t, h, "/api/v1/segments?muid=mn1&limit=1", &got)
	if len(got) != 1 || got[0].SegKey != "0.1" {
		t.Fatalf("limited segments = %+v", got)
	}
	getJSON(t, h, "/api/v1/segments?muid=mn1&limit=1&offset=0", &got)
	if len(got) != 1 || got[0].SegKey != "0.1" {
		t.Fatalf("offset 0 segments = %+v", got)
	}
	getJSON(t, h, "/api/v1/segments?muid=mn1&limit=1&offset=1", &got)
	if len(got) != 1 || got[0].SegKey != "0.2" {
		t.Fatalf("offset window segments = %+v, want only 0.2", got)
	}
	getJSON(t, h, "/api/v1/segments?muid=mn1&offset=3", &got)
	if got == nil || len(got) != 0 {
		t.Fatalf("offset past end segments = %+v, want empty non-nil", got)
	}
	getJSON(t, h, "/api/v1/segments?muid=mn999", &got)
	if got == nil || len(got) != 0 {
		t.Fatalf("unknown muid segments = %+v, want empty non-nil", got)
	}

	status, body := getError(t, h, "/api/v1/segments")
	if status != http.StatusBadRequest || body.Code != codeInvalidParam {
		t.Fatalf("missing muid = %d/%s", status, body.Code)
	}
	status, body = getError(t, h, "/api/v1/segments?muid=mn1&limit=abc")
	if status != http.StatusBadRequest || body.Code != codeInvalidParam {
		t.Fatalf("bad limit = %d/%s", status, body.Code)
	}
	for _, target := range []string{
		"/api/v1/segments?muid=mn1&offset=-1",
		"/api/v1/segments?muid=mn1&offset=abc",
	} {
		status, body = getError(t, h, target)
		if status != http.StatusBadRequest || body.Code != codeInvalidParam {
			t.Fatalf("segments %s = %d/%s, want 400 invalid_param", target, status, body.Code)
		}
	}
}

func TestAPISearch(t *testing.T) {
	srv, _ := newFixtureServer(t)
	h := srv.Routes()

	var got []segmentJSON
	getJSON(t, h, "/api/v1/search?text=pariyaya&lang=pali", &got)
	if len(got) != 1 || got[0].Muid != "mn1" || got[0].SegKey != "0.1" || got[0].En != "the relational" {
		t.Fatalf("pali search = %+v", got)
	}
	getJSON(t, h, "/api/v1/search?text=taints&lang=en", &got)
	if len(got) != 1 || got[0].Muid != "mn2" || got[0].En != "all the taints" {
		t.Fatalf("en search = %+v", got)
	}
	getJSON(t, h, "/api/v1/search?text=ekodhana", &got)
	if len(got) != 1 || got[0].SegKey != "0.2" {
		t.Fatalf("default-lang search = %+v", got)
	}
	getJSON(t, h, `/api/v1/search?text=`+url.QueryEscape(`said "hi"`)+"&lang=en", &got)
	if len(got) != 0 {
		t.Fatalf("quoted search = %+v, want none", got)
	}

	getJSON(t, h, "/api/v1/search?text=the&lang=en&limit=1&offset=1", &got)
	if len(got) != 1 || got[0].Muid != "mn2" || got[0].En != "all the taints" {
		t.Fatalf("search offset window = %+v, want the mn2 row", got)
	}

	for _, target := range []string{
		"/api/v1/search",
		"/api/v1/search?text=",
		"/api/v1/search?text=%20%20",
		"/api/v1/search?text=x&lang=fr",
		"/api/v1/search?text=x&offset=-1",
		"/api/v1/search?text=x&offset=abc",
	} {
		status, body := getError(t, h, target)
		if status != http.StatusBadRequest || body.Code != codeInvalidParam {
			t.Fatalf("search %s = %d/%s, want 400 invalid_param", target, status, body.Code)
		}
	}
}

func TestAPITree(t *testing.T) {
	srv, _ := newFixtureServer(t)
	h := srv.Routes()

	var got []treeNodeJSON
	getJSON(t, h, "/api/v1/tree?division=mn-div", &got)
	if len(got) != 10 {
		t.Fatalf("tree all = %d nodes:\n%+v", len(got), got)
	}
	if got[0].Key != "mn-div" || got[0].Type != "division" || got[0].Depth != 0 {
		t.Fatalf("tree root = %+v", got[0])
	}
	if _, ok := got[0].Data.(map[string]any); !ok {
		t.Fatalf("tree data = %#v, want decoded object", got[0].Data)
	}

	getJSON(t, h, "/api/v1/tree?division=mn-div&types=text", &got)
	if len(got) != 2 || got[0].Key != "mn1" || got[1].Key != "mn2" {
		t.Fatalf("tree texts = %+v", got)
	}

	getJSON(t, h, "/api/v1/tree?division=mn-div&types=text&limit=1&offset=1", &got)
	if len(got) != 1 || got[0].Key != "mn2" {
		t.Fatalf("tree offset window = %+v, want only mn2", got)
	}
	getJSON(t, h, "/api/v1/tree?division=mn-div&types=text&offset=2", &got)
	if got == nil || len(got) != 0 {
		t.Fatalf("tree offset past end = %+v, want empty non-nil", got)
	}

	status, body := getError(t, h, "/api/v1/tree")
	if status != http.StatusBadRequest || body.Code != codeInvalidParam {
		t.Fatalf("missing division = %d/%s", status, body.Code)
	}
	status, body = getError(t, h, "/api/v1/tree?division=mn-div&types=bogus")
	if status != http.StatusBadRequest || body.Code != codeInvalidParam {
		t.Fatalf("bad types = %d/%s", status, body.Code)
	}
	status, body = getError(t, h, "/api/v1/tree?division=mn-div&offset=abc")
	if status != http.StatusBadRequest || body.Code != codeInvalidParam {
		t.Fatalf("bad offset = %d/%s", status, body.Code)
	}
}

func TestAPIWordForm(t *testing.T) {
	srv, _ := newFixtureServer(t)
	h := srv.Routes()

	var got []map[string]any
	getJSON(t, h, "/api/v1/word/form?token="+url.QueryEscape("dhammā"), &got)
	if len(got) != 1 {
		t.Fatalf("form entries = %+v, want 1", got)
	}
	e := got[0]
	if e["lemma_1"] != "dhamma" || e["pos"] != "m" || e["root"] != "dhṛ" ||
		e["family_root"] != "dhṛ" || e["segments"].(float64) != 3 {
		t.Fatalf("form entry = %+v", e)
	}

	getJSON(t, h, "/api/v1/word/form?token=nope", &got)
	if got == nil || len(got) != 0 {
		t.Fatalf("unknown form = %+v, want empty non-nil", got)
	}

	status, body := getError(t, h, "/api/v1/word/form")
	if status != http.StatusBadRequest || body.Code != codeInvalidParam {
		t.Fatalf("missing token = %d/%s", status, body.Code)
	}
}

func TestAPIWordHeadword(t *testing.T) {
	srv, _ := newFixtureServer(t)
	h := srv.Routes()

	var got headwordUsageJSON
	getJSON(t, h, "/api/v1/word/headword?lemma=dhamma&segments", &got)
	if got.Headword != "dhamma" || got.Segments != 3 || len(got.Refs) != 3 {
		t.Fatalf("headword usage = %+v", got)
	}
	want := []string{"mn1:0.1", "mn1:0.2", "mn2:0.1"}
	for i, ref := range want {
		if got.Refs[i] != ref {
			t.Fatalf("refs[%d] = %s, want %s (%v)", i, got.Refs[i], ref, got.Refs)
		}
	}

	getJSON(t, h, "/api/v1/word/headword?lemma=dhamma", &got)
	if got.Segments != 3 || got.Refs == nil || len(got.Refs) != 0 {
		t.Fatalf("count-only usage = %+v, want no refs", got)
	}
	getJSON(t, h, "/api/v1/word/headword?lemma=dhamma&segments&limit=1&offset=1", &got)
	if got.Segments != 3 || len(got.Refs) != 1 || got.Refs[0] != "mn1:0.2" {
		t.Fatalf("headword offset window = %+v, want 3 total with the 0.2 ref", got)
	}
	getJSON(t, h, "/api/v1/word/headword?lemma=dhamma&segments&offset=3", &got)
	if got.Segments != 3 || got.Refs == nil || len(got.Refs) != 0 {
		t.Fatalf("headword offset past end = %+v, want 3 total with empty refs", got)
	}
	getJSON(t, h, "/api/v1/word/headword?lemma=nobody", &got)
	if got.Segments != 0 || got.Refs == nil {
		t.Fatalf("unknown lemma usage = %+v", got)
	}

	status, body := getError(t, h, "/api/v1/word/headword")
	if status != http.StatusBadRequest || body.Code != codeInvalidParam {
		t.Fatalf("missing lemma = %d/%s", status, body.Code)
	}
	status, body = getError(t, h, "/api/v1/word/headword?lemma=dhamma&offset=-1")
	if status != http.StatusBadRequest || body.Code != codeInvalidParam {
		t.Fatalf("negative offset = %d/%s", status, body.Code)
	}
}

func TestAPIRuns(t *testing.T) {
	srv, _ := newFixtureServer(t)
	var got []runJSON
	getJSON(t, srv.Routes(), "/api/v1/runs", &got)
	if len(got) != 1 {
		t.Fatalf("runs = %+v, want 1 row", got)
	}
	r := got[0]
	if r.ID != 1 || r.BilaraCommit == nil || *r.BilaraCommit != "bilara1" ||
		r.EditionsCommit == nil || *r.EditionsCommit != "editions1" ||
		r.DpdTag == nil || *r.DpdTag != "dpd-tag-1" ||
		r.Started == nil || r.Finished == nil {
		t.Fatalf("run row = %+v", r)
	}
}

func TestAPIDiscrepancies(t *testing.T) {
	srv, _ := newFixtureServer(t)
	h := srv.Routes()

	var got discrepanciesJSON
	getJSON(t, h, "/api/v1/discrepancies", &got)
	if got.Total != 2 || len(got.Rows) != 2 {
		t.Fatalf("discrepancies = %+v", got)
	}
	if got.Rows[0].ID != 1 || got.Rows[0].Check != "B005" || got.Rows[0].Severity != "info" ||
		got.Rows[0].Muid == nil || *got.Rows[0].Muid != "mn1" ||
		got.Rows[0].Detail != "empty root value at 0.3" {
		t.Fatalf("row 0 = %+v", got.Rows[0])
	}
	if got.Rows[1].Check != "B004" || got.Rows[1].Severity != "error" {
		t.Fatalf("row 1 = %+v", got.Rows[1])
	}

	getJSON(t, h, "/api/v1/discrepancies?severity=error", &got)
	if got.Total != 1 || len(got.Rows) != 1 || got.Rows[0].Check != "B004" {
		t.Fatalf("severity filter = %+v", got)
	}
	getJSON(t, h, "/api/v1/discrepancies?check=B005", &got)
	if got.Total != 1 || got.Rows[0].Check != "B005" {
		t.Fatalf("check filter = %+v", got)
	}

	getJSON(t, h, "/api/v1/discrepancies?limit=1", &got)
	if got.Total != 2 || len(got.Rows) != 1 || got.Rows[0].ID != 1 {
		t.Fatalf("limit 1 = %+v", got)
	}
	getJSON(t, h, "/api/v1/discrepancies?limit=1&offset=1", &got)
	if got.Total != 2 || len(got.Rows) != 1 || got.Rows[0].ID != 2 {
		t.Fatalf("offset 1 = %+v", got)
	}
	getJSON(t, h, "/api/v1/discrepancies?run=1", &got)
	if got.Total != 2 {
		t.Fatalf("explicit run = %+v", got)
	}

	status, body := getError(t, h, "/api/v1/discrepancies?run=999")
	if status != http.StatusNotFound || body.Code != codeRunNotFound {
		t.Fatalf("unknown run = %d/%s, want 404 run_not_found", status, body.Code)
	}
	for _, target := range []string{
		"/api/v1/discrepancies?run=abc",
		"/api/v1/discrepancies?severity=fatal",
		"/api/v1/discrepancies?offset=-1",
	} {
		status, body = getError(t, h, target)
		if status != http.StatusBadRequest || body.Code != codeInvalidParam {
			t.Fatalf("%s = %d/%s, want 400 invalid_param", target, status, body.Code)
		}
	}
}

func TestAPINotReady(t *testing.T) {
	cfg := newNotReadyConfig(t)
	srv, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = srv.Close() }()
	h := srv.Routes()
	for _, target := range []string{
		"/api/v1/stats",
		"/api/v1/segments?muid=mn1",
		"/api/v1/search?text=x",
		"/api/v1/tree?division=mn-div",
		"/api/v1/word/form?token=dhamma",
		"/api/v1/word/headword?lemma=dhamma",
		"/api/v1/runs",
		"/api/v1/discrepancies",
		"/api/v1/graph/word?token=dhamma",
		"/api/v1/graph/root?root=dhṛ",
		"/api/v1/graph/tree?division=mn-div",
		"/api/v1/graph/coverage?division=mn-div",
	} {
		status, body := getError(t, h, target)
		if status != http.StatusServiceUnavailable || body.Code != codeDBNotReady {
			t.Fatalf("%s = %d/%s, want 503 db_not_ready", target, status, body.Code)
		}
	}
}

func newNotReadyConfig(t *testing.T) *config.Config {
	t.Helper()
	return newFixtureConfig(t)
}
