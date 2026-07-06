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
