package store

import (
	"errors"
	"testing"

	"github.com/lavantien/quotes-manager/internal/quote"
)

func TestAddSuttaToCollectionAppends(t *testing.T) {
	s := newTestStore(t)
	cid := mustCollection(t, s)

	if err := s.AddSuttaToCollection(cid, "mn1", "MN 1", "https://suttacentral.net/mn1"); err != nil {
		t.Fatal(err)
	}
	if err := s.AddSuttaToCollection(cid, "mn2", "MN 2", "https://suttacentral.net/mn2"); err != nil {
		t.Fatal(err)
	}
	items, err := s.CollectionSuttaItems(cid)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("items = %+v, want 2", items)
	}
	if items[0].Muid != "mn1" || items[0].Position != 1 || items[0].Display != "MN 1" {
		t.Errorf("item 0 = %+v", items[0])
	}
	if items[1].Muid != "mn2" || items[1].Position != 2 || items[1].Link != "https://suttacentral.net/mn2" {
		t.Errorf("item 1 = %+v", items[1])
	}
}

func TestAddSuttaToCollectionDuplicateIsNoOp(t *testing.T) {
	s := newTestStore(t)
	cid := mustCollection(t, s)

	if err := s.AddSuttaToCollection(cid, "mn1", "MN 1", "https://suttacentral.net/mn1"); err != nil {
		t.Fatal(err)
	}
	if err := s.AddSuttaToCollection(cid, "mn1", "MN 1", "https://suttacentral.net/mn1/en/sujato"); err != nil {
		t.Fatalf("duplicate add: %v", err)
	}
	items, _ := s.CollectionSuttaItems(cid)
	if len(items) != 1 || items[0].Position != 1 {
		t.Errorf("items after duplicate = %+v", items)
	}
	suttas, _ := s.ListSuttas()
	if len(suttas) != 1 || suttas[0].Link != "https://suttacentral.net/mn1" {
		t.Errorf("registry after duplicate = %+v, want first link kept", suttas)
	}
}

func TestAddSuttaToCollectionUnknownCollection(t *testing.T) {
	s := newTestStore(t)
	if err := s.AddSuttaToCollection(999, "mn1", "MN 1", "https://suttacentral.net/mn1"); !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestListSuttasOrderedByMuid(t *testing.T) {
	s := newTestStore(t)
	cid := mustCollection(t, s)
	for _, su := range []Sutta{
		{Muid: "mn2", Display: "MN 2", Link: "https://suttacentral.net/mn2"},
		{Muid: "mn10", Display: "MN 10", Link: "https://suttacentral.net/mn10"},
		{Muid: "mn1", Display: "MN 1", Link: "https://suttacentral.net/mn1"},
	} {
		if err := s.AddSuttaToCollection(cid, su.Muid, su.Display, su.Link); err != nil {
			t.Fatal(err)
		}
	}
	suttas, err := s.ListSuttas()
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"mn1", "mn10", "mn2"}
	if len(suttas) != len(want) {
		t.Fatalf("suttas = %v, want %v", suttas, want)
	}
	for i, su := range suttas {
		if su.Muid != want[i] {
			t.Errorf("pos %d = %s, want %s", i, su.Muid, want[i])
		}
	}
}

func TestListCollectionsCountSumsQuotesAndSuttas(t *testing.T) {
	s := newTestStore(t)
	q := mustCreate(t, s, quote.New("A", "A", []string{"a"}))
	cid, err := s.CreateCollection([]int64{q})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.AddSuttaToCollection(cid, "mn1", "MN 1", "https://suttacentral.net/mn1"); err != nil {
		t.Fatal(err)
	}
	cols, _ := s.ListCollections()
	if len(cols) != 1 || cols[0].Count != 2 {
		t.Errorf("count = %+v, want 2 items", cols)
	}
	got, _ := s.GetCollection(cid)
	if got.Count != 2 {
		t.Errorf("GetCollection count = %d, want 2", got.Count)
	}
}

func TestRemoveSuttaFromCollectionKeepsRegistry(t *testing.T) {
	s := newTestStore(t)
	cid := mustCollection(t, s)
	_ = s.AddSuttaToCollection(cid, "mn1", "MN 1", "https://suttacentral.net/mn1")

	if err := s.RemoveSuttaFromCollection(cid, "mn1"); err != nil {
		t.Fatal(err)
	}
	items, _ := s.CollectionSuttaItems(cid)
	if len(items) != 0 {
		t.Errorf("items after remove = %+v", items)
	}
	suttas, _ := s.ListSuttas()
	if len(suttas) != 1 || suttas[0].Muid != "mn1" {
		t.Errorf("registry after remove = %+v, want mn1 kept", suttas)
	}
}

