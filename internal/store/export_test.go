package store

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/lavantien/quotes-manager/internal/quote"
)

func buildDumpSource(t *testing.T) *SQLiteStore {
	t.Helper()
	s := newTestStore(t)
	q1 := mustCreate(t, s, quote.New("MN 1", "the Buddha, MN 1", []string{"body one"}))
	q2 := mustCreate(t, s, quote.New("MN 2", "the Buddha, MN 2", []string{"body two"}))
	cid, err := s.CreateCollection([]int64{q1, q2})
	if err != nil {
		t.Fatalf("create collection: %v", err)
	}
	if err := s.RenameCollection(cid, "My Col"); err != nil {
		t.Fatalf("rename collection: %v", err)
	}
	catID, err := s.CreateCategory("suffering")
	if err != nil {
		t.Fatalf("create category: %v", err)
	}
	if err := s.SetQuoteCategories(q1, []int64{catID}); err != nil {
		t.Fatalf("set categories: %v", err)
	}
	return s
}

func TestExportCapturesFullState(t *testing.T) {
	s := buildDumpSource(t)
	dump, err := s.Export()
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if dump.Version != DumpVersion {
		t.Errorf("Version = %d, want %d", dump.Version, DumpVersion)
	}
	if len(dump.Quotes) != 2 {
		t.Errorf("Quotes = %d, want 2", len(dump.Quotes))
	}
	if len(dump.Collections) != 1 {
		t.Fatalf("Collections = %d, want 1", len(dump.Collections))
	}
	c := dump.Collections[0]
	if c.Name != "My Col" {
		t.Errorf("collection name = %q, want My Col", c.Name)
	}
	if !equalInts(c.Items, []int64{1, 2}) {
		t.Errorf("collection items = %v, want [1 2]", c.Items)
	}
	if len(dump.Categories) != 1 || !equalInts(dump.Categories[0].Items, []int64{1}) {
		t.Errorf("categories = %+v, want one tagging quote 1", dump.Categories)
	}
}

func TestImportRestoresIntoFreshStore(t *testing.T) {
	src := buildDumpSource(t)
	dump, err := src.Export()
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	dst := newTestStore(t)
	if err := dst.Import(dump); err != nil {
		t.Fatalf("import: %v", err)
	}
	assertSameState(t, src, dst)
}

func TestImportJSONRoundTrip(t *testing.T) {
	src := buildDumpSource(t)
	data, err := json.Marshal(src.exportOrFail(t))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var dump Dump
	if err := json.Unmarshal(data, &dump); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	dst := newTestStore(t)
	if err := dst.Import(&dump); err != nil {
		t.Fatalf("import: %v", err)
	}
	assertSameState(t, src, dst)
}

func TestImportRejectsBadVersion(t *testing.T) {
	s := newTestStore(t)
	if err := s.Import(nil); !errors.Is(err, ErrUnsupportedDump) {
		t.Errorf("Import(nil) = %v, want ErrUnsupportedDump", err)
	}
	if err := s.Import(&Dump{Version: 999}); !errors.Is(err, ErrUnsupportedDump) {
		t.Errorf("Import(v999) = %v, want ErrUnsupportedDump", err)
	}
}

func (s *SQLiteStore) exportOrFail(t *testing.T) *Dump {
	t.Helper()
	d, err := s.Export()
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	return d
}

func assertSameState(t *testing.T, a, b *SQLiteStore) {
	t.Helper()
	la, _ := a.List()
	lb, _ := b.List()
	if len(la) != len(lb) {
		t.Errorf("quote count %d vs %d", len(la), len(lb))
	} else {
		for i := range la {
			if la[i].ID != lb[i].ID || la[i].SuttaID != lb[i].SuttaID || la[i].BodyText != lb[i].BodyText {
				t.Errorf("quote %d differs: %+v vs %+v", i, la[i], lb[i])
			}
		}
	}
	ca, _ := a.ListCollections()
	cb, _ := b.ListCollections()
	if len(ca) != len(cb) {
		t.Errorf("collection count %d vs %d", len(ca), len(lb))
	} else {
		for i := range ca {
			ia, _ := a.CollectionQuotes(ca[i].ID)
			ib, _ := b.CollectionQuotes(cb[i].ID)
			if ca[i].Name != cb[i].Name || !equalInts(quoteIDsOf(ia), quoteIDsOf(ib)) {
				t.Errorf("collection %d differs: %+v/%v vs %+v/%v", i, ca[i], quoteIDsOf(ia), cb[i], quoteIDsOf(ib))
			}
		}
	}
	pa, _ := a.ListCategories()
	pb, _ := b.ListCategories()
	if len(pa) != len(pb) {
		t.Errorf("category count %d vs %d", len(pa), len(pb))
	} else {
		for i := range pa {
			ia, _ := a.CategoryQuotes(pa[i].ID)
			ib, _ := b.CategoryQuotes(pb[i].ID)
			if pa[i].Name != pb[i].Name || !equalInts(quoteIDsOf(ia), quoteIDsOf(ib)) {
				t.Errorf("category %d differs: %+v/%v vs %+v/%v", i, pa[i], quoteIDsOf(ia), pb[i], quoteIDsOf(ib))
			}
		}
	}
}

