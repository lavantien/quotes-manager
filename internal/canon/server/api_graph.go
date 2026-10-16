package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/lavantien/quotes-manager/internal/canon/graph"
	"github.com/lavantien/quotes-manager/internal/canon/model"
	"github.com/lavantien/quotes-manager/internal/canon/norm"
)

func (s *Server) registerGraphRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/graph/word", s.apiGraphWord)
	mux.HandleFunc("GET /api/v1/graph/root", s.apiGraphRoot)
	mux.HandleFunc("GET /api/v1/graph/tree", s.apiGraphTree)
	mux.HandleFunc("GET /api/v1/graph/coverage", s.apiGraphCoverage)
}

type graphNodeData struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Type  string `json:"type"`
}

type graphNode struct {
	Data graphNodeData `json:"data"`
}

type graphEdgeData struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Type   string `json:"type"`
	Label  string `json:"label"`
}

type graphEdge struct {
	Data graphEdgeData `json:"data"`
}

type graphElements struct {
	Nodes []graphNode `json:"nodes"`
	Edges []graphEdge `json:"edges"`
}

type graphCaps struct {
	MaxNodes int `json:"max_nodes"`
	MaxEdges int `json:"max_edges"`
}

type graphMeta struct {
	Nodes     int       `json:"nodes"`
	Edges     int       `json:"edges"`
	Truncated bool      `json:"truncated"`
	Caps      graphCaps `json:"caps"`
}

type graphResult struct {
	Elements graphElements `json:"elements"`
	Meta     graphMeta     `json:"meta"`
}

type edgeKey struct {
	source, target, typ string
}

type elementSet struct {
	nodes []graphNode
	edges []graphEdge
	seenN map[string]bool
	seenE map[edgeKey]bool
	maxN  int
	maxE  int
}

func newElementSet(maxN, maxE int) *elementSet {
	return &elementSet{
		nodes: []graphNode{},
		edges: []graphEdge{},
		seenN: map[string]bool{},
		seenE: map[edgeKey]bool{},
		maxN:  maxN,
		maxE:  maxE,
	}
}

func (e *elementSet) addNode(id, label, typ string) {
	if e.seenN[id] {
		return
	}
	e.seenN[id] = true
	e.nodes = append(e.nodes, graphNode{Data: graphNodeData{ID: id, Label: label, Type: typ}})
}

func (e *elementSet) addEdge(source, target, typ, label string) {
	key := edgeKey{source: source, target: target, typ: typ}
	if e.seenE[key] {
		return
	}
	e.seenE[key] = true
	e.edges = append(e.edges, graphEdge{
		Data: graphEdgeData{Source: source, Target: target, Type: typ, Label: label},
	})
}

func (e *elementSet) finalize() graphResult {
	truncated := false
	nodes := e.nodes
	if len(nodes) > e.maxN {
		nodes = nodes[:e.maxN]
		truncated = true
	}
	kept := make(map[string]bool, len(nodes))
	for _, n := range nodes {
		kept[n.Data.ID] = true
	}
	edges := make([]graphEdge, 0, len(e.edges))
	for _, ed := range e.edges {
		if kept[ed.Data.Source] && kept[ed.Data.Target] {
			edges = append(edges, ed)
		} else {
			truncated = true
		}
	}
	if len(edges) > e.maxE {
		edges = edges[:e.maxE]
		truncated = true
	}
	return graphResult{
		Elements: graphElements{Nodes: nodes, Edges: edges},
		Meta: graphMeta{
			Nodes: len(nodes), Edges: len(edges), Truncated: truncated,
			Caps: graphCaps{MaxNodes: e.maxN, MaxEdges: e.maxE},
		},
	}
}