func TestRemoveSuttaFromCollectionNotFound(t *testing.T) {
	s := newTestStore(t)
	cid := mustCollection(t, s)
	if err := s.RemoveSuttaFromCollection(cid, "mn1"); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown membership: err = %v, want ErrNotFound", err)
	}
	if err := s.RemoveSuttaFromCollection(999, "mn1"); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown collection: err = %v, want ErrNotFound", err)
	}
}

func TestSuttaPositionsAppendAfterRemove(t *testing.T) {
	s := newTestStore(t)
	cid := mustCollection(t, s)
	for _, muid := range []string{"mn1", "mn2", "mn3"} {
		if err := s.AddSuttaToCollection(cid, muid, muid, muid); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.RemoveSuttaFromCollection(cid, "mn2"); err != nil {
		t.Fatal(err)
	}
	if err := s.AddSuttaToCollection(cid, "mn4", "mn4", "mn4"); err != nil {
		t.Fatal(err)
	}
	items, _ := s.CollectionSuttaItems(cid)
	want := []struct {
		muid string
		pos  int
	}{{"mn1", 1}, {"mn3", 3}, {"mn4", 4}}
	if len(items) != len(want) {
		t.Fatalf("items = %+v, want %v", items, want)
	}
	for i, it := range items {
		if it.Muid != want[i].muid || it.Position != want[i].pos {
			t.Errorf("item %d = %+v, want %+v", i, it, want[i])
		}
	}
}

func TestSetSuttaGroup(t *testing.T) {
	s := newTestStore(t)
	cid := mustCollection(t, s)
	g, _ := s.CreateCollectionGroup(cid, "group")
	_ = s.AddSuttaToCollection(cid, "mn1", "MN 1", "https://suttacentral.net/mn1")

	if err := s.SetSuttaGroup(cid, "mn1", g); err != nil {
		t.Fatal(err)
	}
	items, _ := s.CollectionSuttaItems(cid)
	if items[0].GroupID != g {
		t.Errorf("group id = %d, want %d", items[0].GroupID, g)
	}
	if err := s.SetSuttaGroup(cid, "mn1", 0); err != nil {
		t.Fatal(err)
	}
	items, _ = s.CollectionSuttaItems(cid)
	if items[0].GroupID != 0 {
		t.Errorf("group id after clear = %d, want 0", items[0].GroupID)
	}
}

func TestSetSuttaGroupNotFound(t *testing.T) {
	s := newTestStore(t)
	cid := mustCollection(t, s)
	g, _ := s.CreateCollectionGroup(cid, "group")
	_ = s.AddSuttaToCollection(cid, "mn1", "MN 1", "https://suttacentral.net/mn1")

	if err := s.SetSuttaGroup(cid, "mn1", 999); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown group: err = %v, want ErrNotFound", err)
	}
	if err := s.SetSuttaGroup(cid, "mn404", g); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown membership: err = %v, want ErrNotFound", err)
	}
	other := mustCollection(t, s)
	if err := s.SetSuttaGroup(other, "mn1", g); !errors.Is(err, ErrNotFound) {
		t.Errorf("group of another collection: err = %v, want ErrNotFound", err)
	}
}

func TestSetQuoteGroup(t *testing.T) {
	s := newTestStore(t)
	q := mustCreate(t, s, quote.New("A", "A", []string{"a"}))
	cid, _ := s.CreateCollection([]int64{q})
	g, _ := s.CreateCollectionGroup(cid, "group")

	if err := s.SetQuoteGroup(cid, q, g); err != nil {
		t.Fatal(err)
	}
	items, err := s.CollectionQuoteItems(cid)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].ID != q || items[0].GroupID != g {
		t.Errorf("items = %+v, want quote %d in group %d", items, q, g)
	}
	if err := s.SetQuoteGroup(cid, q, 0); err != nil {
		t.Fatal(err)
	}
	items, _ = s.CollectionQuoteItems(cid)
	if items[0].GroupID != 0 {
		t.Errorf("group id after clear = %d, want 0", items[0].GroupID)
	}
}

