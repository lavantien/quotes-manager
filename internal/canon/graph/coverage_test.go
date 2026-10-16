package graph

import (
	"testing"

	"github.com/lavantien/quotes-manager/internal/canon/model"
)

func buildCoverageStore(t *testing.T) *Store {
	t.Helper()
	s := openTempStore(t)
	l := newTestLoader(t, s, 100)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}

	segIDs := map[string]int64{}
	paliSeg := func(key string) int64 {
		t.Helper()
		id, err := l.EnsureNode(model.NodeSegment, "pli:"+key, `{"text":"x"}`)
		must(err)
		segIDs[key] = id
		return id
	}

	texts := []struct {
		key, div, data string
		segs           []string
	}{
		{"mn1", "mn-div", `{"name":"Mūlapariyāya"}`, []string{"mn1:0.1", "mn1:0.2", "mn1:0.3"}},
		{"mn2", "mn-div", `{}`, []string{"mn2:0.1"}},
		{"sn1", "sn-div", `{}`, []string{"sn1:0.1"}},
	}
	for _, tx := range texts {
		textID, err := l.EnsureNode(model.NodeText, tx.key, tx.data)
		must(err)
		divID, err := l.EnsureNode(model.NodeDivision, tx.div, `{}`)
		must(err)
		must(l.AddEdge(textID, divID, model.EdgeChildOf, nil, "{}"))
		for _, seg := range tx.segs {
			must(l.AddEdge(paliSeg(seg), textID, model.EdgeChildOf, nil, "{}"))
		}
	}

	root := func(key string) int64 {
		t.Helper()
		id, err := l.EnsureNode(model.NodeDpdRoot, dpdRootKeyPrefix+key, `{"root":"`+key+`"}`)
		must(err)
		return id
	}
	headword := func(lemma, data string) int64 {
		t.Helper()
		id, err := l.EnsureNode(model.NodeHeadword, headwordKeyPrefix+lemma, data)
		must(err)
		return id
	}
	dhr := root("dhṛ")
	gam := root("gam")
	dhamma := headword("dhamma", `{"lemma_1":"dhamma","pos":"m","meaning_1":"the teaching; nature"}`)
	dhareti := headword("dhāreti", `{"lemma_1":"dhāreti","pos":"v","meaning_1":"bears, carries"}`)
	gacchati := headword("gacchati", `{"lemma_1":"gacchati","pos":"v","meaning_1":"goes"}`)
	eva := headword("eva", `{"lemma_1":"eva","pos":"ind","meaning_1":"thus"}`)
	must(l.AddEdge(dhamma, dhr, model.EdgeDerivedFrom, nil, "{}"))
	must(l.AddEdge(dhareti, dhr, model.EdgeDerivedFrom, nil, "{}"))
	must(l.AddEdge(gacchati, gam, model.EdgeDerivedFrom, nil, "{}"))
	hasWord := func(segKey string, dst int64) {
		must(l.AddEdge(segIDs[segKey], dst, model.EdgeHasWord, nil, "{}"))
	}
	hasWord("mn1:0.1", dhamma)
	hasWord("mn1:0.2", dhamma)
	hasWord("mn2:0.1", dhamma)
	hasWord("sn1:0.1", dhareti)
	hasWord("mn1:0.2", gacchati)
	hasWord("mn1:0.3", eva)

	_, err := l.EnsureNode(model.NodeEdition, "sujato/en/mn", "{}")
	must(err)
	must(l.Flush())
	must(s.SetCoversEdge(t.Context(), "mn1", "sujato/en/mn", `{"tex":"mn.tex"}`))
	return s
}

func TestRootFamily(t *testing.T) {
	s := buildCoverageStore(t)
	ctx := t.Context()

	got, err := s.RootFamily(ctx, "dhṛ", 0)
	if err != nil {
		t.Fatal(err)
	}
	want := []RootMember{
		{Lemma1: "dhamma", POS: "m", Meaning1: "the teaching; nature", Segments: 3},
		{Lemma1: "dhāreti", POS: "v", Meaning1: "bears, carries", Segments: 1},
	}
	if len(got) != len(want) {
		t.Fatalf("root family = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("root family[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}

	lim, err := s.RootFamily(ctx, "dhṛ", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(lim) != 1 || lim[0].Lemma1 != "dhamma" {
		t.Fatalf("limited family = %+v, want only dhamma", lim)
	}

	gam, err := s.RootFamily(ctx, "gam", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(gam) != 1 || gam[0].Lemma1 != "gacchati" || gam[0].Segments != 1 {
		t.Fatalf("gam family = %+v", gam)
	}
}

func TestRootFamilyUnknownRoot(t *testing.T) {
	s := buildCoverageStore(t)
	got, err := s.RootFamily(t.Context(), "ka", 0)
	if err != nil {
		t.Fatalf("unknown root must not error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("unknown root family = %+v, want none", got)
	}
}

func TestCoverage(t *testing.T) {
	s := buildCoverageStore(t)
	ctx := t.Context()

	got, err := s.Coverage(ctx, "mn-div", 0)
	if err != nil {
		t.Fatal(err)
	}
	want := []CoverRow{
		{Muid: "mn1", Name: "Mūlapariyāya", Segments: 3, Covered: true},
		{Muid: "mn2", Name: "", Segments: 1, Covered: false},
	}
	if len(got) != len(want) {
		t.Fatalf("coverage = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("coverage[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}

	lim, err := s.Coverage(ctx, "mn-div", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(lim) != 1 || lim[0].Muid != "mn1" {
		t.Fatalf("limited coverage = %+v, want only mn1", lim)
	}

	sn, err := s.Coverage(ctx, "sn-div", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(sn) != 1 || sn[0].Muid != "sn1" || sn[0].Covered {
		t.Fatalf("sn coverage = %+v", sn)
	}
}

func TestCoverageSummary(t *testing.T) {
	s := buildCoverageStore(t)

	got, err := s.CoverageSummary(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	want := []DivisionCoverage{
		{Division: "mn-div", Texts: 2, Covered: 1},
		{Division: "sn-div", Texts: 1, Covered: 0},
	}
	if len(got) != len(want) {
		t.Fatalf("coverage summary = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("coverage summary[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestCoverageUnknownDivision(t *testing.T) {
	s := buildCoverageStore(t)
	got, err := s.Coverage(t.Context(), "no-div", 0)
	if err != nil {
		t.Fatalf("unknown division must not error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("unknown division coverage = %+v, want none", got)
	}
}