func (s *Server) coreGraphWord(ctx context.Context, token string) (any, error) {
	store, err := s.db.Store(ctx)
	if err != nil {
		return nil, err
	}
	entries, err := store.ResolveForm(ctx, token)
	if err != nil {
		return nil, err
	}
	es := newElementSet(s.cfg.Server.GraphMaxNodes, s.cfg.Server.GraphMaxEdges)
	formID := "dpd-form:" + norm.FoldPali(token)
	es.addNode(formID, token, "dpd_form")
	roots := []string{}
	for _, entry := range entries {
		hwID := "dpd:" + entry.Lemma1
		es.addNode(hwID, entry.Lemma1, "headword")
		es.addEdge(formID, hwID, "HAS_WORD", token)
		if entry.Root != "" && !slices.Contains(roots, entry.Root) {
			roots = append(roots, entry.Root)
		}
	}
	for _, root := range roots {
		rootID := "dpd-root:" + root
		es.addNode(rootID, root, "dpd_root")
		for _, entry := range entries {
			if entry.Root == root {
				es.addEdge("dpd:"+entry.Lemma1, rootID, "DERIVED_FROM", "")
			}
		}
		family, err := store.RootFamily(ctx, root, s.cfg.Server.GraphMaxNodes)
		if err != nil {
			return nil, err
		}
		for _, m := range family {
			es.addNode("dpd:"+m.Lemma1, m.Lemma1, "headword")
			es.addEdge("dpd:"+m.Lemma1, rootID, "DERIVED_FROM", "")
		}
	}
	return es.finalize(), nil
}

func (s *Server) coreGraphRoot(ctx context.Context, root string, limit int) (any, error) {
	store, err := s.db.Store(ctx)
	if err != nil {
		return nil, err
	}
	family, err := store.RootFamily(ctx, root, limit)
	if err != nil {
		return nil, err
	}
	es := newElementSet(s.cfg.Server.GraphMaxNodes, s.cfg.Server.GraphMaxEdges)
	rootID := "dpd-root:" + root
	es.addNode(rootID, root, "dpd_root")
	for _, m := range family {
		id := "dpd:" + m.Lemma1
		es.addNode(id, fmt.Sprintf("%s (%d)", m.Lemma1, m.Segments), "headword")
		es.addEdge(id, rootID, "DERIVED_FROM", "")
	}
	return es.finalize(), nil
}

type treeNodeRef struct {
	typ, key string
}

func treeChildOfEdgesSQL(emitted []treeNodeRef) (string, []any) {
	refs := make([]string, len(emitted))
	args := make([]any, 0, len(emitted)*2)
	for i, n := range emitted {
		refs[i] = "(?,?)"
		args = append(args, n.typ, n.key)
	}
	return `WITH emitted(id) AS (
		SELECT id FROM node WHERE (type, key) IN (VALUES ` + strings.Join(refs, ",") + `))
	SELECT child.key, parent.key
	FROM emitted c
	JOIN edge e ON e.src = c.id AND e.type = 'CHILD_OF'
	JOIN emitted p ON p.id = e.dst
	JOIN node child ON child.id = c.id
	JOIN node parent ON parent.id = e.dst
	ORDER BY child.key, parent.key`, args
}

func divisionLabel(ctx context.Context, store *graph.Store, division string) string {
	var name sql.NullString
	if err := store.DB().QueryRowContext(ctx,
		`SELECT COALESCE(json_extract(data, '$.name'), '') FROM node
		 WHERE type = 'division' AND key = ?`, division,
	).Scan(&name); err != nil || name.String == "" {
		return division
	}
	return name.String
}