func TestSetQuoteGroupNotFound(t *testing.T) {
	s := newTestStore(t)
	q := mustCreate(t, s, quote.New("A", "A", []string{"a"}))
	q2 := mustCreate(t, s, quote.New("B", "B", []string{"b"}))
	cid, _ := s.CreateCollection([]int64{q})
	g, _ := s.CreateCollectionGroup(cid, "group")

	if err := s.SetQuoteGroup(cid, q, 999); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown group: err = %v, want ErrNotFound", err)
	}
	if err := s.SetQuoteGroup(cid, q2, g); !errors.Is(err, ErrNotFound) {
		t.Errorf("non-member quote: err = %v, want ErrNotFound", err)
	}
	other := mustCollection(t, s)
	if err := s.SetQuoteGroup(other, q, g); !errors.Is(err, ErrNotFound) {
		t.Errorf("group of another collection: err = %v, want ErrNotFound", err)
	}
}

func TestDeleteCollectionGroupNullsMemberships(t *testing.T) {
	s := newTestStore(t)
	q := mustCreate(t, s, quote.New("A", "A", []string{"a"}))
	cid, _ := s.CreateCollection([]int64{q})
	g, _ := s.CreateCollectionGroup(cid, "group")
	_ = s.AddSuttaToCollection(cid, "mn1", "MN 1", "https://suttacentral.net/mn1")
	if err := s.SetQuoteGroup(cid, q, g); err != nil {
		t.Fatal(err)
	}
	if err := s.SetSuttaGroup(cid, "mn1", g); err != nil {
		t.Fatal(err)
	}

	if err := s.DeleteCollectionGroup(cid, g); err != nil {
		t.Fatal(err)
	}
	qi, _ := s.CollectionQuoteItems(cid)
	si, _ := s.CollectionSuttaItems(cid)
	if qi[0].GroupID != 0 {
		t.Errorf("quote group id = %d, want 0", qi[0].GroupID)
	}
	if si[0].GroupID != 0 {
		t.Errorf("sutta group id = %d, want 0", si[0].GroupID)
	}
}

func TestGroupCountsIncludeQuotesAndSuttas(t *testing.T) {
	s := newTestStore(t)
	q := mustCreate(t, s, quote.New("A", "A", []string{"a"}))
	cid, _ := s.CreateCollection([]int64{q})
	g, _ := s.CreateCollectionGroup(cid, "group")
	_ = s.AddSuttaToCollection(cid, "mn1", "MN 1", "https://suttacentral.net/mn1")
	if err := s.SetQuoteGroup(cid, q, g); err != nil {
		t.Fatal(err)
	}
	if err := s.SetSuttaGroup(cid, "mn1", g); err != nil {
		t.Fatal(err)
	}

	groups, _ := s.ListCollectionGroups(cid)
	if len(groups) != 1 || groups[0].Count != 2 {
		t.Errorf("group count = %+v, want 2 members", groups)
	}
}

func TestReorderCollectionSuttas(t *testing.T) {
	s := newTestStore(t)
	cid := mustCollection(t, s)
	for _, muid := range []string{"mn1", "mn2", "mn3"} {
		_ = s.AddSuttaToCollection(cid, muid, muid, muid)
	}

	if err := s.ReorderCollectionSuttas(cid, []string{"mn3", "mn1", "mn2"}); err != nil {
		t.Fatal(err)
	}
	items, _ := s.CollectionSuttaItems(cid)
	want := []string{"mn3", "mn1", "mn2"}
	for i, it := range items {
		if it.Muid != want[i] {
			t.Errorf("pos %d = %s, want %s", i, it.Muid, want[i])
		}
		if it.Position != i+1 {
			t.Errorf("pos %d position = %d, want %d", i, it.Position, i+1)
		}
	}
}

func TestReorderCollectionSuttasRejectsMismatch(t *testing.T) {
	s := newTestStore(t)
	cid := mustCollection(t, s)
	for _, muid := range []string{"mn1", "mn2"} {
		_ = s.AddSuttaToCollection(cid, muid, muid, muid)
	}
	if err := s.ReorderCollectionSuttas(cid, []string{"mn2"}); !errors.Is(err, ErrReorderMismatch) {
		t.Errorf("partial: err = %v, want ErrReorderMismatch", err)
	}
	if err := s.ReorderCollectionSuttas(cid, []string{"mn1", "mn1"}); !errors.Is(err, ErrReorderMismatch) {
		t.Errorf("duplicate: err = %v, want ErrReorderMismatch", err)
	}
	items, _ := s.CollectionSuttaItems(cid)
	if len(items) != 2 || items[0].Muid != "mn1" || items[1].Muid != "mn2" {
		t.Errorf("items after rejected reorders = %+v, want original order", items)
	}
}

