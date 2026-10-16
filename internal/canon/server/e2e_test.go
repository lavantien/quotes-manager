package server

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/lavantien/quotes-manager/internal/canon/jobs"
	"github.com/lavantien/quotes-manager/internal/canon/source/dpd"
	"github.com/lavantien/quotes-manager/internal/cli"
	"github.com/lavantien/quotes-manager/internal/config"
)

const e2eLiveEnv = "QM_E2E_LIVE"

const e2eJobTimeout = 5 * time.Minute

func e2eWriteFiles(t *testing.T, path string, files map[string]string) {
	t.Helper()
	for rel, body := range files {
		p := filepath.Join(path, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func e2eGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{
		"-c", "user.email=e2e@quotes-manager", "-c", "user.name=e2e",
	}, args...)...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s in %s: %v: %s", strings.Join(args, " "), dir, err, out)
	}
}

func e2eFileURL(path string) string {
	return "file:///" + strings.TrimPrefix(filepath.ToSlash(path), "/")
}

func buildE2EBilara(t *testing.T, path string) {
	t.Helper()
	e2eWriteFiles(t, path, map[string]string{
		"root/pli/ms/sutta/mn/mn1_root-pli-ms.json": `{
	"mn1:0.1": "Middle Discourses 1",
	"mn1:1.1": "Evaṁ me sutaṁ.",
	"mn1:2.1": "Deep indeed."
}`,
		"root/pli/ms/sutta/mn/mn2_root-pli-ms.json": `{
	"mn2:1.1": "All the defilements."
}`,
		"translation/en/sujato/sutta/mn/mn1_translation-en-sujato.json": `{
	"mn1:0.1": "Middle Discourses 1",
	"mn1:1.1": "So I have heard.",
	"mn1:2.1": "Deep indeed."
}`,
		"html/pli/ms/sutta/mn/mn1_html.json": `{
	"mn1:0.1": "<p>",
	"mn1:1.1": "<p>"
}`,
		"reference/pli/ms/sutta/mn/mn1_reference.json": `{
	"mn1:0.1": "MN i 1",
	"mn1:1.1": "MN i 2",
	"mn1:2.1": "MN i 3"
}`,
	})
	e2eGit(t, path, "init", "-b", "main")
	e2eGit(t, path, "add", "-A")
	e2eGit(t, path, "commit", "-m", "chore: synthetic bilara corpus")
}

func buildE2EEditions(t *testing.T, path string) {
	t.Helper()
	zipPath := filepath.Join(path, "en/sujato/mn/paperback/Test-sujato-2026-01-02-tex.zip")
	if err := os.MkdirAll(filepath.Dir(zipPath), 0o755); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	w, err := zw.Create("vol-1.tex")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte(`\chapter*{Preface}
Front matter.
\section*{{\suttatitleacronym MN 1}{\suttatitletranslation Title}{\suttatitleroot Root}}
\addcontentsline{toc}{section}{MN 1}
\scevam{Middle Discourses\marginnote{0.1} 1 So\marginnote{1.1} I have heard.}
Deep indeed.
\section*{{\suttatitleacronym MN 2}{\suttatitletranslation T2}{\suttatitleroot R2}}
All the defilements.
`)); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	e2eWriteFiles(t, path, map[string]string{
		"en/sujato/mn/html/Middle-Discourses-sujato-2026-01-02.html": `<!doctype html>
<html><body>
<article id="mn1">
<h3 class="sutta-title heading"><span class="sutta-heading acronym">MN 1</span></h3>
<p id="mn1:0.1">Middle Discourses 1</p>
<p id="mn1:1.1">So I have heard.<a epub:type="noteref" href="#note-1" id="noteref-1" role="doc-noteref">1</a></p>
<p id="mn1:2.1">Deep indeed.</p>
</article>
<article id="mn2">
<p id="mn2:1.1">All the defilements.</p>
</article>
</body></html>`,
		"last_run_date": "2026-01-02",
	})
	e2eGit(t, path, "init", "-b", "main")
	e2eGit(t, path, "add", "-A")
	e2eGit(t, path, "commit", "-m", "chore: synthetic editions corpus")
}

