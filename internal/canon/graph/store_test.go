package graph

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lavantien/quotes-manager/internal/canon/model"
)

func newTestLoader(t *testing.T, s *Store, batchSize int) *Loader {
	t.Helper()
	l := NewLoader(s, batchSize)
	t.Cleanup(l.abort)
	return l
}

func TestLoaderNodeRoundTrip(t *testing.T) {
	s := openTempStore(t)
	l := newTestLoader(t, s, 100)

	id1, err := l.EnsureNode(model.NodeText, "mn1", `{"name":"Mūlapariyāya"}`)
	if err != nil {
		t.Fatal(err)
	}
	if id1 <= 0 {
		t.Fatalf("id = %d, want > 0", id1)
	}
	id2, err := l.EnsureNode(model.NodeText, "mn1", `{"name":"updated"}`)
	if err != nil {
		t.Fatal(err)
	}
	if id2 != id1 {
		t.Fatalf("re-ensure id = %d, want %d", id2, id1)
	}
	id3, err := l.EnsureNode(model.NodeSegment, "mn1", `{}`)
	if err != nil {
		t.Fatal(err)
	}
	if id3 == id1 {
		t.Fatal("different type must be a different node")
	}
	if err := l.Flush(); err != nil {
		t.Fatal(err)
	}

	var data string
	if err := s.DB().QueryRow(`SELECT data FROM node WHERE id = ?`, id1).Scan(&data); err != nil {
		t.Fatal(err)
	}
	if data != `{"name":"updated"}` {
		t.Fatalf("data = %q, want updated", data)
	}
	var n int
	if err := s.DB().QueryRow(`SELECT COUNT(*) FROM node`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("node count = %d, want 2", n)
	}
}

