package server_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func groupedFake(t *testing.T) (*fakeStore, int64, int64, int64) {
	t.Helper()
	fs, cid := fakeWithCollection(t)
	g1, err := fs.CreateCollectionGroup(cid, "senses restraint")
	if err != nil {
		t.Fatal(err)
	}
	g2, err := fs.CreateCollectionGroup(cid, "wakefulness")
	if err != nil {
		t.Fatal(err)
	}
	if err := fs.SetQuoteGroup(cid, 1, g1); err != nil {
		t.Fatal(err)
	}
	for _, su := range []struct {
		muid, display, link string
	}{
		{"mn10", "MN 10", "https://suttacentral.net/mn10"},
		{"mn5", "MN 5", "https://suttacentral.net/mn5"},
		{"vv51", "Vv 51", "https://suttacentral.net/vv51"},
	} {
		if err := fs.AddSuttaToCollection(cid, su.muid, su.display, su.link); err != nil {
			t.Fatal(err)
		}
		if err := fs.SetSuttaGroup(cid, su.muid, g1); err != nil {
			t.Fatal(err)
		}
	}
	if err := fs.AddSuttaToCollection(cid, "mn2", "MN 2", "https://suttacentral.net/mn2"); err != nil {
		t.Fatal(err)
	}
	if err := fs.SetSuttaGroup(cid, "mn2", g2); err != nil {
		t.Fatal(err)
	}
	if err := fs.SetSuttaLengths(map[string]int{"mn10": 900, "mn5": 50, "mn2": 10}); err != nil {
		t.Fatal(err)
	}
	return fs, cid, g1, g2
}