func writeE2EConfig(t *testing.T, tmp string) string {
	t.Helper()
	_, dpdTar, err := dpd.WriteFixture(filepath.Join(tmp, "dpd-fixture"))
	if err != nil {
		t.Fatalf("dpd fixture: %v", err)
	}
	work := filepath.Join(tmp, "work")
	cfg := map[string]any{
		"server": map[string]any{"sql_console": true},
		"canon": map[string]any{
			"repos": map[string]any{
				"bilara": map[string]any{
					"url": e2eFileURL(filepath.Join(tmp, "bilara-src")), "branch": "main", "dir": "bilara-data",
				},
				"editions": map[string]any{
					"url": e2eFileURL(filepath.Join(tmp, "editions-src")), "branch": "main", "dir": "editions",
				},
			},
			"paths": map[string]any{
				"data_dir":      filepath.ToSlash(work),
				"db_dir":        filepath.ToSlash(filepath.Join(work, "db")),
				"dist_dir":      filepath.ToSlash(filepath.Join(work, "dist")),
				"manifests_dir": filepath.ToSlash(filepath.Join(work, "docs", "manifests")),
				"reports_dir":   filepath.ToSlash(filepath.Join(work, "docs", "reports")),
			},
			"dpd": map[string]any{"url": e2eFileURL(dpdTar)},
			"e2e": map[string]any{"muids": []string{}},
		},
	}
	raw, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(tmp, "config.json")
	if err := os.WriteFile(p, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func e2eStartJob(t *testing.T, h http.Handler, stage string) jobs.JobSnapshot {
	t.Helper()
	rec := postBody(t, h, "/api/v1/jobs", `{"stage":"`+stage+`"}`)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("POST /api/v1/jobs %s = %d: %s", stage, rec.Code, rec.Body.String())
	}
	var snap jobs.JobSnapshot
	if err := json.Unmarshal(rec.Body.Bytes(), &snap); err != nil {
		t.Fatalf("start %s body: %v (%s)", stage, err, rec.Body.String())
	}
	return snap
}

func e2ePollJob(t *testing.T, h http.Handler, id int64, stage string, timeout time.Duration) jobs.JobSnapshot {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		var snap jobs.JobSnapshot
		getJSON(t, h, fmt.Sprintf("/api/v1/jobs/%d", id), &snap)
		if !jobActive(snap.State) {
			return snap
		}
		if time.Now().After(deadline) {
			t.Fatalf("job %d (%s) stuck in %s after %s", id, stage, snap.State, timeout)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func e2eWaitJob(t *testing.T, h http.Handler, id int64, stage, tolerate string) jobs.JobSnapshot {
	t.Helper()
	snap := e2ePollJob(t, h, id, stage, e2eJobTimeout)
	if snap.State == jobs.StateDone {
		return snap
	}
	if snap.State == jobs.StateError && tolerate != "" && strings.Contains(snap.Error, tolerate) {
		return snap
	}
	t.Fatalf("job %d (%s) reached %s: %s", id, stage, snap.State, snap.Error)
	return jobs.JobSnapshot{}
}

func e2eLatestJob(t *testing.T, h http.Handler) jobs.JobSnapshot {
	t.Helper()
	var list []jobs.JobSnapshot
	getJSON(t, h, "/api/v1/jobs", &list)
	if len(list) == 0 {
		t.Fatal("no jobs recorded")
	}
	return list[len(list)-1]
}

type e2ePair struct {
	Muid   string `json:"muid"`
	SegKey string `json:"seg_key"`
	Pali   string `json:"pali"`
	En     string `json:"en"`
	Author string `json:"author"`
}

type e2eGraph struct {
	Elements struct {
		Nodes []struct {
			Data struct {
				ID    string `json:"id"`
				Label string `json:"label"`
				Type  string `json:"type"`
			} `json:"data"`
		} `json:"nodes"`
		Edges []struct {
			Data struct {
				Source string `json:"source"`
				Target string `json:"target"`
				Type   string `json:"type"`
			} `json:"data"`
		} `json:"edges"`
	} `json:"elements"`
	Meta struct {
		Nodes int `json:"nodes"`
		Edges int `json:"edges"`
		Caps  struct {
			MaxNodes int `json:"max_nodes"`
			MaxEdges int `json:"max_edges"`
		} `json:"caps"`
	} `json:"meta"`
}

func e2eGetGraph(t *testing.T, h http.Handler, target string) e2eGraph {
	t.Helper()
	var g e2eGraph
	getJSON(t, h, target, &g)
	return g
}

func TestE2EServerHermetic(t *testing.T) {
	tmp := t.TempDir()
	buildE2EBilara(t, filepath.Join(tmp, "bilara-src"))
	buildE2EEditions(t, filepath.Join(tmp, "editions-src"))
	cfg, err := config.Load(writeE2EConfig(t, tmp))
	if err != nil {
		t.Fatal(err)
	}
	srv, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = srv.Close() })
	h := srv.Routes()

	syncJob := e2eStartJob(t, h, "sync")
	rec := postBody(t, h, "/api/v1/jobs", `{"stage":"sync"}`)
	if rec.Code != http.StatusConflict {
		t.Fatalf("busy probe = %d: %s", rec.Code, rec.Body.String())
	}
	var env errorEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil || env.Error.Code != codeJobBusy {
		t.Fatalf("busy probe body = %s (%v)", rec.Body.String(), err)
	}
	e2eWaitJob(t, h, syncJob.ID, "sync", "")

	for _, stage := range []string{"parse", "load", "triangulate", "export"} {
		j := e2eStartJob(t, h, stage)
		e2eWaitJob(t, h, j.ID, stage, "")
	}

	var logLines []struct {
		Seq  int64  `json:"seq"`
		At   string `json:"at"`
		Text string `json:"text"`
	}
	getJSON(t, h, fmt.Sprintf("/api/v1/jobs/%d/logs", syncJob.ID), &logLines)
	if len(logLines) == 0 {
		t.Fatal("sync job captured no log lines")
	}
	for i, line := range logLines {
		if line.Seq != int64(i+1) {
			t.Fatalf("log seq at %d = %d, want %d", i, line.Seq, i+1)
		}
	}

	var ver struct {
		Version string `json:"version"`
		Schema  int    `json:"schema"`
		Run     string `json:"run"`
	}
	getJSON(t, h, "/api/v1/version", &ver)
	if ver.Version != config.Version || ver.Schema != config.SchemaVersion {
		t.Fatalf("version = %s/%d", ver.Version, ver.Schema)
	}
	runA := ver.Run
	if runA == "" {
		t.Fatal("run id empty after load")
	}

	stats := map[string]int64{}
	getJSON(t, h, "/api/v1/stats", &stats)
	for _, k := range []string{"node:segment", "node:headword", "edge:HAS_WORD"} {
		if stats[k] <= 0 {
			t.Errorf("stats[%s] = %d, want > 0", k, stats[k])
		}
	}

	var pairs []e2ePair
	getJSON(t, h, "/api/v1/segments?muid=mn1&with_en=true", &pairs)
	aligned := false
	for _, p := range pairs {
		if p.SegKey == "1.1" && p.Pali == "Evaṁ me sutaṁ." && p.En == "So I have heard." && p.Author == "sujato" {
			aligned = true
		}
	}
	if !aligned {
		t.Errorf("mn1 aligned pair 1.1 missing: %+v", pairs)
	}

	getJSON(t, h, "/api/v1/search?text="+url.QueryEscape("So I have heard")+"&lang=en", &pairs)
	found := false
	for _, p := range pairs {
		if p.Muid == "mn1" && p.SegKey == "1.1" && p.En == "So I have heard." {
			found = true
		}
	}
	if !found {
		t.Errorf("en search missed mn1:1.1: %+v", pairs)
	}

	var entries []struct {
		Lemma1 string `json:"lemma_1"`
	}
	getJSON(t, h, "/api/v1/word/form?token=evam", &entries)
	if len(entries) == 0 || entries[0].Lemma1 != "eva" {
		t.Errorf("word form evam = %+v", entries)
	}

	word := e2eGetGraph(t, h, "/api/v1/graph/word?token=dhamma")
	nodeTypes := map[string]bool{}
	for _, n := range word.Elements.Nodes {
		nodeTypes[n.Data.Type] = true
	}
	for _, typ := range []string{"dpd_form", "headword", "dpd_root"} {
		if !nodeTypes[typ] {
			t.Errorf("graph word missing %s node: %+v", typ, word.Elements.Nodes)
		}
	}
	edgeTypes := map[string]bool{}
	for _, e := range word.Elements.Edges {
		edgeTypes[e.Data.Type] = true
	}
	if !edgeTypes["HAS_WORD"] || !edgeTypes["DERIVED_FROM"] {
		t.Errorf("graph word edges = %+v", word.Elements.Edges)
	}
	if word.Meta.Nodes != len(word.Elements.Nodes) ||
		word.Meta.Caps.MaxNodes != cfg.Server.GraphMaxNodes ||
		word.Meta.Caps.MaxEdges != cfg.Server.GraphMaxEdges {
		t.Errorf("graph word meta = %+v", word.Meta)
	}

	tree := e2eGetGraph(t, h, "/api/v1/graph/tree?division=mn&types=text")
	treeIDs := map[string]bool{}
	for _, n := range tree.Elements.Nodes {
		treeIDs[n.Data.ID] = true
	}
	if !treeIDs["mn"] || !treeIDs["mn1"] || !treeIDs["mn2"] {
		t.Errorf("graph tree nodes = %+v", tree.Elements.Nodes)
	}

	var disc struct {
		Rows []struct {
			Check    string `json:"check"`
			Severity string `json:"severity"`
		} `json:"rows"`
		Total int64 `json:"total"`
	}
	getJSON(t, h, "/api/v1/discrepancies", &disc)
	if disc.Total == 0 || len(disc.Rows) == 0 || int64(len(disc.Rows)) > disc.Total {
		t.Errorf("discrepancies = %d rows of total %d", len(disc.Rows), disc.Total)
	}

	var export struct {
		Tables []struct {
			Name    string `json:"name"`
			Rows    int64  `json:"rows"`
			Parquet bool   `json:"parquet"`
		} `json:"tables"`
	}
	getJSON(t, h, "/api/v1/export", &export)
	if len(export.Tables) != 8 {
		t.Fatalf("export tables = %d, want 8: %+v", len(export.Tables), export.Tables)
	}
	dpdRows := int64(0)
	for _, tbl := range export.Tables {
		if !tbl.Parquet {
			t.Errorf("table %s has no parquet file", tbl.Name)
		}
		if tbl.Name == "dpd_entries" {
			dpdRows = tbl.Rows
		}
	}
	if dpdRows == 0 {
		t.Errorf("dpd_entries rows = 0, want the fixture headwords")
	}

	prec := getPage(t, h, "/api/v1/export/parquet/dpd_entries")
	if prec.Code != http.StatusOK {
		t.Fatalf("parquet download = %d: %s", prec.Code, prec.Body.String())
	}
	if cd := prec.Header().Get("Content-Disposition"); !strings.Contains(cd, "dpd_entries.parquet") {
		t.Errorf("parquet disposition = %q", cd)
	}
	if !bytes.HasPrefix(prec.Body.Bytes(), []byte("PAR1")) {
		t.Errorf("parquet magic missing: %q", prec.Body.String())
	}

	var cov struct {
		Cols []string `json:"cols"`
		Rows [][]any  `json:"rows"`
	}
	getJSON(t, h, "/api/v1/analytics/coverage-by-division", &cov)
	if len(cov.Cols) == 0 || len(cov.Rows) == 0 {
		t.Errorf("coverage-by-division = %+v", cov)
	}

	qrec := postBody(t, h, "/api/v1/analytics/query", `{"sql":"SELECT 42 AS answer","limit":5}`)
	if qrec.Code != http.StatusOK {
		t.Fatalf("console query = %d: %s", qrec.Code, qrec.Body.String())
	}
	var answer struct {
		Cols []string `json:"cols"`
		Rows [][]any  `json:"rows"`
	}
	if err := json.Unmarshal(qrec.Body.Bytes(), &answer); err != nil {
		t.Fatalf("console query body: %v (%s)", err, qrec.Body.String())
	}
	if len(answer.Cols) != 1 || answer.Cols[0] != "answer" || len(answer.Rows) != 1 ||
		len(answer.Rows[0]) != 1 || answer.Rows[0][0] != float64(42) {
		t.Errorf("console query = %+v", answer)
	}
	wantHTML(t, postForm(t, h, "/analytics", url.Values{
		"console": {"1"}, "sql": {"SELECT 42 AS answer"},
	}), http.StatusOK, "<td>42</td>")

	formRec := postForm(t, h, "/pipeline", url.Values{"stage": {"load"}})
	if formRec.Code != http.StatusSeeOther || formRec.Header().Get("Location") != "/pipeline" {
		t.Fatalf("pipeline form = %d/%q: %s", formRec.Code, formRec.Header().Get("Location"), formRec.Body.String())
	}
	load2 := e2eLatestJob(t, h)
	if load2.Stage != "load" {
		t.Fatalf("latest job after form post = %s", load2.Stage)
	}
	e2eWaitJob(t, h, load2.ID, "load", "retiring the previous database failed")
	getJSON(t, h, "/api/v1/version", &ver)
	if ver.Run == "" || ver.Run == runA {
		t.Fatalf("run after reload = %q, want a different id than %q", ver.Run, runA)
	}
	stats = map[string]int64{}
	getJSON(t, h, "/api/v1/stats", &stats)
	if stats["node:segment"] <= 0 {
		t.Errorf("stats after reload: node:segment = %d", stats["node:segment"])
	}

	hxReq := httptest.NewRequest(http.MethodPost, "/pipeline", strings.NewReader("stage=report"))
	hxReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	hxReq.Header.Set("HX-Request", "true")
	hxRec := httptest.NewRecorder()
	h.ServeHTTP(hxRec, hxReq)
	wantHTML(t, hxRec, http.StatusOK, `id="job-status"`, "report")
	reportJob := e2eLatestJob(t, h)
	if reportJob.Stage != "report" {
		t.Fatalf("latest job after hx post = %s", reportJob.Stage)
	}
	e2eWaitJob(t, h, reportJob.ID, "report", "")

	var reports struct {
		Reports []struct {
			Date  string   `json:"date"`
			Files []string `json:"files"`
		} `json:"reports"`
	}
	getJSON(t, h, "/api/v1/report", &reports)
	if len(reports.Reports) == 0 || len(reports.Reports[0].Files) == 0 {
		t.Fatalf("report listing empty after the report job: %+v", reports)
	}
	md := getPage(t, h, "/api/v1/report/"+reports.Reports[0].Date+"/report.md")
	if md.Code != http.StatusOK || md.Body.Len() == 0 {
		t.Fatalf("report download = %d: %s", md.Code, md.Body.String())
	}
	if ct := md.Header().Get("Content-Type"); ct != "text/markdown; charset=utf-8" {
		t.Errorf("report download content type = %q", ct)
	}

	customOut := filepath.Join(cfg.Canon.Paths.ReportsDir, "custom")
	rec = postBody(t, h, "/api/v1/jobs",
		fmt.Sprintf(`{"stage":"report","args":["--out",%q]}`, filepath.ToSlash(customOut)))
	if rec.Code != http.StatusAccepted {
		t.Fatalf("report with args = %d: %s", rec.Code, rec.Body.String())
	}
	argsJob := e2eLatestJob(t, h)
	e2eWaitJob(t, h, argsJob.ID, "report", "")
	if _, err := os.Stat(filepath.Join(customOut, "report.md")); err != nil {
		t.Errorf("job args did not reach the stage flag parser: %v", err)
	}

	escRec := postBody(t, h, "/api/v1/jobs",
		fmt.Sprintf(`{"stage":"report","args":["--out",%q]}`, filepath.ToSlash(filepath.Join(tmp, "escape"))))
	if escRec.Code != http.StatusBadRequest {
		t.Fatalf("escaping --out = %d: %s", escRec.Code, escRec.Body.String())
	}

	badRec := postBody(t, h, "/api/v1/jobs", `{"stage":"prune","args":["--nope"]}`)
	if badRec.Code != http.StatusAccepted {
		t.Fatalf("bad-args job = %d: %s", badRec.Code, badRec.Body.String())
	}
	badJob := e2eLatestJob(t, h)
	if snap := e2ePollJob(t, h, badJob.ID, "prune", e2eJobTimeout); snap.State != jobs.StateError {
		t.Errorf("bad-args job reached %s: %s", snap.State, snap.Error)
	}

	getJSON(t, h, "/api/v1/segments?muid=mn1&limit=1&offset=1", &pairs)
	if len(pairs) != 1 || pairs[0].SegKey != "1.1" {
		t.Errorf("segments window limit 1 offset 1 = %+v, want exactly 1.1", pairs)
	}

	wantHTML(t, getPage(t, h, "/dashboard"), http.StatusOK, "dashboard", "node:segment")
	wantHTML(t, getPage(t, h, "/texts"), http.StatusOK, "texts")
	wantHTML(t, getPage(t, h, "/search?text="+url.QueryEscape("heard")+"&lang=en"), http.StatusOK, "search")
	wantHTML(t, getPage(t, h, "/words?form=evam"), http.StatusOK, ">eva</a>", "thus, in this way")
	wantHTML(t, getPage(t, h, "/discrepancies"), http.StatusOK, "discrepancies")
	wantHTML(t, getPage(t, h, "/analytics"), http.StatusOK, "coverage-by-division", "sql console")
	wantHTML(t, getPage(t, h, fmt.Sprintf("/partials/job/%d", syncJob.ID)), http.StatusOK, `id="job-status"`)
	wantHTML(t, getPage(t, h, "/texts/mn1"), http.StatusOK, `/words?form=suta%E1%B9%81`)
	pipe := wantHTML(t, getPage(t, h, "/pipeline"), http.StatusOK, "pipeline")
	for _, stage := range cli.Stages() {
		if !strings.Contains(pipe.Body.String(), `name="stage" value="`+stage+`"`) {
			t.Errorf("pipeline page missing the %s form", stage)
		}
	}

	wantHTML(t, getPage(t, h, "/nope"), http.StatusNotFound, "not found")
	status, body := getError(t, h, "/api/v1/nope")
	if status != http.StatusNotFound || body.Code != codeNotFound {
		t.Errorf("unknown api route = %d/%s", status, body.Code)
	}
}

func e2eRepoRoot(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(thisFile))))
}

