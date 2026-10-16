package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lavantien/quotes-manager/internal/canon/graph"
	"github.com/lavantien/quotes-manager/internal/canon/model"
	"github.com/lavantien/quotes-manager/internal/config"
)

func hasNUL(s string) bool { return strings.ContainsRune(s, 0) }

func FuzzServerSearch(f *testing.F) {
	for _, c := range [][2]string{
		{"pariyaya", "pali"},
		{"pariyāya", "en"},
		{"", "pali"},
		{"x", "fr"},
		{"said \"hi\"", "en"},
		{"a OR b", "pali"},
		{"NEAR(x y)", "en"},
		{"*", ""},
		{"^start", "pali"},
		{"col:*", "en"},
		{"'", "pali"},
		{"'; DROP TABLE fts_segment;--", "en"},
		{"छन्दो", "pali"},
		{" ", "pali"},
	} {
		f.Add(c[0], c[1])
	}
	srv, _ := newFixtureServer(f)
	h := srv.Routes()
	f.Fuzz(func(t *testing.T, text, lang string) {
		if hasNUL(text) || hasNUL(lang) {
			return
		}
		target := "/api/v1/search?text=" + url.QueryEscape(text) + "&lang=" + url.QueryEscape(lang)
		req := httptest.NewRequest(http.MethodGet, target, nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		switch rec.Code {
		case http.StatusOK:
			if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
				t.Fatalf("search %q/%q content type = %q", text, lang, ct)
			}
		case http.StatusBadRequest:
			var env errorEnvelope
			if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil || env.Error.Code != codeInvalidParam {
				t.Fatalf("search %q/%q 400 body = %s", text, lang, rec.Body.String())
			}
		default:
			t.Fatalf("search %q/%q = %d: %s", text, lang, rec.Code, rec.Body.String())
		}
	})
}

func FuzzServerLimits(f *testing.F) {
	for _, c := range [][]string{
		{"", ""},
		{"10", "5"},
		{"0", "0"},
		{"-5", "-1"},
		{"99999", "1000000"},
		{"abc", "x"},
		{"1e3", " 7"},
		{"+3", "3"},
		{"0x10", "0b1"},
		{"9999999999999999999", "9999999999999999999"},
	} {
		f.Add(c[0], c[1])
	}
	srv, _ := newFixtureServer(f)
	max := srv.cfg.Server.MaxLimit
	h := srv.Routes()
	f.Fuzz(func(t *testing.T, limit, offset string) {
		if hasNUL(limit) || hasNUL(offset) {
			return
		}
		req := httptest.NewRequest(http.MethodGet, "/api/v1/stats?limit="+url.QueryEscape(limit)+
			"&offset="+url.QueryEscape(offset), nil)
		n, err := srv.queryLimit(req)
		if err == nil && (n < 1 || n > max) {
			t.Fatalf("limit %q = %d, outside [1,%d]", limit, n, max)
		}
		off, err := queryOffset(req)
		if err == nil && off < 0 {
			t.Fatalf("offset %q = %d, negative", offset, off)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, req.URL.String(), nil))
		if rec.Code >= 500 {
			t.Fatalf("stats with limit %q offset %q = %d: %s", limit, offset, rec.Code, rec.Body.String())
		}
	})
}

const fanoutSize = 40

func FuzzServerReportPath(f *testing.F) {
	for _, c := range [][2]string{
		{"2026-01-02", "report.md"},
		{"2026-01-02", "report.json"},
		{"2026-1-2", "report.md"},
		{"2026-01-02", "../canary"},
		{"2026-01-02", "a/b"},
		{"2026-01-02", ""},
		{"", "report.md"},
		{"2026-01-02", ".hidden"},
		{"2026-01-02", "report.MD"},
		{"../..", "canary"},
		{"2026-02-30", "report.md"},
		{"2026-01-02", "CON"},
		{"2026-01-02", "nul.md"},
		{"2026-01-02", "com9.log"},
	} {
		f.Add(c[0], c[1])
	}
	cfg := newFixtureConfig(f)
	cfg.Canon.Paths.ReportsDir = filepath.Join(f.TempDir(), "reports")
	plantReportFile(f, cfg, "2026-01-02", "report.md", "# report\n")
	plantReportFile(f, cfg, "2026-01-02", "report.json", `{"ok":true}`)
	srv, err := New(cfg)
	if err != nil {
		f.Fatal(err)
	}
	f.Cleanup(func() { _ = srv.Close() })
	h := srv.Routes()
	f.Fuzz(func(t *testing.T, date, file string) {
		if hasNUL(date) || hasNUL(file) {
			return
		}
		path := "/api/v1/report/" + date + "/" + file
		for _, seg := range append(strings.Split(date, "/"), strings.Split(file, "/")...) {
			if seg == "" || seg == "." || seg == ".." {
				return
			}
		}
		enc := func(s string) string { return strings.ReplaceAll(s, "/", "%2F") }
		u := url.URL{Path: path, RawPath: "/api/v1/report/" + enc(date) + "/" + enc(file)}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, u.String(), nil))
		switch rec.Code {
		case http.StatusOK:
			want, err := os.ReadFile(filepath.Join(cfg.Canon.Paths.ReportsDir, date, file))
			if err != nil {
				t.Fatalf("200 for %q/%q but no such file: %v", date, file, err)
			}
			if !bytes.Equal(rec.Body.Bytes(), want) {
				t.Fatalf("report %q/%q body %q, want %q", date, file, rec.Body.String(), want)
			}
		case http.StatusBadRequest, http.StatusNotFound:
		default:
			t.Fatalf("report %q/%q = %d: %s", date, file, rec.Code, rec.Body.String())
		}
	})
}

