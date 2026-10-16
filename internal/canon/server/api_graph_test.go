package server

import (
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"testing"

	"github.com/lavantien/quotes-manager/internal/canon/graph"
	"github.com/lavantien/quotes-manager/internal/canon/model"
	"github.com/lavantien/quotes-manager/internal/config"
)

func nodesByID(g graphResult) map[string]graphNodeData {
	out := map[string]graphNodeData{}
	for _, n := range g.Elements.Nodes {
		out[n.Data.ID] = n.Data
	}
	return out
}

func edgeSet(g graphResult) map[graphEdgeData]bool {
	out := map[graphEdgeData]bool{}
	for _, e := range g.Elements.Edges {
		out[e.Data] = true
	}
	return out
}

func getGraph(t *testing.T, h http.Handler, target string) graphResult {
	t.Helper()
	var g graphResult
	getJSON(t, h, target, &g)
	return g
}

func TestAPIGraphWord(t *testing.T) {
	srv, _ := newFixtureServer(t)
	h := srv.Routes()

	g := getGraph(t, h, "/api/v1/graph/word?token="+url.QueryEscape("dhammā"))
	nodes := nodesByID(g)
	wantNodes := map[string]graphNodeData{
		"dpd-form:dhamma": {ID: "dpd-form:dhamma", Label: "dhammā", Type: "dpd_form"},
		"dpd:dhamma":      {ID: "dpd:dhamma", Label: "dhamma", Type: "headword"},
		"dpd-root:dhṛ":    {ID: "dpd-root:dhṛ", Label: "dhṛ", Type: "dpd_root"},
		"dpd:dhāreti":     {ID: "dpd:dhāreti", Label: "dhāreti", Type: "headword"},
	}
	if len(nodes) != len(wantNodes) {
		t.Fatalf("word graph nodes = %v, want %v", nodes, wantNodes)
	}
	for id, want := range wantNodes {
		if nodes[id] != want {
			t.Fatalf("node %s = %+v, want %+v", id, nodes[id], want)
		}
	}
	edges := edgeSet(g)
	for _, want := range []graphEdgeData{
		{Source: "dpd-form:dhamma", Target: "dpd:dhamma", Type: "HAS_WORD", Label: "dhammā"},
		{Source: "dpd:dhamma", Target: "dpd-root:dhṛ", Type: "DERIVED_FROM"},
		{Source: "dpd:dhāreti", Target: "dpd-root:dhṛ", Type: "DERIVED_FROM"},
	} {
		if !edges[want] {
			t.Fatalf("word graph misses edge %+v (has %v)", want, edges)
		}
	}
	if g.Meta.Nodes != 4 || g.Meta.Edges != 3 || g.Meta.Truncated {
		t.Fatalf("word graph meta = %+v", g.Meta)
	}
	if g.Meta.Caps.MaxNodes != srv.cfg.Server.GraphMaxNodes ||
		g.Meta.Caps.MaxEdges != srv.cfg.Server.GraphMaxEdges {
		t.Fatalf("caps = %+v", g.Meta.Caps)
	}

	g = getGraph(t, h, "/api/v1/graph/word?token=evam")
	nodes = nodesByID(g)
	if len(nodes) != 2 || nodes["dpd:eva"].Label != "eva" {
		t.Fatalf("evam graph nodes = %v", nodes)
	}
	if len(g.Elements.Edges) != 1 {
		t.Fatalf("evam graph edges = %+v", g.Elements.Edges)
	}

	g = getGraph(t, h, "/api/v1/graph/word?token=nope")
	if len(g.Elements.Nodes) != 1 || g.Elements.Nodes[0].Data.ID != "dpd-form:nope" {
		t.Fatalf("unknown token graph = %+v", g.Elements.Nodes)
	}

	status, body := getError(t, h, "/api/v1/graph/word")
	if status != http.StatusBadRequest || body.Code != codeInvalidParam {
		t.Fatalf("missing token = %d/%s", status, body.Code)
	}
}

