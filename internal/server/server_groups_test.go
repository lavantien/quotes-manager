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

func TestGrouplessCollectionRendersSuttas(t *testing.T) {
	fs, cid := fakeWithCollection(t)
	if err := fs.AddSuttaToCollection(cid, "mn26", "MN 26", "https://suttacentral.net/mn26"); err != nil {
		t.Fatal(err)
	}
	srv := newServer(t, fs)
	body := do(t, srv, "GET", fmt.Sprintf("/collections/%d", cid), "").Body.String()
	s0 := sectionSlice(t, body, `id="group-0"`, "")
	if !strings.Contains(s0, ">MN 26<") {
		t.Error("groupless collection must render sutta display ids in the ungrouped section")
	}
	if !strings.Contains(s0, "https://suttacentral.net/mn26") {
		t.Error("groupless collection must link its suttas")
	}
	if !strings.Contains(s0, `hx-delete="/collections/`) {
		t.Error("groupless sutta rows must keep the remove control")
	}
}

const (
	formCT  = "Content-Type"
	formEnc = "application/x-www-form-urlencoded"
)

func TestCreateGroupHandler(t *testing.T) {
	fs, cid, _, _ := groupedFake(t)
	srv := newServer(t, fs)
	rec := do(t, srv, "POST", fmt.Sprintf("/collections/%d/groups", cid), "name=clear comprehension", formCT, formEnc)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `id="collection-zone"`) {
		t.Error("create group must swap the collection zone")
	}
	if !strings.Contains(rec.Body.String(), "clear comprehension") {
		t.Error("zone must render the new group")
	}
	groups, _ := fs.ListCollectionGroups(cid)
	found := false
	for _, g := range groups {
		if g.Name == "clear comprehension" {
			found = true
		}
	}
	if !found {
		t.Errorf("groups = %+v, want clear comprehension stored", groups)
	}
}

func TestCreateGroupEmptyName(t *testing.T) {
	fs, cid, _, _ := groupedFake(t)
	srv := newServer(t, fs)
	rec := do(t, srv, "POST", fmt.Sprintf("/collections/%d/groups", cid), "name=%20%20", formCT, formEnc)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestCreateGroupRejectsControlCharacters(t *testing.T) {
	fs, cid, _, _ := groupedFake(t)
	srv := newServer(t, fs)
	rec := do(t, srv, "POST", fmt.Sprintf("/collections/%d/groups", cid), "name=ab%00c", formCT, formEnc)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
	rec = do(t, srv, "POST", fmt.Sprintf("/collections/%d/groups", cid), "name=ab%0Ac", formCT, formEnc)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("newline name status = %d, want 400", rec.Code)
	}
	groups, _ := fs.ListCollectionGroups(cid)
	if len(groups) != 2 {
		t.Errorf("groups = %+v, want only the 2 seeded ones", groups)
	}
}

