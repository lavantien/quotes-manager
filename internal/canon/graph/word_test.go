package graph

import (
	"math"
	"testing"

	"github.com/lavantien/quotes-manager/internal/canon/model"
)

func buildWordStore(t *testing.T) *Store {
	t.Helper()
	s := openTempStore(t)
	l := newTestLoader(t, s, 100)
	hw := func(lemma, data string) int64 {
		t.Helper()
		id, err := l.EnsureNode(model.NodeHeadword, "dpd:"+lemma, data)
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	dhamma := hw("dhamma", `{"lemma_1":"dhamma","pos":"m","meaning_1":"the teaching; nature","root_key":"dhṛ","family_root":"dhṛ"}`)
	eva := hw("eva", `{"lemma_1":"eva","pos":"ind","meaning_1":"thus, in this way"}`)
	form := func(form string, headwords ...string) {
		t.Helper()
		list := "["
		for i, h := range headwords {
			if i > 0 {
				list += ","
			}
			list += `"` + h + `"`
		}
		list += "]"
		if _, err := l.EnsureNode(model.NodeDpdForm, "dpd-form:"+form, `{"headwords":`+list+`}`); err != nil {
			t.Fatal(err)
		}
	}
	form("dhamma", "dhamma")
	form("dhammo", "dhamma")
	form("evam", "eva")
	seg := func(key string) int64 {
		t.Helper()
		id, err := l.EnsureNode(model.NodeSegment, key, `{"text":"x"}`)
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	link := func(segKey string, dst int64, data string) {
		t.Helper()
		if err := l.AddEdge(seg(segKey), dst, model.EdgeHasWord, nil, data); err != nil {
			t.Fatal(err)
		}
	}
	link("pli:mn1:1.1", dhamma, `{"forms":["dhamma"]}`)
	link("pli:mn1:2.1", dhamma, `{"forms":["dhamma","dhammo"]}`)
	link("pli:mn2:1.1", dhamma, `{"forms":["dhammo"]}`)
	link("pli:mn1:1.1", eva, `{"forms":["evam"]}`)
	if err := l.Flush(); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestResolveForm(t *testing.T) {
	s := buildWordStore(t)
	ctx := t.Context()

	entries, err := s.ResolveForm(ctx, "dhammā")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("entries = %+v; want 1", entries)
	}
	e := entries[0]
	if e.Lemma1 != "dhamma" || e.POS != "m" || e.Meaning1 != "the teaching; nature" ||
		e.Root != "dhṛ" || e.FamilyRoot != "dhṛ" || e.Segments != 3 {
		t.Errorf("entry = %+v", e)
	}

	entries, err = s.ResolveForm(ctx, "evaṃ")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Lemma1 != "eva" || entries[0].Root != "" || entries[0].Segments != 1 {
		t.Errorf("eva entries = %+v", entries)
	}
}

func TestResolveFormUnknown(t *testing.T) {
	s := buildWordStore(t)
	entries, err := s.ResolveForm(t.Context(), "dhammaca")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("unknown form entries = %+v; want none", entries)
	}
}

func TestHeadwordUsage(t *testing.T) {
	s := buildWordStore(t)
	ctx := t.Context()

	n, refs, err := s.HeadwordUsage(ctx, "dhamma", true, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Errorf("usage = %d; want 3", n)
	}
	want := []string{"mn1:1.1", "mn1:2.1", "mn2:1.1"}
	if len(refs) != len(want) {
		t.Fatalf("refs = %v; want %v", refs, want)
	}
	for i := range want {
		if refs[i] != want[i] {
			t.Fatalf("refs = %v; want %v", refs, want)
		}
	}

	n, refs, err = s.HeadwordUsage(ctx, "dhamma", true, 2, 0)
	if err != nil {
		t.Fatal(err)
	}
	if n != 3 || len(refs) != 2 {
		t.Errorf("limited usage = %d/%v; want 3 total with 2 refs", n, refs)
	}

	n, refs, err = s.HeadwordUsage(ctx, "dhamma", true, 2, 1)
	if err != nil {
		t.Fatal(err)
	}
	if n != 3 || len(refs) != 2 || refs[0] != "mn1:2.1" || refs[1] != "mn2:1.1" {
		t.Errorf("offset usage = %d/%v; want 3 total with the last 2 refs", n, refs)
	}

	n, refs, err = s.HeadwordUsage(ctx, "dhamma", true, 0, 2)
	if err != nil {
		t.Fatal(err)
	}
	if n != 3 || len(refs) != 1 || refs[0] != "mn2:1.1" {
		t.Errorf("offset-only usage = %d/%v; want 3 total with the last ref", n, refs)
	}

	n, refs, err = s.HeadwordUsage(ctx, "dhamma", true, 50, math.MaxInt64)
	if err != nil {
		t.Fatalf("maximal offset = %v; want an empty window without error", err)
	}
	if n != 3 || len(refs) != 0 {
		t.Fatalf("maximal offset usage = %d/%v; want 3 total with no refs", n, refs)
	}

	n, refs, err = s.HeadwordUsage(ctx, "dhamma", false, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if n != 3 || refs != nil {
		t.Errorf("count-only usage = %d/%v; want 3 with no refs", n, refs)
	}

	n, _, err = s.HeadwordUsage(ctx, "nobody", true, 0, 0)
	if err != nil || n != 0 {
		t.Errorf("unknown lemma usage = %d/%v; want 0", n, err)
	}
}