func TestAPIGraphRoot(t *testing.T) {
	srv, _ := newFixtureServer(t)
	h := srv.Routes()

	g := getGraph(t, h, "/api/v1/graph/root?root="+url.QueryEscape("dhṛ"))
	nodes := nodesByID(g)
	if len(nodes) != 3 {
		t.Fatalf("root graph nodes = %v", nodes)
	}
	if nodes["dpd-root:dhṛ"].Label != "dhṛ" || nodes["dpd-root:dhṛ"].Type != "dpd_root" {
		t.Fatalf("root node = %+v", nodes["dpd-root:dhṛ"])
	}
	if nodes["dpd:dhamma"].Label != "dhamma (3)" || nodes["dpd:dhāreti"].Label != "dhāreti (1)" {
		t.Fatalf("member labels = %+v", nodes)
	}
	if len(g.Elements.Edges) != 2 || g.Meta.Truncated {
		t.Fatalf("root graph = %+v", g)
	}

	g = getGraph(t, h, "/api/v1/graph/root?root="+url.QueryEscape("dhṛ")+"&limit=1")
	if len(g.Elements.Nodes) != 2 || len(g.Elements.Edges) != 1 {
		t.Fatalf("limited root graph = %+v", g)
	}

	g = getGraph(t, h, "/api/v1/graph/root?root=ka")
	if len(g.Elements.Nodes) != 1 || len(g.Elements.Edges) != 0 {
		t.Fatalf("unknown root graph = %+v", g)
	}

	status, body := getError(t, h, "/api/v1/graph/root")
	if status != http.StatusBadRequest || body.Code != codeInvalidParam {
		t.Fatalf("missing root = %d/%s", status, body.Code)
	}
}

func TestAPIGraphTree(t *testing.T) {
	srv, _ := newFixtureServer(t)
	h := srv.Routes()

	g := getGraph(t, h, "/api/v1/graph/tree?division=mn-div")
	if g.Meta.Nodes != 10 {
		t.Fatalf("tree graph nodes = %d:\n%+v", g.Meta.Nodes, g.Elements.Nodes)
	}
	if g.Meta.Edges != 9 || g.Meta.Truncated {
		t.Fatalf("tree graph meta = %+v", g.Meta)
	}
	nodes := nodesByID(g)
	if nodes["mn-div"].Label != "Middle Length Discourses" {
		t.Fatalf("division label = %+v", nodes["mn-div"])
	}
	if nodes["mn1"].Label != "Mūlapariyāya" || nodes["mn2"].Label != "mn2" {
		t.Fatalf("text labels = %+v / %+v", nodes["mn1"], nodes["mn2"])
	}
	edges := edgeSet(g)
	for _, want := range []graphEdgeData{
		{Source: "mn1", Target: "mn-div", Type: "CHILD_OF"},
		{Source: "mn2", Target: "mn-div", Type: "CHILD_OF"},
		{Source: "pli:mn1:0.1", Target: "mn1", Type: "CHILD_OF"},
		{Source: "pli:mn2:0.1", Target: "mn2", Type: "CHILD_OF"},
		{Source: "en/sujato:mn1:0.1", Target: "mn1", Type: "CHILD_OF"},
	} {
		if !edges[want] {
			t.Fatalf("tree graph misses edge %+v (has %v)", want, edges)
		}
	}

	g = getGraph(t, h, "/api/v1/graph/tree?division=mn-div&types=text")
	if g.Meta.Nodes != 3 || g.Meta.Edges != 2 {
		t.Fatalf("text-only tree graph = %+v", g)
	}
	edges = edgeSet(g)
	if !edges[graphEdgeData{Source: "mn2", Target: "mn-div", Type: "CHILD_OF"}] {
		t.Fatalf("text-only edges = %v", edges)
	}

	g = getGraph(t, h, "/api/v1/graph/tree?division=no-div")
	if g.Meta.Nodes != 1 || g.Meta.Edges != 0 {
		t.Fatalf("unknown division graph = %+v", g)
	}

	status, body := getError(t, h, "/api/v1/graph/tree")
	if status != http.StatusBadRequest || body.Code != codeInvalidParam {
		t.Fatalf("missing division = %d/%s", status, body.Code)
	}
	status, body = getError(t, h, "/api/v1/graph/tree?division=mn-div&types=bogus")
	if status != http.StatusBadRequest || body.Code != codeInvalidParam {
		t.Fatalf("bad types = %d/%s", status, body.Code)
	}
}