func TestCreateGroupUnknownCollection(t *testing.T) {
	srv := newServer(t, newFake(sampleQuote(1)))
	rec := do(t, srv, "POST", "/collections/999/groups", "name=x", formCT, formEnc)
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestRenameGroupViaPromptHeader(t *testing.T) {
	fs, cid, g1, _ := groupedFake(t)
	srv := newServer(t, fs)
	rec := do(t, srv, "POST", fmt.Sprintf("/collections/%d/groups/%d/rename", cid, g1), "", "HX-Prompt", "wise%20attention")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	groups, _ := fs.ListCollectionGroups(cid)
	if groups[0].Name != "wise attention" {
		t.Errorf("name = %q, want wise attention", groups[0].Name)
	}
}

func TestRenameGroupViaFormField(t *testing.T) {
	fs, cid, g1, _ := groupedFake(t)
	srv := newServer(t, fs)
	rec := do(t, srv, "POST", fmt.Sprintf("/collections/%d/groups/%d/rename", cid, g1), "name=renamed", formCT, formEnc)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	groups, _ := fs.ListCollectionGroups(cid)
	if groups[0].Name != "renamed" {
		t.Errorf("name = %q, want renamed", groups[0].Name)
	}
}

func TestRenameGroupEmptyPrompt(t *testing.T) {
	fs, cid, g1, _ := groupedFake(t)
	srv := newServer(t, fs)
	rec := do(t, srv, "POST", fmt.Sprintf("/collections/%d/groups/%d/rename", cid, g1), "", "HX-Prompt", "%20%20")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestRenameGroupUnknownGroup(t *testing.T) {
	fs, cid, _, _ := groupedFake(t)
	srv := newServer(t, fs)
	rec := do(t, srv, "POST", fmt.Sprintf("/collections/%d/groups/999/rename", cid), "name=x", formCT, formEnc)
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestDeleteGroupHandler(t *testing.T) {
	fs, cid, g1, _ := groupedFake(t)
	srv := newServer(t, fs)
	rec := do(t, srv, "DELETE", fmt.Sprintf("/collections/%d/groups/%d", cid, g1), "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	groups, _ := fs.ListCollectionGroups(cid)
	if len(groups) != 1 {
		t.Fatalf("groups after delete = %+v, want 1", groups)
	}
	qi, _ := fs.CollectionQuoteItems(cid)
	for _, it := range qi {
		if it.GroupID == g1 {
			t.Errorf("quote %d still references the deleted group", it.ID)
		}
	}
	si, _ := fs.CollectionSuttaItems(cid)
	for _, it := range si {
		if it.GroupID == g1 {
			t.Errorf("sutta %s still references the deleted group", it.Muid)
		}
	}
}

func TestDeleteGroupUnknown(t *testing.T) {
	fs, cid, _, _ := groupedFake(t)
	srv := newServer(t, fs)
	rec := do(t, srv, "DELETE", fmt.Sprintf("/collections/%d/groups/999", cid), "")
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestMoveGroupUp(t *testing.T) {
	fs, cid, _, g2 := groupedFake(t)
	srv := newServer(t, fs)
	rec := do(t, srv, "POST", fmt.Sprintf("/collections/%d/groups/%d/move?dir=up", cid, g2), "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	groups, _ := fs.ListCollectionGroups(cid)
	if len(groups) != 2 || groups[0].ID != g2 {
		t.Errorf("groups after move = %+v, want g2 first", groups)
	}
	if !strings.Contains(rec.Body.String(), "wakefulness") {
		t.Error("move must return the refreshed zone")
	}
}

func TestMoveGroupUpAtTopIsNoOp(t *testing.T) {
	fs, cid, g1, _ := groupedFake(t)
	srv := newServer(t, fs)
	rec := do(t, srv, "POST", fmt.Sprintf("/collections/%d/groups/%d/move?dir=up", cid, g1), "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	groups, _ := fs.ListCollectionGroups(cid)
	if groups[0].ID != g1 || len(groups) != 2 {
		t.Errorf("top move changed order: %+v", groups)
	}
}

func TestMoveGroupDown(t *testing.T) {
	fs, cid, g1, g2 := groupedFake(t)
	srv := newServer(t, fs)
	rec := do(t, srv, "POST", fmt.Sprintf("/collections/%d/groups/%d/move?dir=down", cid, g1), "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	groups, _ := fs.ListCollectionGroups(cid)
	if len(groups) != 2 || groups[0].ID != g2 || groups[1].ID != g1 {
		t.Errorf("groups after move down = %+v, want g2 then g1", groups)
	}
}

func TestMoveGroupDownAtBottomIsNoOp(t *testing.T) {
	fs, cid, g1, g2 := groupedFake(t)
	srv := newServer(t, fs)
	rec := do(t, srv, "POST", fmt.Sprintf("/collections/%d/groups/%d/move?dir=down", cid, g2), "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	groups, _ := fs.ListCollectionGroups(cid)
	if groups[0].ID != g1 || groups[1].ID != g2 {
		t.Errorf("bottom move changed order: %+v", groups)
	}
}

func TestMoveGroupDirViaFormField(t *testing.T) {
	fs, cid, g1, g2 := groupedFake(t)
	srv := newServer(t, fs)
	rec := do(t, srv, "POST", fmt.Sprintf("/collections/%d/groups/%d/move", cid, g1), "dir=down", formCT, formEnc)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	groups, _ := fs.ListCollectionGroups(cid)
	if len(groups) != 2 || groups[0].ID != g2 {
		t.Errorf("groups after form-field move = %+v, want g2 first", groups)
	}
}

func TestMoveGroupMissingDir(t *testing.T) {
	fs, cid, _, g2 := groupedFake(t)
	srv := newServer(t, fs)
	rec := do(t, srv, "POST", fmt.Sprintf("/collections/%d/groups/%d/move", cid, g2), "")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestGroupedViewCountsSumQuotesAndSuttas(t *testing.T) {
	fs, cid, _, _ := groupedFake(t)
	srv := newServer(t, fs)
	body := do(t, srv, "GET", fmt.Sprintf("/collections/%d", cid), "").Body.String()
	if !strings.Contains(body, "6 items") {
		t.Error("collection count must sum the 2 quotes and 4 suttas")
	}
}

func TestUngroupedCountPillSumsQuotesAndSuttas(t *testing.T) {
	fs, cid := fakeWithCollection(t)
	g1, err := fs.CreateCollectionGroup(cid, "only")
	if err != nil {
		t.Fatal(err)
	}
	if err := fs.SetQuoteGroup(cid, 1, g1); err != nil {
		t.Fatal(err)
	}
	for _, muid := range []string{"mn10", "mn5"} {
		if err := fs.AddSuttaToCollection(cid, muid, muid, "https://suttacentral.net/"+muid); err != nil {
			t.Fatal(err)
		}
	}
	srv := newServer(t, fs)
	body := do(t, srv, "GET", fmt.Sprintf("/collections/%d", cid), "").Body.String()
	s0 := sectionSlice(t, body, `id="group-0"`, "")
	if !strings.Contains(s0, `<span class="zone__count">3</span>`) {
		t.Errorf("ungrouped count pill must sum its quote and 2 suttas, got %s", clipSection(s0))
	}
}

func clipSection(s string) string {
	if len(s) > 400 {
		return s[:400]
	}
	return s
}

func TestMoveGroupBadDir(t *testing.T) {
	fs, cid, _, g2 := groupedFake(t)
	srv := newServer(t, fs)
	rec := do(t, srv, "POST", fmt.Sprintf("/collections/%d/groups/%d/move?dir=sideways", cid, g2), "")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestMoveGroupUnknownGroup(t *testing.T) {
	fs, cid, _, _ := groupedFake(t)
	srv := newServer(t, fs)
	rec := do(t, srv, "POST", fmt.Sprintf("/collections/%d/groups/999/move?dir=up", cid), "")
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestSetQuoteGroupHandler(t *testing.T) {
	fs, cid, g1, _ := groupedFake(t)
	srv := newServer(t, fs)
	rec := do(t, srv, "POST", fmt.Sprintf("/collections/%d/items/2/group", cid), fmt.Sprintf("group=%d", g1), formCT, formEnc)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	qi, _ := fs.CollectionQuoteItems(cid)
	for _, it := range qi {
		if it.ID == 2 && it.GroupID != g1 {
			t.Errorf("quote 2 group = %d, want %d", it.GroupID, g1)
		}
	}
	rec = do(t, srv, "POST", fmt.Sprintf("/collections/%d/items/2/group", cid), "group=0", formCT, formEnc)
	if rec.Code != http.StatusOK {
		t.Fatalf("clear status = %d", rec.Code)
	}
	qi, _ = fs.CollectionQuoteItems(cid)
	for _, it := range qi {
		if it.ID == 2 && it.GroupID != 0 {
			t.Errorf("quote 2 group after clear = %d, want 0", it.GroupID)
		}
	}
}

func TestSetQuoteGroupUnknownGroup(t *testing.T) {
	fs, cid, _, _ := groupedFake(t)
	srv := newServer(t, fs)
	rec := do(t, srv, "POST", fmt.Sprintf("/collections/%d/items/2/group", cid), "group=999", formCT, formEnc)
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestSetQuoteGroupNonMember(t *testing.T) {
	fs, cid, g1, _ := groupedFake(t)
	srv := newServer(t, fs)
	rec := do(t, srv, "POST", fmt.Sprintf("/collections/%d/items/3/group", cid), fmt.Sprintf("group=%d", g1), formCT, formEnc)
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestSetQuoteGroupBadPayload(t *testing.T) {
	fs, cid, _, _ := groupedFake(t)
	srv := newServer(t, fs)
	rec := do(t, srv, "POST", fmt.Sprintf("/collections/%d/items/2/group", cid), "group=abc", formCT, formEnc)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestAddSuttaHandler(t *testing.T) {
	fs, cid, _, _ := groupedFake(t)
	srv := newServer(t, fs)
	rec := do(t, srv, "POST", fmt.Sprintf("/collections/%d/suttas", cid), "muid=MN 7", formCT, formEnc)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, ">MN 7<") {
		t.Error("zone must render the added sutta with its display id")
	}
	if !strings.Contains(body, "https://suttacentral.net/mn7/") {
		t.Error("added sutta must link its canonical page")
	}
	if !strings.Contains(body, `id="right-rail"`) {
		t.Error("add sutta must refresh the rail counts out of band")
	}
	items, _ := fs.CollectionSuttaItems(cid)
	found := false
	for _, it := range items {
		if it.Muid == "mn7" {
			found = true
		}
	}
	if !found {
		t.Errorf("items = %+v, want mn7 membership", items)
	}
	suttas, _ := fs.ListSuttas()
	for _, su := range suttas {
		if su.Muid == "mn7" && su.Display != "MN 7" {
			t.Errorf("registry display = %q, want MN 7", su.Display)
		}
	}
}

func TestAddSuttaInvalidMuid(t *testing.T) {
	fs, cid, _, _ := groupedFake(t)
	srv := newServer(t, fs)
	rec := do(t, srv, "POST", fmt.Sprintf("/collections/%d/suttas", cid), "muid=%21%21%21", formCT, formEnc)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestAddSuttaEmptyMuid(t *testing.T) {
	fs, cid, _, _ := groupedFake(t)
	srv := newServer(t, fs)
	rec := do(t, srv, "POST", fmt.Sprintf("/collections/%d/suttas", cid), "muid=%20", formCT, formEnc)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestAddSuttaUnknownCollection(t *testing.T) {
	srv := newServer(t, newFake(sampleQuote(1)))
	rec := do(t, srv, "POST", "/collections/999/suttas", "muid=mn7", formCT, formEnc)
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestRemoveSuttaHandler(t *testing.T) {
	fs, cid, _, _ := groupedFake(t)
	srv := newServer(t, fs)
	rec := do(t, srv, "DELETE", fmt.Sprintf("/collections/%d/suttas/mn5", cid), "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	items, _ := fs.CollectionSuttaItems(cid)
	for _, it := range items {
		if it.Muid == "mn5" {
			t.Error("mn5 membership must be gone")
		}
	}
	suttas, _ := fs.ListSuttas()
	found := false
	for _, su := range suttas {
		if su.Muid == "mn5" {
			found = true
		}
	}
	if !found {
		t.Error("registry must keep mn5 after removal")
	}
}

func TestRemoveSuttaUnknown(t *testing.T) {
	fs, cid, _, _ := groupedFake(t)
	srv := newServer(t, fs)
	rec := do(t, srv, "DELETE", fmt.Sprintf("/collections/%d/suttas/mn404", cid), "")
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestSetSuttaGroupHandler(t *testing.T) {
	fs, cid, g2, _ := groupedFake(t)
	srv := newServer(t, fs)
	rec := do(t, srv, "POST", fmt.Sprintf("/collections/%d/suttas/mn5/group", cid), fmt.Sprintf("group=%d", g2), formCT, formEnc)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	items, _ := fs.CollectionSuttaItems(cid)
	for _, it := range items {
		if it.Muid == "mn5" && it.GroupID != g2 {
			t.Errorf("mn5 group = %d, want %d", it.GroupID, g2)
		}
	}
}

func TestSetSuttaGroupUnknownGroup(t *testing.T) {
	fs, cid, _, _ := groupedFake(t)
	srv := newServer(t, fs)
	rec := do(t, srv, "POST", fmt.Sprintf("/collections/%d/suttas/mn5/group", cid), "group=999", formCT, formEnc)
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestSetSuttaGroupNonMember(t *testing.T) {
	fs, cid, g1, _ := groupedFake(t)
	srv := newServer(t, fs)
	rec := do(t, srv, "POST", fmt.Sprintf("/collections/%d/suttas/mn404/group", cid), fmt.Sprintf("group=%d", g1), formCT, formEnc)
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestGroupMutationsStoreError(t *testing.T) {
	assert500(t, newServer(t, failingStore{}), "POST", "/collections/1/groups", "name=x", formCT, formEnc)
	assert500(t, newServer(t, failingStore{}), "DELETE", "/collections/1/groups/1", "")
	assert500(t, newServer(t, failingStore{}), "POST", "/collections/1/suttas", "muid=mn7", formCT, formEnc)
}