func TestLoaderNodeEmptyDataDefaults(t *testing.T) {
	s := openTempStore(t)
	l := newTestLoader(t, s, 100)
	id, err := l.EnsureNode(model.NodeEdition, "sujato/en/mn", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := l.Flush(); err != nil {
		t.Fatal(err)
	}
	var data string
	if err := s.DB().QueryRow(`SELECT data FROM node WHERE id = ?`, id).Scan(&data); err != nil {
		t.Fatal(err)
	}
	if data != "{}" {
		t.Fatalf("data = %q, want {}", data)
	}
}

func TestLoaderNodeInvalidJSONRejected(t *testing.T) {
	s := openTempStore(t)
	l := newTestLoader(t, s, 100)
	if _, err := l.EnsureNode(model.NodeText, "mn1", "not json"); err == nil {
		t.Fatal("invalid JSON data must be rejected by the schema check")
	}
}

func TestLoaderEdgeUpsert(t *testing.T) {
	s := openTempStore(t)
	l := newTestLoader(t, s, 100)
	src, err := l.EnsureNode(model.NodeText, "mn1", `{}`)
	if err != nil {
		t.Fatal(err)
	}
	dst, err := l.EnsureNode(model.NodeCollection, "mn", `{}`)
	if err != nil {
		t.Fatal(err)
	}
	ord := int64(0)
	if err := l.AddEdge(src, dst, model.EdgeChildOf, &ord, `{"a":1}`); err != nil {
		t.Fatal(err)
	}
	ord2 := int64(7)
	if err := l.AddEdge(src, dst, model.EdgeChildOf, &ord2, `{"a":2}`); err != nil {
		t.Fatal(err)
	}
	if err := l.Flush(); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := s.DB().QueryRow(`SELECT COUNT(*) FROM edge`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("edge count = %d, want 1 (upsert, no duplicate)", n)
	}
	var gotOrd int64
	var data string
	if err := s.DB().QueryRow(`SELECT ord, data FROM edge`).Scan(&gotOrd, &data); err != nil {
		t.Fatal(err)
	}
	if gotOrd != 7 || data != `{"a":2}` {
		t.Fatalf("ord,data = %d,%q, want 7,{\"a\":2}", gotOrd, data)
	}
}

func TestLoaderEdgeForeignKey(t *testing.T) {
	s := openTempStore(t)
	l := newTestLoader(t, s, 100)
	if err := l.AddEdge(9999, 10000, model.EdgeChildOf, nil, "{}"); err == nil {
		t.Fatal("edge with unknown endpoints must fail the foreign key check")
	}
}

func TestLoaderBatchingVisibility(t *testing.T) {
	s := openTempStore(t)
	l := newTestLoader(t, s, 3)
	for i := 0; i < 8; i++ {
		if _, err := l.EnsureNode(model.NodeSegment, strings.Repeat("s", i), `{}`); err != nil {
			t.Fatal(err)
		}
	}
	var visible int
	if err := s.DB().QueryRow(`SELECT COUNT(*) FROM node`).Scan(&visible); err != nil {
		t.Fatal(err)
	}
	if visible != 6 {
		t.Fatalf("visible = %d, want 6 (two commits of 3, two buffered)", visible)
	}
	if err := l.Flush(); err != nil {
		t.Fatal(err)
	}
	if err := s.DB().QueryRow(`SELECT COUNT(*) FROM node`).Scan(&visible); err != nil {
		t.Fatal(err)
	}
	if visible != 8 {
		t.Fatalf("after flush visible = %d, want 8", visible)
	}
	if err := l.Flush(); err != nil {
		t.Fatal(err)
	}
}

func TestLoaderBatchSizeBelowOneDegrades(t *testing.T) {
	s := openTempStore(t)
	l := NewLoader(s, 0)
	for i := range 3 {
		if _, err := l.EnsureNode(model.NodeText, string(rune('a'+i)), "{}"); err != nil {
			t.Fatal(err)
		}
	}
	var visible int
	if err := s.DB().QueryRow(`SELECT COUNT(*) FROM node`).Scan(&visible); err != nil {
		t.Fatal(err)
	}
	if visible != 3 {
		t.Fatalf("visible = %d, want 3 (per-statement commits)", visible)
	}
}

func TestRunLifecycle(t *testing.T) {
	s := openTempStore(t)
	runID, err := s.NewRun("bilara123", "editions456", "v0.4.20260926")
	if err != nil {
		t.Fatal(err)
	}
	var bilara, editions, dpdTag, started string
	var finished *string
	if err := s.DB().QueryRow(
		`SELECT bilara_commit, editions_commit, dpd_tag, started, finished FROM run WHERE id = ?`, runID,
	).Scan(&bilara, &editions, &dpdTag, &started, &finished); err != nil {
		t.Fatal(err)
	}
	if bilara != "bilara123" || editions != "editions456" || dpdTag != "v0.4.20260926" {
		t.Fatalf("commits = %q,%q,%q", bilara, editions, dpdTag)
	}
	if _, err := time.Parse(time.RFC3339, started); err != nil {
		t.Fatalf("started %q is not RFC3339: %v", started, err)
	}
	if !strings.HasSuffix(started, "Z") {
		t.Fatalf("started %q is not UTC", started)
	}
	if finished != nil {
		t.Fatalf("finished = %q, want nil", *finished)
	}
	if err := s.FinishRun(runID); err != nil {
		t.Fatal(err)
	}
	if err := s.DB().QueryRow(`SELECT finished FROM run WHERE id = ?`, runID).Scan(&finished); err != nil {
		t.Fatal(err)
	}
	if finished == nil || *finished == "" {
		t.Fatal("finished must be set after FinishRun")
	}
	if err := s.FinishRun(runID + 1000); err == nil {
		t.Fatal("FinishRun on unknown run must fail")
	}
}

func TestInsertDiscrepancies(t *testing.T) {
	s := openTempStore(t)
	ctx := context.Background()
	runID, err := s.NewRun("b", "e", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.InsertDiscrepancies(ctx, runID, nil); err != nil {
		t.Fatalf("empty slice: %v", err)
	}
	ds := []model.Discrepancy{
		{Check: model.CheckEmptyValue, Severity: model.Info, Muid: "mn1", Detail: "empty root value at 1.1"},
		{Check: model.CheckDuplicateKeys, Severity: model.Error, Muid: "mn2", NodeID: 42, Detail: "dup key"},
	}
	if err := s.InsertDiscrepancies(ctx, runID, ds); err != nil {
		t.Fatal(err)
	}
	rows, err := s.DB().Query(
		`SELECT "check", severity, muid, node_id, detail FROM discrepancy WHERE run_id = ? ORDER BY id`, runID,
	)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	var got []model.Discrepancy
	for rows.Next() {
		var d model.Discrepancy
		var muid *string
		var nodeID sql.NullInt64
		if err := rows.Scan(&d.Check, &d.Severity, &muid, &nodeID, &d.Detail); err != nil {
			t.Fatal(err)
		}
		if muid != nil {
			d.Muid = *muid
		}
		d.NodeID = nodeID.Int64
		got = append(got, d)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Muid != "mn1" || got[1].NodeID != 42 {
		t.Fatalf("readback = %+v", got)
	}

	badSev := []model.Discrepancy{{Check: model.CheckEmptyValue, Severity: "fatal", Detail: "d"}}
	if err := s.InsertDiscrepancies(ctx, runID, badSev); err == nil {
		t.Fatal("invalid severity must be rejected by the schema check")
	}
	if err := s.InsertDiscrepancies(ctx, runID+1000, ds); err == nil {
		t.Fatal("unknown run_id must fail the foreign key check")
	}
}

func TestSetCoversEdge(t *testing.T) {
	s := openTempStore(t)
	l := newTestLoader(t, s, 100)
	textID, err := l.EnsureNode(model.NodeText, "mn1", `{}`)
	if err != nil {
		t.Fatal(err)
	}
	edID, err := l.EnsureNode(model.NodeEdition, "sujato/en/mn", `{}`)
	if err != nil {
		t.Fatal(err)
	}
	if err := l.Flush(); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := s.SetCoversEdge(ctx, "mn1", "sujato/en/mn", `{"tex":"mn.tex"}`); err != nil {
		t.Fatal(err)
	}
	var src, dst int64
	var typ, data string
	if err := s.DB().QueryRow(
		`SELECT src, dst, type, data FROM edge WHERE type = 'COVERED_BY'`,
	).Scan(&src, &dst, &typ, &data); err != nil {
		t.Fatal(err)
	}
	if src != textID || dst != edID || data != `{"tex":"mn.tex"}` {
		t.Fatalf("edge = %d->%d %s %s", src, dst, typ, data)
	}
	if err := s.SetCoversEdge(ctx, "mn1", "sujato/en/mn", `{"tex":"v2.tex"}`); err != nil {
		t.Fatal(err)
	}
	if err := s.DB().QueryRow(`SELECT data FROM edge WHERE type = 'COVERED_BY'`).Scan(&data); err != nil {
		t.Fatal(err)
	}
	if data != `{"tex":"v2.tex"}` {
		t.Fatalf("upsert data = %q", data)
	}
	if err := s.SetCoversEdge(ctx, "mn404", "sujato/en/mn", "{}"); err == nil {
		t.Fatal("unknown text node must error")
	}
	if err := s.SetCoversEdge(ctx, "mn1", "nope", "{}"); err == nil {
		t.Fatal("unknown edition node must error")
	}
}

func TestSetFTS(t *testing.T) {
	s := openTempStore(t)
	l := newTestLoader(t, s, 100)
	id, err := l.EnsureNode(model.NodeSegment, "pli:mn1:1.1", `{"text":"pariyāya"}`)
	if err != nil {
		t.Fatal(err)
	}
	if err := l.SetFTS(id, "pariyāya", "the basis"); err != nil {
		t.Fatal(err)
	}
	if err := l.Flush(); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := s.DB().QueryRow(
		`SELECT COUNT(*) FROM fts_segment WHERE fts_segment MATCH 'pali:pariyāya' AND rowid = ?`, id,
	).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("pali match count = %d, want 1", n)
	}
	if err := s.DB().QueryRow(
		`SELECT COUNT(*) FROM fts_segment WHERE fts_segment MATCH 'en:"the basis"'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("en match count = %d, want 1", n)
	}
	if err := l.SetFTS(id, "ekodhāna", "settled"); err != nil {
		t.Fatal(err)
	}
	if err := l.Flush(); err != nil {
		t.Fatal(err)
	}
	if err := s.DB().QueryRow(
		`SELECT COUNT(*) FROM fts_segment WHERE fts_segment MATCH 'pali:pariyāya'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("old token match count after merge = %d, want 1", n)
	}
	if err := s.DB().QueryRow(
		`SELECT COUNT(*) FROM fts_segment WHERE fts_segment MATCH 'pali:ekodhana'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("new pali match count = %d, want 1", n)
	}
	if err := s.DB().QueryRow(
		`SELECT COUNT(*) FROM fts_segment WHERE fts_segment MATCH 'pali:ekodhana'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("new pali match count = %d, want 1", n)
	}
}

func TestSetFTSDiacriticFoldingStored(t *testing.T) {
	s := openTempStore(t)
	l := newTestLoader(t, s, 100)
	id, err := l.EnsureNode(model.NodeSegment, "pli:mn1:1.1", `{"text":"pariyāya"}`)
	if err != nil {
		t.Fatal(err)
	}
	if err := l.SetFTS(id, "pariyāya", "x"); err != nil {
		t.Fatal(err)
	}
	if err := l.Flush(); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := s.DB().QueryRow(
		`SELECT COUNT(*) FROM fts_segment WHERE fts_segment MATCH 'pali:pariyaya'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("ascii query match count = %d, want 1", n)
	}
}

func TestOpenMissingDirFails(t *testing.T) {
	if _, err := Open(filepath.Join(t.TempDir(), "nope", "x.db")); err == nil {
		t.Fatal("opening a database in a missing directory must fail")
	}
}
