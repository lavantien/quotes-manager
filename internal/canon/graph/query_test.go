package graph

import (
	"context"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/lavantien/quotes-manager/internal/canon/model"
)

func ptr[T any](v T) *T { return &v }

func loadFixture(t *testing.T) *Store {
	t.Helper()
	s := openTempStore(t)
	l := newTestLoader(t, s, 5)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	ord := func(v int64) *int64 { return &v }

	col, err := l.EnsureNode(model.NodeCollection, "mn", `{}`)
	must(err)
	div, err := l.EnsureNode(model.NodeDivision, "mn-div", `{}`)
	must(err)
	must(l.AddEdge(div, col, model.EdgeChildOf, ord(0), "{}"))

	type seg struct {
		key, pali, en string
	}
	texts := []struct {
		key   string
		ord   int64
		seggs []seg
	}{
		{"mn1", 0, []seg{
			{"pli:mn1:0.1", "pariyāya", "the relational"},
			{"pli:mn1:0.2", "ekodhāna", "settled"},
			{"pli:mn1:0.3", "āsavā", ""},
		}},
		{"mn2", 1, []seg{
			{"pli:mn2:0.1", "sabbāsava", "all the taints"},
		}},
		{"mn10", 2, []seg{
			{"pli:mn10:0.1", "satipaṭṭhāna", ""},
		}},
	}
	for _, tx := range texts {
		textID, err := l.EnsureNode(model.NodeText, tx.key, `{}`)
		must(err)
		must(l.AddEdge(textID, div, model.EdgeChildOf, ord(tx.ord), "{}"))
		for i, sg := range tx.seggs {
			pid, err := l.EnsureNode(model.NodeSegment, sg.key, `{"text":"`+sg.pali+`"}`)
			must(err)
			must(l.AddEdge(pid, textID, model.EdgeChildOf, ord(int64(i)), "{}"))
			must(l.SetFTS(pid, sg.pali, sg.en))
			if sg.en != "" {
				enKey := "en/sujato:" + sg.key
				eid, err := l.EnsureNode(model.NodeSegment, enKey, `{"text":"`+sg.en+`"}`)
				must(err)
				must(l.AddEdge(eid, pid, model.EdgeTranslates, nil, "{}"))
			}
		}
	}
	must(l.Flush())
	return s
}