func TestReorderCollectionSuttasNotFound(t *testing.T) {
	s := newTestStore(t)
	cid := mustCollection(t, s)
	_ = s.AddSuttaToCollection(cid, "mn1", "mn1", "mn1")

	if err := s.ReorderCollectionSuttas(999, []string{"mn1"}); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown collection: err = %v, want ErrNotFound", err)
	}
	err := s.ReorderCollectionSuttas(cid, []string{"mn1", "mn404"})
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown muid: err = %v, want ErrNotFound", err)
	}
	items, _ := s.CollectionSuttaItems(cid)
	if len(items) != 1 || items[0].Muid != "mn1" || items[0].Position != 1 {
		t.Errorf("partial reorder changed state: %+v", items)
	}
}

func TestDeleteCollectionRemovesGroupsAndSuttaItems(t *testing.T) {
	s := newTestStore(t)
	cid := mustCollection(t, s)
	_, _ = s.CreateCollectionGroup(cid, "group")
	_ = s.AddSuttaToCollection(cid, "mn1", "MN 1", "https://suttacentral.net/mn1")

	if err := s.DeleteCollection(cid); err != nil {
		t.Fatal(err)
	}
	var groups, items int
	_ = s.db.QueryRow("SELECT COUNT(*) FROM collection_groups").Scan(&groups)
	_ = s.db.QueryRow("SELECT COUNT(*) FROM collection_sutta_items").Scan(&items)
	if groups != 0 || items != 0 {
		t.Errorf("after delete: groups %d items %d, want 0 0", groups, items)
	}
	suttas, _ := s.ListSuttas()
	if len(suttas) != 1 || suttas[0].Muid != "mn1" {
		t.Errorf("registry after collection delete = %+v, want mn1 kept", suttas)
	}
}

func TestSuttaLengthsDefaultUnknownAndSet(t *testing.T) {
	s := newTestStore(t)
	cid := mustCollection(t, s)
	_ = s.AddSuttaToCollection(cid, "mn1", "MN 1", "https://suttacentral.net/mn1")
	_ = s.AddSuttaToCollection(cid, "mn2", "MN 2", "https://suttacentral.net/mn2")

	suttas, _ := s.ListSuttas()
	if suttas[0].Length != 0 || suttas[1].Length != 0 {
		t.Errorf("default lengths = %+v, want unknown (0)", suttas)
	}
	if err := s.SetSuttaLengths(map[string]int{"mn1": 312, "mn2": 45}); err != nil {
		t.Fatal(err)
	}
	suttas, _ = s.ListSuttas()
	if suttas[0].Length != 312 || suttas[1].Length != 45 {
		t.Errorf("lengths after set = %+v", suttas)
	}
	items, _ := s.CollectionSuttaItems(cid)
	if items[0].Length != 312 || items[1].Length != 45 {
		t.Errorf("item lengths = %+v", items)
	}
}

func TestSetSuttaLengthsIgnoresUnknownMuids(t *testing.T) {
	s := newTestStore(t)
	cid := mustCollection(t, s)
	_ = s.AddSuttaToCollection(cid, "mn1", "MN 1", "https://suttacentral.net/mn1")

	if err := s.SetSuttaLengths(map[string]int{"mn404": 10, "": 3}); err != nil {
		t.Fatalf("unknown muid: %v", err)
	}
	suttas, _ := s.ListSuttas()
	if suttas[0].Length != 0 {
		t.Errorf("length = %d, want 0", suttas[0].Length)
	}
}

func TestSortShortestFirst(t *testing.T) {
	items := []SuttaItem{
		{Muid: "unknown-early", Position: 2, Length: 0},
		{Muid: "long", Position: 3, Length: 900},
		{Muid: "short", Position: 4, Length: 10},
		{Muid: "mid", Position: 1, Length: 100},
		{Muid: "tie-a", Position: 6, Length: 100},
		{Muid: "tie-b", Position: 5, Length: 100},
		{Muid: "unknown-late", Position: 7, Length: 0},
	}
	SortShortestFirst(items)
	want := []string{"short", "mid", "tie-b", "tie-a", "long", "unknown-early", "unknown-late"}
	if len(items) != len(want) {
		t.Fatalf("sorted = %v, want %v", items, want)
	}
	for i, it := range items {
		if it.Muid != want[i] {
			t.Errorf("pos %d = %s, want %s", i, it.Muid, want[i])
		}
	}
}

func TestSortShortestFirstEmptyAndNil(t *testing.T) {
	SortShortestFirst(nil)
	SortShortestFirst([]SuttaItem{})
}