func plantReportFile(f *testing.F, cfg *config.Config, date, name, content string) {
	f.Helper()
	dir := filepath.Join(cfg.Canon.Paths.ReportsDir, date)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		f.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		f.Fatal(err)
	}
}

func fanoutFixture(f *testing.F) *Server {
	cfg := newFixtureConfig(f)
	path := cfg.Canon.DBFile("fanout")
	s, err := graph.Open(path)
	if err != nil {
		f.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	if err := s.Migrate(); err != nil {
		f.Fatal(err)
	}
	l := graph.NewLoader(s, 100)
	if err := populateFanout(l); err != nil {
		_ = l.Flush()
		f.Fatalf("populate fanout: %v", err)
	}
	if err := l.Flush(); err != nil {
		f.Fatal(err)
	}
	for i := range fanoutSize {
		if err := s.SetCoversEdge(f.Context(), fmt.Sprintf("bt%d", i), "sujato/en/big", "{}"); err != nil {
			f.Fatal(err)
		}
	}
	if err := s.Checkpoint(); err != nil {
		f.Fatal(err)
	}
	if err := s.Close(); err != nil {
		f.Fatal(err)
	}
	if err := graph.PublishDB(cfg, "fanout", path); err != nil {
		f.Fatal(err)
	}
	srv, err := New(cfg)
	if err != nil {
		f.Fatal(err)
	}
	f.Cleanup(func() { _ = srv.Close() })
	return srv
}

func populateFanout(l *graph.Loader) error {
	root, err := l.EnsureNode(model.NodeDpdRoot, "dpd-root:big", "{}")
	if err != nil {
		return err
	}
	div, err := l.EnsureNode(model.NodeDivision, "big-div", "{}")
	if err != nil {
		return err
	}
	_, err = l.EnsureNode(model.NodeEdition, "sujato/en/big", "{}")
	if err != nil {
		return err
	}
	headwords := make([]string, 0, fanoutSize)
	for i := range fanoutSize {
		lemma := fmt.Sprintf("hw%d", i)
		headwords = append(headwords, lemma)
		hw, err := l.EnsureNode(model.NodeHeadword, "dpd:"+lemma,
			fmt.Sprintf(`{"lemma_1":%q,"root_key":"big"}`, lemma))
		if err != nil {
			return err
		}
		if err := l.AddEdge(hw, root, model.EdgeDerivedFrom, nil, "{}"); err != nil {
			return err
		}
	}
	raw, _ := json.Marshal(headwords)
	if _, err := l.EnsureNode(model.NodeDpdForm, "dpd-form:hw1", `{"headwords":`+string(raw)+`}`); err != nil {
		return err
	}
	for i := range fanoutSize {
		muid := fmt.Sprintf("bt%d", i)
		text, err := l.EnsureNode(model.NodeText, muid, "{}")
		if err != nil {
			return err
		}
		if err := l.AddEdge(text, div, model.EdgeChildOf, nil, "{}"); err != nil {
			return err
		}
		seg, err := l.EnsureNode(model.NodeSegment, "pli:"+muid+":0.1", `{"text":"x"}`)
		if err != nil {
			return err
		}
		if err := l.AddEdge(seg, text, model.EdgeChildOf, nil, "{}"); err != nil {
			return err
		}
	}
	return nil
}

func FuzzServerGraphCaps(f *testing.F) {
	for _, c := range [][2]int{
		{1, 1}, {2, 4}, {5, 5}, {10, 40}, {400, 1600}, {3, 1},
	} {
		f.Add(c[0], c[1])
	}
	srv := fanoutFixture(f)
	f.Fuzz(func(t *testing.T, maxNodes, maxEdges int) {
		if maxNodes < 1 {
			maxNodes = 1
		}
		if maxEdges < 1 {
			maxEdges = 1
		}
		cfgCopy := *srv.cfg
		cfgCopy.Server.GraphMaxNodes = min(maxNodes, config.MaxGraphNodes)
		cfgCopy.Server.GraphMaxEdges = min(maxEdges, 20000)
		capped, err := New(&cfgCopy)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = capped.Close() }()
		h := capped.Routes()
		for _, target := range []string{
			"/api/v1/graph/root?root=big",
			"/api/v1/graph/word?token=hw1",
			"/api/v1/graph/tree?division=big-div",
			"/api/v1/graph/coverage?division=big-div",
		} {
			var g graphResult
			getJSON(t, h, target, &g)
			if g.Meta.Nodes > cfgCopy.Server.GraphMaxNodes || g.Meta.Edges > cfgCopy.Server.GraphMaxEdges {
				t.Fatalf("%s caps %d/%d violated: %+v", target,
					cfgCopy.Server.GraphMaxNodes, cfgCopy.Server.GraphMaxEdges, g.Meta)
			}
			if g.Meta.Nodes != len(g.Elements.Nodes) || g.Meta.Edges != len(g.Elements.Edges) {
				t.Fatalf("%s meta counts disagree: %+v", target, g.Meta)
			}
			kept := map[string]bool{}
			for _, n := range g.Elements.Nodes {
				kept[n.Data.ID] = true
			}
			for _, e := range g.Elements.Edges {
				if !kept[e.Data.Source] || !kept[e.Data.Target] {
					t.Fatalf("%s dangling edge %+v", target, e.Data)
				}
			}
		}
	})
}