func TestSegments(t *testing.T) {
	s := loadFixture(t)
	ctx := context.Background()

	got, err := s.Segments(ctx, "mn1", false, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	want := []Pair{
		{Muid: "mn1", SegKey: "0.1", Pali: "pariyāya", En: "the relational", Author: "sujato"},
		{Muid: "mn1", SegKey: "0.2", Pali: "ekodhāna", En: "settled", Author: "sujato"},
		{Muid: "mn1", SegKey: "0.3", Pali: "āsavā", En: "", Author: ""},
	}
	if len(got) != len(want) {
		t.Fatalf("segments = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("segments[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}

	only, err := s.Segments(ctx, "mn1", true, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(only) != 2 {
		t.Fatalf("withEn=true segments = %+v, want 2 rows", only)
	}
	for _, p := range only {
		if p.En == "" || p.Author == "" {
			t.Fatalf("withEn row must carry translation: %+v", p)
		}
	}

	lim, err := s.Segments(ctx, "mn1", false, 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(lim) != 1 || lim[0].SegKey != "0.1" {
		t.Fatalf("limit 1 = %+v", lim)
	}

	page, err := s.Segments(ctx, "mn1", false, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(page) != 1 || page[0].SegKey != "0.2" {
		t.Fatalf("limit 1 offset 1 = %+v, want only 0.2", page)
	}

	tail, err := s.Segments(ctx, "mn1", false, 0, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(tail) != 1 || tail[0].SegKey != "0.3" {
		t.Fatalf("offset-only window = %+v, want only 0.3", tail)
	}

	far, err := s.Segments(ctx, "mn1", false, 50, math.MaxInt64)
	if err != nil {
		t.Fatalf("maximal offset = %v; want an empty window without error", err)
	}
	if len(far) != 0 {
		t.Fatalf("maximal offset = %+v; want no rows", far)
	}
}

func TestSegmentsMuidNoPrefixBleed(t *testing.T) {
	s := loadFixture(t)
	got, err := s.Segments(context.Background(), "mn1", false, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range got {
		if p.Muid != "mn1" {
			t.Fatalf("prefix bleed: %+v", p)
		}
	}
	if len(got) != 3 {
		t.Fatalf("segments = %d rows, want 3 (mn10 excluded)", len(got))
	}
}

func TestSegmentsUnknownMuid(t *testing.T) {
	s := loadFixture(t)
	got, err := s.Segments(context.Background(), "mn999", false, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("unknown muid = %+v, want none", got)
	}
}

func TestSearchFTS(t *testing.T) {
	s := loadFixture(t)
	ctx := context.Background()

	got, err := s.SearchFTS(ctx, "pariyaya", "pali", 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("ascii pali search = %+v, want 1 row (diacritic folding)", got)
	}
	want := Pair{Muid: "mn1", SegKey: "0.1", Pali: "pariyāya", En: "the relational", Author: "sujato"}
	if got[0] != want {
		t.Fatalf("search hit = %+v, want %+v", got[0], want)
	}

	en, err := s.SearchFTS(ctx, "taints", "en", 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(en) != 1 || en[0].Muid != "mn2" || en[0].En != "all the taints" {
		t.Fatalf("en search = %+v", en)
	}

	if _, err := s.SearchFTS(ctx, "x", "fr", 10, 0); err == nil {
		t.Fatal("invalid lang must error")
	}

	quoted, err := s.SearchFTS(ctx, `said "hi"`, "en", 10, 0)
	if err != nil {
		t.Fatalf("embedded quotes must be escaped, got %v", err)
	}
	if len(quoted) != 0 {
		t.Fatalf("quoted search = %+v, want none", quoted)
	}

	lim, err := s.SearchFTS(ctx, "the", "en", 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(lim) != 1 {
		t.Fatalf("limit 1 returned %d rows", len(lim))
	}

	page, err := s.SearchFTS(ctx, "the", "en", 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(page) != 1 || page[0].Muid != "mn2" || page[0].En != "all the taints" {
		t.Fatalf("limit 1 offset 1 = %+v, want the mn2 row", page)
	}

	tail, err := s.SearchFTS(ctx, "the", "en", 0, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(tail) != 1 || tail[0].Muid != "mn2" {
		t.Fatalf("offset-only window = %+v, want the mn2 row", tail)
	}
}

func TestTree(t *testing.T) {
	s := loadFixture(t)
	ctx := context.Background()

	all, err := s.Tree(ctx, "mn-div", "all", 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 9 {
		t.Fatalf("tree all = %d rows, want 9:\n%+v", len(all), all)
	}
	root := all[0]
	if root.Key != "mn-div" || root.Type != model.NodeDivision || root.Ord != nil || root.Depth != 0 {
		t.Fatalf("root = %+v", root)
	}
	want := []struct {
		key   string
		depth int
		ord   *int64
	}{
		{"mn-div", 0, nil},
		{"mn1", 1, ptr(int64(0))},
		{"mn2", 1, ptr(int64(1))},
		{"mn10", 1, ptr(int64(2))},
		{"pli:mn10:0.1", 2, ptr(int64(0))},
		{"pli:mn1:0.1", 2, ptr(int64(0))},
		{"pli:mn2:0.1", 2, ptr(int64(0))},
		{"pli:mn1:0.2", 2, ptr(int64(1))},
		{"pli:mn1:0.3", 2, ptr(int64(2))},
	}
	for i, w := range want {
		if all[i].Key != w.key || all[i].Depth != w.depth {
			t.Fatalf("tree[%d] = %+v, want %s at depth %d", i, all[i], w.key, w.depth)
		}
		if (all[i].Ord == nil) != (w.ord == nil) || (w.ord != nil && *all[i].Ord != *w.ord) {
			t.Fatalf("tree[%d].Ord = %+v, want %+v", i, all[i].Ord, w.ord)
		}
	}

	texts, err := s.Tree(ctx, "mn-div", "text", 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(texts) != 3 || texts[0].Key != "mn1" || texts[2].Key != "mn10" {
		t.Fatalf("tree text = %+v", texts)
	}

	segs, err := s.Tree(ctx, "mn-div", "segment", 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(segs) != 5 {
		t.Fatalf("tree segment = %d rows, want 5", len(segs))
	}
	wantOrder := []string{
		"pli:mn10:0.1", "pli:mn1:0.1", "pli:mn2:0.1",
		"pli:mn1:0.2", "pli:mn1:0.3",
	}
	for i, k := range wantOrder {
		if segs[i].Key != k {
			t.Fatalf("segment order[%d] = %s, want %s", i, segs[i].Key, k)
		}
	}

	lim, err := s.Tree(ctx, "mn-div", "text", 2, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(lim) != 2 {
		t.Fatalf("tree limit = %d rows, want 2", len(lim))
	}

	page, err := s.Tree(ctx, "mn-div", "text", 2, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(page) != 2 || page[0].Key != "mn2" || page[1].Key != "mn10" {
		t.Fatalf("tree offset window = %+v, want mn2 then mn10", page)
	}

	tail, err := s.Tree(ctx, "mn-div", "text", 0, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(tail) != 2 || tail[0].Key != "mn2" || tail[1].Key != "mn10" {
		t.Fatalf("tree offset-only window = %+v, want mn2 then mn10", tail)
	}

	if _, err := s.Tree(ctx, "mn-div", "bogus", 0, 0); err == nil {
		t.Fatal("invalid types filter must error")
	}

	none, err := s.Tree(ctx, "no-div", "all", 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(none) != 0 {
		t.Fatalf("unknown division = %+v, want none", none)
	}
}

func TestTreeSinglePassOrder(t *testing.T) {
	s := openTempStore(t)
	l := newTestLoader(t, s, 500)
	ord := func(v int64) *int64 { return &v }

	div, err := l.EnsureNode(model.NodeDivision, "d", `{}`)
	if err != nil {
		t.Fatal(err)
	}
	for i := range 500 {
		text, err := l.EnsureNode(model.NodeText, fmt.Sprintf("t%04d", i), `{}`)
		if err != nil {
			t.Fatal(err)
		}
		if err := l.AddEdge(text, div, model.EdgeChildOf, ord(int64(i)), "{}"); err != nil {
			t.Fatal(err)
		}
		for j := range 29 {
			seg, err := l.EnsureNode(model.NodeSegment, fmt.Sprintf("pli:t%04d:%d", i, j), `{}`)
			if err != nil {
				t.Fatal(err)
			}
			if err := l.AddEdge(seg, text, model.EdgeChildOf, ord(int64(j)), "{}"); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := l.Flush(); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	rows, err := s.Tree(ctx, "d", "all", 400, 0)
	if err != nil {
		t.Fatalf("tree over a 15k-node subtree must stay single-pass: %v", err)
	}
	if len(rows) != 400 {
		t.Fatalf("tree rows = %d, want 400", len(rows))
	}
	if rows[0].Key != "d" || rows[0].Ord != nil || rows[0].Depth != 0 {
		t.Fatalf("root row = %+v", rows[0])
	}
	if rows[1].Key != "t0000" || rows[1].Ord == nil || *rows[1].Ord != 0 || rows[1].Depth != 1 {
		t.Fatalf("first text row = %+v", rows[1])
	}
}

func TestStats(t *testing.T) {
	s := loadFixture(t)
	st, err := s.Stats(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for k, want := range map[string]int64{
		"node:collection": 1,
		"node:division":   1,
		"node:text":       3,
		"node:segment":    8,
		"edge:CHILD_OF":   9,
		"edge:TRANSLATES": 3,
	} {
		if st[k] != want {
			t.Fatalf("stats[%s] = %d, want %d", k, st[k], want)
		}
	}
}