func equalInts(a, b []int64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func buildGroupedDumpSource(t *testing.T) *SQLiteStore {
	t.Helper()
	s := newTestStore(t)
	q1 := mustCreate(t, s, quote.New("MN 1", "the Buddha, MN 1", []string{"body one"}))
	q2 := mustCreate(t, s, quote.New("MN 2", "the Buddha, MN 2", []string{"body two"}))
	cid, err := s.CreateCollection([]int64{q1, q2})
	if err != nil {
		t.Fatalf("create collection: %v", err)
	}
	g, err := s.CreateCollectionGroup(cid, "g")
	if err != nil {
		t.Fatalf("create group: %v", err)
	}
	if err := s.SetQuoteGroup(cid, q1, g); err != nil {
		t.Fatalf("set quote group: %v", err)
	}
	if err := s.AddSuttaToCollection(cid, "mn1", "MN 1", "https://suttacentral.net/mn1"); err != nil {
		t.Fatalf("add sutta: %v", err)
	}
	if err := s.AddSuttaToCollection(cid, "mn2", "MN 2", "https://suttacentral.net/mn2"); err != nil {
		t.Fatalf("add sutta: %v", err)
	}
	if err := s.SetSuttaGroup(cid, "mn1", g); err != nil {
		t.Fatalf("set sutta group: %v", err)
	}
	if err := s.SetSuttaLengths(map[string]int{"mn1": 312}); err != nil {
		t.Fatalf("set lengths: %v", err)
	}
	return s
}

func TestExportCapturesGroupsAndSuttas(t *testing.T) {
	s := buildGroupedDumpSource(t)
	dump, err := s.Export()
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if dump.Version != 2 {
		t.Errorf("Version = %d, want 2", dump.Version)
	}
	if len(dump.Suttas) != 2 || dump.Suttas[0].Muid != "mn1" || dump.Suttas[1].Muid != "mn2" {
		t.Errorf("Suttas = %+v", dump.Suttas)
	}
	if dump.Suttas[0].Length != 312 || dump.Suttas[1].Length != 0 {
		t.Errorf("Sutta lengths = %+v, want 312 and 0", dump.Suttas)
	}
	c := dump.Collections[0]
	if !equalInts(c.Items, []int64{1, 2}) {
		t.Errorf("Items = %v", c.Items)
	}
	if !equalInts(c.ItemGroups, []int64{1, 0}) {
		t.Errorf("ItemGroups = %v, want [1 0]", c.ItemGroups)
	}
	if len(c.Groups) != 1 || c.Groups[0].Name != "g" || !equalInts(c.Groups[0].Items, []int64{1}) {
		t.Errorf("Groups = %+v", c.Groups)
	}
	if len(c.SuttaItems) != 2 || c.SuttaItems[0].Muid != "mn1" || c.SuttaItems[0].GroupID != 1 {
		t.Errorf("SuttaItems[0] = %+v", c.SuttaItems[0])
	}
	if c.SuttaItems[1].Muid != "mn2" || c.SuttaItems[1].GroupID != 0 {
		t.Errorf("SuttaItems[1] = %+v", c.SuttaItems[1])
	}
}

func TestImportRoundTripsGroupsAndSuttas(t *testing.T) {
	src := buildGroupedDumpSource(t)
	dump, err := src.Export()
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	dst := newTestStore(t)
	if err := dst.Import(dump); err != nil {
		t.Fatalf("import: %v", err)
	}

	sg, _ := src.ListCollectionGroups(1)
	dg, _ := dst.ListCollectionGroups(1)
	if len(sg) != len(dg) || len(dg) != 1 {
		t.Fatalf("groups = %+v vs %+v", sg, dg)
	}
	if dg[0].ID != sg[0].ID || dg[0].Name != sg[0].Name || dg[0].Count != sg[0].Count {
		t.Errorf("group = %+v, want %+v", dg[0], sg[0])
	}
	sq, _ := src.CollectionQuoteItems(1)
	dq, _ := dst.CollectionQuoteItems(1)
	if len(sq) != len(dq) || len(dq) != 2 {
		t.Fatalf("quote items = %+v vs %+v", sq, dq)
	}
	for i := range dq {
		if dq[i].ID != sq[i].ID || dq[i].GroupID != sq[i].GroupID {
			t.Errorf("quote item %d = %+v, want %+v", i, dq[i], sq[i])
		}
	}
	ss, _ := src.CollectionSuttaItems(1)
	ds, _ := dst.CollectionSuttaItems(1)
	if len(ss) != len(ds) || len(ds) != 2 {
		t.Fatalf("sutta items = %+v vs %+v", ss, ds)
	}
	for i := range ds {
		if ds[i].Muid != ss[i].Muid || ds[i].Position != ss[i].Position || ds[i].GroupID != ss[i].GroupID {
			t.Errorf("sutta item %d = %+v, want %+v", i, ds[i], ss[i])
		}
	}
	sreg, _ := src.ListSuttas()
	dreg, _ := dst.ListSuttas()
	if len(sreg) != len(dreg) {
		t.Fatalf("registry = %+v vs %+v", sreg, dreg)
	}
	for i := range dreg {
		if dreg[i] != sreg[i] {
			t.Errorf("registry %d = %+v, want %+v", i, dreg[i], sreg[i])
		}
	}
	if dreg[0].Length != 312 {
		t.Errorf("registry length after round-trip = %d, want 312", dreg[0].Length)
	}
}

func TestImportAcceptsV1Dump(t *testing.T) {
	s := newTestStore(t)
	d := &Dump{
		Version: 1,
		Quotes: []Quote{
			{ID: 1, SuttaID: "MN 1", Citation: "the Buddha, MN 1", BodyMD: "x", BodyText: "x", LineCount: 1, CharCount: 1, Sources: nil},
		},
		Collections: []CollectionDump{
			{ID: 1, Name: "old", Items: []int64{1}},
		},
	}
	if err := s.Import(d); err != nil {
		t.Fatalf("import v1: %v", err)
	}
	qs, _ := s.CollectionQuotes(1)
	if len(qs) != 1 || qs[0].ID != 1 {
		t.Errorf("v1 membership = %+v", qs)
	}
	groups, _ := s.ListCollectionGroups(1)
	if len(groups) != 0 {
		t.Errorf("v1 import created groups: %+v", groups)
	}
	suttas, _ := s.ListSuttas()
	if len(suttas) != 0 {
		t.Errorf("v1 import created suttas: %+v", suttas)
	}
}

func TestImportPositionsKeepInsertAtBoundary(t *testing.T) {
	src := newTestStore(t)
	var ids []int64
	for _, body := range []string{"a", "b", "c"} {
		ids = append(ids, mustCreate(t, src, quote.New(body, body, []string{body})))
	}
	cid, err := src.CreateCollection(ids)
	if err != nil {
		t.Fatal(err)
	}
	dump, err := src.Export()
	if err != nil {
		t.Fatal(err)
	}
	dst := newTestStore(t)
	if err := dst.Import(dump); err != nil {
		t.Fatal(err)
	}
	q4 := mustCreate(t, dst, quote.New("d", "d", []string{"d"}))
	if err := dst.InsertAtCollection(cid, []int64{q4}, 1); err != nil {
		t.Fatal(err)
	}
	qs, err := dst.CollectionQuotes(cid)
	if err != nil {
		t.Fatal(err)
	}
	want := append([]int64{q4}, ids...)
	if len(qs) != len(want) {
		t.Fatalf("collection = %v, want %v", ids, want)
	}
	for i, q := range qs {
		if q.ID != want[i] {
			t.Errorf("pos %d = %d, want %d", i, q.ID, want[i])
		}
	}
}