func TestGroupedCollectionViewSectionsInOrder(t *testing.T) {
	fs, cid, g1, g2 := groupedFake(t)
	srv := newServer(t, fs)
	rec := do(t, srv, "GET", fmt.Sprintf("/collections/%d", cid), "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	body := rec.Body.String()
	i1 := strings.Index(body, fmt.Sprintf(`id="group-%d"`, g1))
	i2 := strings.Index(body, fmt.Sprintf(`id="group-%d"`, g2))
	i0 := strings.Index(body, `id="group-0"`)
	if i1 < 0 || i2 < 0 || i0 < 0 {
		t.Fatalf("group sections missing: g1=%d g2=%d ungrouped=%d", i1, i2, i0)
	}
	if i1 >= i2 || i2 >= i0 {
		t.Errorf("section order = g1 %d, g2 %d, ungrouped %d, want g1 < g2 < ungrouped", i1, i2, i0)
	}
	for _, name := range []string{"senses restraint", "wakefulness", "Ungrouped"} {
		if !strings.Contains(body, name) {
			t.Errorf("group name %q missing", name)
		}
	}
}

func sectionSlice(t *testing.T, body, openMarker, closeMarker string) string {
	t.Helper()
	start := strings.Index(body, openMarker)
	if start < 0 {
		t.Fatalf("section %s missing from body", openMarker)
	}
	if closeMarker == "" {
		return body[start:]
	}
	end := strings.Index(body[start:], closeMarker)
	if end < 0 {
		t.Fatalf("section end %s missing after %s", closeMarker, openMarker)
	}
	return body[start : start+end]
}

func TestGroupedViewAssignsItemsToTheirSections(t *testing.T) {
	fs, cid, g1, g2 := groupedFake(t)
	srv := newServer(t, fs)
	body := do(t, srv, "GET", fmt.Sprintf("/collections/%d", cid), "").Body.String()
	s1 := sectionSlice(t, body, fmt.Sprintf(`id="group-%d"`, g1), fmt.Sprintf(`id="group-%d"`, g2))
	if !strings.Contains(s1, `id="col-quote-1"`) {
		t.Error("quote 1 must render inside its group section")
	}
	if strings.Contains(s1, `id="col-quote-2"`) {
		t.Error("quote 2 is ungrouped, must not render in group 1")
	}
	s0 := sectionSlice(t, body, `id="group-0"`, "")
	if !strings.Contains(s0, `id="col-quote-2"`) {
		t.Error("ungrouped quote 2 must render in the ungrouped section")
	}
}

func TestGroupedSuttaRowsShortestFirstWithLinksAndLengths(t *testing.T) {
	fs, cid, g1, g2 := groupedFake(t)
	srv := newServer(t, fs)
	body := do(t, srv, "GET", fmt.Sprintf("/collections/%d", cid), "").Body.String()
	s1 := sectionSlice(t, body, fmt.Sprintf(`id="group-%d"`, g1), fmt.Sprintf(`id="group-%d"`, g2))
	imn5 := strings.Index(s1, "MN 5")
	imn10 := strings.Index(s1, "MN 10")
	ivv := strings.Index(s1, "Vv 51")
	if imn5 < 0 || imn10 < 0 || ivv < 0 {
		t.Fatalf("sutta rows missing: mn5=%d mn10=%d vv=%d", imn5, imn10, ivv)
	}
	if imn5 >= imn10 || imn10 >= ivv {
		t.Errorf("shortest-first order = mn5 %d, mn10 %d, vv %d, want ascending then unknown", imn5, imn10, ivv)
	}
	for _, link := range []string{"https://suttacentral.net/mn5", "https://suttacentral.net/mn10", "https://suttacentral.net/vv51"} {
		if !strings.Contains(s1, link) {
			t.Errorf("sutta link %s missing", link)
		}
	}
	if !strings.Contains(s1, ">50 seg<") || !strings.Contains(s1, ">900 seg<") {
		t.Error("known lengths must render a segment count")
	}
	if strings.Contains(s1, ">0 seg<") {
		t.Error("unknown length must not render a zero count")
	}
}

func TestGroupedViewControls(t *testing.T) {
	fs, cid, g1, g2 := groupedFake(t)
	srv := newServer(t, fs)
	body := do(t, srv, "GET", fmt.Sprintf("/collections/%d", cid), "").Body.String()
	for _, want := range []string{
		fmt.Sprintf(`hx-post="/collections/%d/groups"`, cid),
		fmt.Sprintf(`hx-post="/collections/%d/groups/%d/rename"`, cid, g1),
		fmt.Sprintf(`hx-delete="/collections/%d/groups/%d"`, cid, g2),
		fmt.Sprintf(`hx-post="/collections/%d/groups/%d/move?dir=up"`, cid, g1),
		fmt.Sprintf(`hx-post="/collections/%d/groups/%d/move?dir=down"`, cid, g1),
		fmt.Sprintf(`hx-post="/collections/%d/items/1/group"`, cid),
		fmt.Sprintf(`hx-post="/collections/%d/suttas/mn5/group"`, cid),
		fmt.Sprintf(`hx-delete="/collections/%d/suttas/mn5"`, cid),
		fmt.Sprintf(`hx-post="/collections/%d/suttas"`, cid),
	} {
		if !strings.Contains(body, want) {
			t.Errorf("missing control %s", want)
		}
	}
	if !strings.Contains(body, `data-reorder="/collections/`) {
		t.Error("grouped view must keep the drag-reorder affordance")
	}
	if !strings.Contains(body, `draggable="true"`) {
		t.Error("grouped view must keep draggable quote blocks")
	}
}

func TestGroupedViewGroupSelectMarksCurrentGroup(t *testing.T) {
	fs, cid, g1, _ := groupedFake(t)
	srv := newServer(t, fs)
	body := do(t, srv, "GET", fmt.Sprintf("/collections/%d", cid), "").Body.String()
	sel := sectionSlice(t, body, fmt.Sprintf(`id="group-%d"`, g1), "</select>")
	if !strings.Contains(sel, fmt.Sprintf(`<option value="%d" selected>senses restraint</option>`, g1)) {
		t.Errorf("quote 1 select must mark its current group selected, got %q", sel)
	}
	if !strings.Contains(body, `<option value="0">Ungrouped</option>`) {
		t.Error("group selects must offer the ungrouped option")
	}
}

func TestGroupedViewUngroupedCarriesInsertGaps(t *testing.T) {
	fs, cid, _, _ := groupedFake(t)
	srv := newServer(t, fs)
	body := do(t, srv, "GET", fmt.Sprintf("/collections/%d", cid), "").Body.String()
	s0 := sectionSlice(t, body, `id="group-0"`, "")
	for _, want := range []string{`data-pos="2"`, `data-pos="3"`} {
		if !strings.Contains(s0, want) {
			t.Errorf("ungrouped section missing insert gap %s", want)
		}
	}
	if strings.Contains(s0, `data-pos="1"`) {
		t.Error("ungrouped section must not carry a gap before grouped positions")
	}
}

func TestFlatViewWithoutGroupsUnchanged(t *testing.T) {
	fs, cid := fakeWithCollection(t)
	srv := newServer(t, fs)
	body := do(t, srv, "GET", fmt.Sprintf("/collections/%d", cid), "").Body.String()
	if strings.Contains(body, `id="group-`) {
		t.Error("collection without groups must not render group sections")
	}
	if !strings.Contains(body, `data-pos="1"`) || !strings.Contains(body, `data-pos="3"`) {
		t.Error("flat view must keep its insert gaps")
	}
	if !strings.Contains(body, `draggable="true"`) {
		t.Error("flat view must keep draggable blocks")
	}
}