func TestAPIGraphCoverage(t *testing.T) {
	srv, _ := newFixtureServer(t)
	h := srv.Routes()

	g := getGraph(t, h, "/api/v1/graph/coverage?division=mn-div")
	nodes := nodesByID(g)
	wantNodes := map[string]graphNodeData{
		"mn-div":       {ID: "mn-div", Label: "Middle Length Discourses", Type: "division"},
		"mn1":          {ID: "mn1", Label: "Mūlapariyāya", Type: "text"},
		"mn2":          {ID: "mn2", Label: "mn2", Type: "text"},
		"sujato/en/mn": {ID: "sujato/en/mn", Label: "sujato/en/mn", Type: "edition"},
	}
	if len(nodes) != len(wantNodes) {
		t.Fatalf("coverage nodes = %v, want %v", nodes, wantNodes)
	}
	for id, want := range wantNodes {
		if nodes[id] != want {
			t.Fatalf("node %s = %+v, want %+v", id, nodes[id], want)
		}
	}
	edges := edgeSet(g)
	for _, want := range []graphEdgeData{
		{Source: "mn1", Target: "mn-div", Type: "CHILD_OF"},
		{Source: "mn2", Target: "mn-div", Type: "CHILD_OF"},
		{Source: "mn1", Target: "sujato/en/mn", Type: "COVERED_BY", Label: "sujato/en/mn"},
	} {
		if !edges[want] {
			t.Fatalf("coverage graph misses edge %+v (has %v)", want, edges)
		}
	}
	if g.Meta.Nodes != 4 || g.Meta.Edges != 3 || g.Meta.Truncated {
		t.Fatalf("coverage meta = %+v", g.Meta)
	}

	g = getGraph(t, h, "/api/v1/graph/coverage?division=no-div")
	if g.Meta.Nodes != 1 || g.Meta.Edges != 0 {
		t.Fatalf("unknown division coverage = %+v", g)
	}

	status, body := getError(t, h, "/api/v1/graph/coverage")
	if status != http.StatusBadRequest || body.Code != codeInvalidParam {
		t.Fatalf("missing division = %d/%s", status, body.Code)
	}
}

func graphGapFixture(t testing.TB, cfg *config.Config) {
	t.Helper()
	path := cfg.Canon.DBFile("run1")
	s, err := graph.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	if err := s.Migrate(); err != nil {
		t.Fatal(err)
	}
	l := graph.NewLoader(s, 100)
	ord := func(v int64) *int64 { return &v }
	div, err := l.EnsureNode(model.NodeDivision, "d1", `{"name":""}`)
	if err != nil {
		t.Fatal(err)
	}
	txt, err := l.EnsureNode(model.NodeText, "t1", `{"name":"T One"}`)
	if err != nil {
		t.Fatal(err)
	}
	if err := l.AddEdge(txt, div, model.EdgeChildOf, ord(0), "{}"); err != nil {
		t.Fatal(err)
	}
	dhr, err := l.EnsureNode(model.NodeDpdRoot, "dpd-root:dhṛ", `{"root":"dhṛ"}`)
	if err != nil {
		t.Fatal(err)
	}
	gam, err := l.EnsureNode(model.NodeDpdRoot, "dpd-root:gam", `{"root":"gam"}`)
	if err != nil {
		t.Fatal(err)
	}
	dhamma, err := l.EnsureNode(model.NodeHeadword, "dpd:dhamma",
		`{"lemma_1":"dhamma","root_key":"dhṛ","family_root":"dhṛ"}`)
	if err != nil {
		t.Fatal(err)
	}
	gacchati, err := l.EnsureNode(model.NodeHeadword, "dpd:gacchati",
		`{"lemma_1":"gacchati","root_key":"gam","family_root":"gam"}`)
	if err != nil {
		t.Fatal(err)
	}
	if err := l.AddEdge(dhamma, dhr, model.EdgeDerivedFrom, nil, "{}"); err != nil {
		t.Fatal(err)
	}
	if err := l.AddEdge(gacchati, gam, model.EdgeDerivedFrom, nil, "{}"); err != nil {
		t.Fatal(err)
	}
	if _, err := l.EnsureNode(model.NodeDpdForm, "dpd-form:ubho",
		`{"headwords":["dhamma","gacchati"]}`); err != nil {
		t.Fatal(err)
	}
	if err := l.Flush(); err != nil {
		t.Fatal(err)
	}
	if err := s.Checkpoint(); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if err := graph.PublishDB(cfg, "run1", path); err != nil {
		t.Fatal(err)
	}
}

func TestAPIGraphWordOwnRootEdgesOnly(t *testing.T) {
	cfg := newFixtureConfig(t)
	graphGapFixture(t, cfg)
	srv, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = srv.Close() })
	h := srv.Routes()

	g := getGraph(t, h, "/api/v1/graph/word?token=ubho")
	if g.Meta.Nodes != 5 || g.Meta.Edges != 4 || g.Meta.Truncated {
		t.Fatalf("cross-root word graph meta = %+v, want 5/4/untruncated", g.Meta)
	}
	edges := edgeSet(g)
	for _, want := range []graphEdgeData{
		{Source: "dpd-form:ubho", Target: "dpd:dhamma", Type: "HAS_WORD", Label: "ubho"},
		{Source: "dpd-form:ubho", Target: "dpd:gacchati", Type: "HAS_WORD", Label: "ubho"},
		{Source: "dpd:dhamma", Target: "dpd-root:dhṛ", Type: "DERIVED_FROM"},
		{Source: "dpd:gacchati", Target: "dpd-root:gam", Type: "DERIVED_FROM"},
	} {
		if !edges[want] {
			t.Fatalf("cross-root graph misses edge %+v (has %v)", want, edges)
		}
	}
	for _, wrong := range []graphEdgeData{
		{Source: "dpd:gacchati", Target: "dpd-root:dhṛ", Type: "DERIVED_FROM"},
		{Source: "dpd:dhamma", Target: "dpd-root:gam", Type: "DERIVED_FROM"},
	} {
		if edges[wrong] {
			t.Fatalf("cross-root graph drew foreign-root edge %+v", wrong)
		}
	}
}