func (s *Server) coreGraphTree(ctx context.Context, division, types string) (any, error) {
	store, err := s.db.Store(ctx)
	if err != nil {
		return nil, err
	}
	nodes, err := store.Tree(ctx, division, types, s.cfg.Server.GraphMaxNodes, 0)
	if err != nil {
		return nil, err
	}
	es := newElementSet(s.cfg.Server.GraphMaxNodes, s.cfg.Server.GraphMaxEdges)
	emitted := []treeNodeRef{{typ: string(model.NodeDivision), key: division}}
	es.addNode(division, divisionLabel(ctx, store, division), "division")
	for _, n := range nodes {
		label := n.Key
		if name := nodeName(n.Data); name != "" {
			label = name
		}
		emitted = append(emitted, treeNodeRef{typ: string(n.Type), key: n.Key})
		es.addNode(n.Key, label, string(n.Type))
	}
	query, args := treeChildOfEdgesSQL(emitted)
	rows, err := store.DB().QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("tree edges %s: %w", division, err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var child, parent string
		if err := rows.Scan(&child, &parent); err != nil {
			return nil, err
		}
		es.addEdge(child, parent, "CHILD_OF", "")
	}
	return es.finalize(), rows.Err()
}

func nodeName(dataJSON string) string {
	var payload struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal([]byte(dataJSON), &payload); err != nil {
		return ""
	}
	return payload.Name
}

const coverageEditionsSQL = `
	SELECT t.key, ed.key
	FROM node dv
	JOIN edge tch ON tch.dst = dv.id AND tch.type = 'CHILD_OF'
	JOIN node t ON t.id = tch.src AND t.type = 'text'
	JOIN edge c ON c.src = t.id AND c.type = 'COVERED_BY'
	JOIN node ed ON ed.id = c.dst
	WHERE dv.type = 'division' AND dv.key = ?
	ORDER BY t.key, ed.key`

func (s *Server) coreGraphCoverage(ctx context.Context, division string) (any, error) {
	store, err := s.db.Store(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := store.Coverage(ctx, division, s.cfg.Server.GraphMaxNodes)
	if err != nil {
		return nil, err
	}
	es := newElementSet(s.cfg.Server.GraphMaxNodes, s.cfg.Server.GraphMaxEdges)
	es.addNode(division, divisionLabel(ctx, store, division), "division")
	for _, r := range rows {
		label := r.Name
		if label == "" {
			label = r.Muid
		}
		es.addNode(r.Muid, label, "text")
		es.addEdge(r.Muid, division, "CHILD_OF", "")
	}
	editions, err := store.DB().QueryContext(ctx, coverageEditionsSQL, division)
	if err != nil {
		return nil, fmt.Errorf("coverage editions %s: %w", division, err)
	}
	defer func() { _ = editions.Close() }()
	for editions.Next() {
		var muid, editionKey string
		if err := editions.Scan(&muid, &editionKey); err != nil {
			return nil, err
		}
		es.addNode(editionKey, editionKey, "edition")
		es.addEdge(muid, editionKey, "COVERED_BY", editionKey)
	}
	return es.finalize(), editions.Err()
}

func (s *Server) apiGraphWord(w http.ResponseWriter, r *http.Request) {
	token, err := requireParam(r, "token")
	if err != nil {
		writeError(w, err)
		return
	}
	v, err := s.coreGraphWord(r.Context(), token)
	s.serveResult(w, v, err)
}

func (s *Server) apiGraphRoot(w http.ResponseWriter, r *http.Request) {
	root, err := requireParam(r, "root")
	if err != nil {
		writeError(w, err)
		return
	}
	limit, err := s.queryLimit(r)
	if err != nil {
		writeError(w, err)
		return
	}
	v, err := s.coreGraphRoot(r.Context(), root, limit)
	s.serveResult(w, v, err)
}

func (s *Server) apiGraphTree(w http.ResponseWriter, r *http.Request) {
	division, err := requireParam(r, "division")
	if err != nil {
		writeError(w, err)
		return
	}
	types, err := queryEnum(r, "types", "all", "segment", "text", "all")
	if err != nil {
		writeError(w, err)
		return
	}
	v, err := s.coreGraphTree(r.Context(), division, types)
	s.serveResult(w, v, err)
}

func (s *Server) apiGraphCoverage(w http.ResponseWriter, r *http.Request) {
	division, err := requireParam(r, "division")
	if err != nil {
		writeError(w, err)
		return
	}
	v, err := s.coreGraphCoverage(r.Context(), division)
	s.serveResult(w, v, err)
}