func TestServerLiveSmoke(t *testing.T) {
	if v := os.Getenv(e2eLiveEnv); v == "" || v == "0" {
		t.Skipf("live smoke disabled; set %s=1 to enable", e2eLiveEnv)
	}
	repoRoot := e2eRepoRoot(t)
	root := filepath.Join(repoRoot, "data", "e2e-live")
	cfg := config.Defaults()
	cfg.Canon.E2E.Muids = []string{"mn1", "mn2", "pli-tv-bu-pm"}
	cfg.Canon.Dpd.URL = e2eFileURL(filepath.Join(repoRoot, "testdata", "dpd", "dpd.db.tar.xz"))
	cfg.Canon.Paths.DataDir = filepath.ToSlash(root)
	cfg.Canon.Paths.DBDir = filepath.ToSlash(filepath.Join(root, "db"))
	cfg.Canon.Paths.DistDir = filepath.ToSlash(filepath.Join(root, "dist"))
	cfg.Canon.Paths.ManifestsDir = filepath.ToSlash(filepath.Join(root, "docs", "manifests"))
	cfg.Canon.Paths.ReportsDir = filepath.ToSlash(filepath.Join(root, "docs", "reports"))
	srv, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = srv.Close() })
	h := srv.Routes()

	if _, err := os.Stat(cfg.Canon.DBPointerFile()); err != nil {
		syncJob := e2eStartJob(t, h, "sync")
		snap := e2ePollJob(t, h, syncJob.ID, "sync", 20*time.Minute)
		if snap.State != jobs.StateDone {
			t.Skipf("live smoke: network unavailable (%s: %s), skipping", snap.State, snap.Error)
		}
		for _, stage := range []string{"parse", "load"} {
			j := e2eStartJob(t, h, stage)
			e2eWaitJobLive(t, h, j.ID, stage)
		}
	}

	var pairs []e2ePair
	getJSON(t, h, "/api/v1/search?text="+url.QueryEscape("I have heard")+"&lang=en", &pairs)
	if len(pairs) == 0 || pairs[0].Muid != "mn1" || pairs[0].En == "" {
		t.Errorf("live en search = %+v", pairs[:min(len(pairs), 3)])
	}

	word := e2eGetGraph(t, h, "/api/v1/graph/word?token=dhamma")
	if len(word.Elements.Nodes) == 0 || len(word.Elements.Edges) == 0 {
		t.Errorf("live graph word = %+v", word)
	}

	wantHTML(t, getPage(t, h, "/dashboard"), http.StatusOK, "dashboard")
}

func e2eWaitJobLive(t *testing.T, h http.Handler, id int64, stage string) {
	t.Helper()
	snap := e2ePollJob(t, h, id, stage, 20*time.Minute)
	if snap.State != jobs.StateDone {
		t.Fatalf("live job %d (%s) reached %s: %s", id, stage, snap.State, snap.Error)
	}
}