func TestAPIGraphDivisionEmptyNameFallsBackToKey(t *testing.T) {
	cfg := newFixtureConfig(t)
	graphGapFixture(t, cfg)
	srv, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = srv.Close() })
	h := srv.Routes()

	g := getGraph(t, h, "/api/v1/graph/tree?division=d1&types=text")
	if got := nodesByID(g)["d1"].Label; got != "d1" {
		t.Fatalf("empty-name division label = %q, want the key d1", got)
	}
	g = getGraph(t, h, "/api/v1/graph/coverage?division=d1")
	if got := nodesByID(g)["d1"].Label; got != "d1" {
		t.Fatalf("empty-name coverage division label = %q, want the key d1", got)
	}
}

func cappedServer(t *testing.T) *Server {
	t.Helper()
	srv, cfg := newFixtureServer(t)
	cfg.Server.GraphMaxNodes = 3
	cfg.Server.GraphMaxEdges = 1
	return srv
}

func TestAPIGraphCapsTruncate(t *testing.T) {
	srv := cappedServer(t)
	h := srv.Routes()

	g := getGraph(t, h, "/api/v1/graph/coverage?division=mn-div")
	if g.Meta.Nodes != 3 || g.Meta.Edges != 1 || !g.Meta.Truncated {
		t.Fatalf("capped coverage meta = %+v", g.Meta)
	}
	nodes := nodesByID(g)
	if nodes["sujato/en/mn"].ID != "" || nodes["mn2"].ID == "" {
		t.Fatalf("capped nodes = %v", nodes)
	}
	edges := edgeSet(g)
	if !edges[graphEdgeData{Source: "mn1", Target: "mn-div", Type: "CHILD_OF"}] {
		t.Fatalf("capped edges = %v", edges)
	}
	if g.Meta.Caps.MaxNodes != 3 || g.Meta.Caps.MaxEdges != 1 {
		t.Fatalf("caps = %+v", g.Meta.Caps)
	}

	g = getGraph(t, h, "/api/v1/graph/word?token=dhamma")
	if g.Meta.Nodes > 3 || g.Meta.Edges > 1 || !g.Meta.Truncated {
		t.Fatalf("capped word graph meta = %+v", g.Meta)
	}
	kept := map[string]bool{}
	for _, n := range g.Elements.Nodes {
		kept[n.Data.ID] = true
	}
	for _, e := range g.Elements.Edges {
		if !kept[e.Data.Source] || !kept[e.Data.Target] {
			t.Fatalf("dangling edge %+v", e.Data)
		}
	}

	srv.cfg.Server.GraphMaxNodes = 400
	srv.cfg.Server.GraphMaxEdges = 1600
	g = getGraph(t, h, "/api/v1/graph/coverage?division=mn-div")
	if g.Meta.Truncated || g.Meta.Nodes != 4 || g.Meta.Edges != 3 {
		t.Fatalf("uncapped coverage meta = %+v", g.Meta)
	}
}

func TestElementSetCapInvariants(t *testing.T) {
	for _, tc := range []struct{ maxN, maxE int }{
		{1, 1}, {2, 3}, {3, 2}, {10, 10}, {5, 100},
	} {
		es := newElementSet(tc.maxN, tc.maxE)
		for i := range 12 {
			es.addNode(fmt.Sprintf("n%d", i), fmt.Sprintf("n%d", i), "t")
			es.addEdge(fmt.Sprintf("n%d", i), "n0", "E", "")
		}
		g := es.finalize()
		if g.Meta.Nodes > tc.maxN || g.Meta.Edges > tc.maxE {
			t.Fatalf("caps %d/%d violated: %+v", tc.maxN, tc.maxE, g.Meta)
		}
		if !g.Meta.Truncated {
			t.Fatalf("12-node fan into caps %d/%d must truncate", tc.maxN, tc.maxE)
		}
		kept := map[string]bool{}
		for _, n := range g.Elements.Nodes {
			kept[n.Data.ID] = true
		}
		for _, e := range g.Elements.Edges {
			if !kept[e.Data.Source] || !kept[e.Data.Target] {
				t.Fatalf("dangling edge %+v", e.Data)
			}
		}
		if g.Meta.Nodes != len(g.Elements.Nodes) || g.Meta.Edges != len(g.Elements.Edges) {
			t.Fatalf("meta counts disagree with elements: %+v", g.Meta)
		}
	}

	es := newElementSet(10, 10)
	for range 3 {
		es.addNode("a", "a", "t")
		es.addNode("b", "b", "t")
		es.addEdge("a", "b", "E", "l1")
		es.addEdge("a", "b", "E", "l2")
	}
	g := es.finalize()
	if len(g.Elements.Nodes) != 2 || len(g.Elements.Edges) != 1 {
		t.Fatalf("dedup graph = %+v", g)
	}
	if g.Elements.Edges[0].Data.Label != "l1" {
		t.Fatalf("edge label = %q, want first l1", g.Elements.Edges[0].Data.Label)
	}

	es = newElementSet(10, 10)
	es.addNode("a", "a", "t")
	es.addNode("b", "b", "t")
	es.addEdge("a", "b", "E", "")
	if g := es.finalize(); g.Meta.Truncated {
		t.Fatalf("within-cap graph truncated: %+v", g.Meta)
	}
}

func TestAPIGraphTreeTypeFilterIsNotTruncation(t *testing.T) {
	srv, _ := newFixtureServer(t)
	h := srv.Routes()

	g := getGraph(t, h, "/api/v1/graph/tree?division=mn-div&types=text")
	if g.Meta.Nodes != 3 || g.Meta.Edges != 2 || g.Meta.Truncated {
		t.Fatalf("text-only tree meta = %+v, want 3/2/untruncated", g.Meta)
	}
	for _, e := range g.Elements.Edges {
		if e.Data.Source == "mn-div" || e.Data.Target != "mn-div" {
			t.Fatalf("text-only tree keeps an edge outside emitted nodes: %+v", e.Data)
		}
	}

	g = getGraph(t, h, "/api/v1/graph/tree?division=mn-div&types=segment")
	if g.Meta.Nodes != 8 || g.Meta.Edges != 0 || g.Meta.Truncated {
		t.Fatalf("segment-only tree meta = %+v, want 8/0/untruncated", g.Meta)
	}
}

func TestTreeChildOfEdgesBoundedToEmitted(t *testing.T) {
	cfg := newFixtureConfig(t)
	path := filepath.Join(cfg.Canon.Paths.DataDir, "edges.db")
	s, err := graph.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	if err := s.Migrate(); err != nil {
		t.Fatal(err)
	}
	l := graph.NewLoader(s, 100)
	ord := func(v int64) *int64 { return &v }
	node := func(typ model.NodeType, key string) int64 {
		id, err := l.EnsureNode(typ, key, `{}`)
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	link := func(child, parent int64, o int64) {
		if err := l.AddEdge(child, parent, model.EdgeChildOf, ord(o), "{}"); err != nil {
			t.Fatal(err)
		}
	}
	d1 := node(model.NodeDivision, "d1")
	d2 := node(model.NodeDivision, "d2")
	t1 := node(model.NodeText, "t1")
	t2 := node(model.NodeText, "t2")
	s1 := node(model.NodeSegment, "pli:t1:0.1")
	link(t1, d1, 0)
	link(t2, d1, 1)
	link(t2, d2, 0)
	link(s1, t1, 0)
	if err := l.Flush(); err != nil {
		t.Fatal(err)
	}

	query, args := treeChildOfEdgesSQL([]treeNodeRef{
		{typ: "division", key: "d1"},
		{typ: "text", key: "t1"},
		{typ: "text", key: "t2"},
	})
	rows, err := s.DB().QueryContext(t.Context(), query, args...)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	var got []struct{ child, parent string }
	for rows.Next() {
		var pair struct{ child, parent string }
		if err := rows.Scan(&pair.child, &pair.parent); err != nil {
			t.Fatal(err)
		}
		got = append(got, pair)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	want := []struct{ child, parent string }{{"t1", "d1"}, {"t2", "d1"}}
	if len(got) != len(want) {
		t.Fatalf("edge pairs = %v, want %v", got, want)
	}
	for i, w := range want {
		if got[i] != w {
			t.Fatalf("edge pairs = %v, want %v", got, want)
		}
	}
}
