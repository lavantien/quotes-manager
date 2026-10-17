package server_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func TestCollectionViewShowsBlockCount(t *testing.T) {
	fs, cid := fakeWithCollection(t)
	srv := newServer(t, fs)
	rec := do(t, srv, "GET", fmt.Sprintf("/collections/%d", cid), "")
	if !strings.Contains(rec.Body.String(), "2 blocks") {
		t.Error("collection view missing block count badge")
	}
}

func fakeWithCollection(t *testing.T) (*fakeStore, int64) {
	t.Helper()
	fs := newFake(sampleQuote(1), sampleQuote(2), sampleQuote(3))
	cid, err := fs.CreateCollection([]int64{1, 2})
	if err != nil {
		t.Fatal(err)
	}
	return fs, cid
}

func TestCreateCollectionFromSelection(t *testing.T) {
	fs := newFake(sampleQuote(1), sampleQuote(2))
	srv := newServer(t, fs)
	rec := do(t, srv, "POST", "/collections", "id=1&id=2", "Content-Type", "application/x-www-form-urlencoded")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}

	if loc := rec.Header().Get("HX-Redirect"); loc != "" {
		t.Errorf("create should swap in place, got HX-Redirect = %q", loc)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `id="collection-zone"`) {
		t.Error("create should return the active collection zone")
	}
	if !strings.Contains(body, "Human beings are shady") {
		t.Error("new collection should contain the selected quotes")
	}
	if len(fs.collections) != 1 || fs.collections[0].Count != 2 {
		t.Errorf("collections = %+v", fs.collections)
	}
}

func TestAddToCollectionItems(t *testing.T) {
	fs, cid := fakeWithCollection(t)
	srv := newServer(t, fs)
	rec := do(t, srv, "POST", fmt.Sprintf("/collections/%d/items", cid), "id=3",
		"Content-Type", "application/x-www-form-urlencoded")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if loc := rec.Header().Get("HX-Redirect"); loc != fmt.Sprintf("/?col=%d", cid) {
		t.Errorf("HX-Redirect = %q, want /?col=%d", loc, cid)
	}

	qs, _ := fs.CollectionQuotes(cid)
	if len(qs) != 3 || qs[0].ID != 3 || qs[1].ID != 1 || qs[2].ID != 2 {
		t.Errorf("after add = %+v", qs)
	}
}

func TestAddToCollectionItemsUnknownCollection(t *testing.T) {
	srv := newServer(t, newFake(sampleQuote(1)))
	rec := do(t, srv, "POST", "/collections/999/items", "id=1",
		"Content-Type", "application/x-www-form-urlencoded")
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestCollectionView(t *testing.T) {
	fs, cid := fakeWithCollection(t)
	srv := newServer(t, fs)
	rec := do(t, srv, "GET", fmt.Sprintf("/collections/%d", cid), "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Human beings are shady") {
		t.Error("collection view missing its quotes")
	}

	if !strings.Contains(body, `id="collection-list"`) {
		t.Error("collection zone should render the collection list")
	}
	if !strings.Contains(body, `id="col-quote-`) {
		t.Error("collection blocks should be read-only (col-quote- ids)")
	}
	if !strings.Contains(body, `draggable="true"`) {
		t.Error("collection blocks should be draggable to reorder")
	}
	if !strings.Contains(body, `data-action="copy"`) {
		t.Error("collection blocks should be copyable")
	}
	if !strings.Contains(body, fmt.Sprintf(`hx-delete="/collections/%d"`, cid)) {
		t.Error("collection zone should expose a delete-collection button")
	}
}

func TestCollectionReorder(t *testing.T) {
	fs, cid := fakeWithCollection(t)
	srv := newServer(t, fs)
	rec := do(t, srv, "POST", fmt.Sprintf("/collections/%d/reorder", cid),
		`{"ids":[2,1]}`, "Content-Type", "application/json")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	got, _ := fs.CollectionQuotes(cid)
	if len(got) != 2 || got[0].ID != 2 || got[1].ID != 1 {
		t.Errorf("after reorder = %+v", got)
	}
}

func TestCollectionExport(t *testing.T) {
	fs, cid := fakeWithCollection(t)
	srv := newServer(t, fs)
	rec := do(t, srv, "GET", fmt.Sprintf("/collections/%d/export.txt", cid), "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), ".  \n.  \n.") {
		t.Error("collection export missing dot separator")
	}
}

func TestDeleteCollection(t *testing.T) {
	fs, cid := fakeWithCollection(t)
	srv := newServer(t, fs)
	rec := do(t, srv, "DELETE", fmt.Sprintf("/collections/%d", cid), "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if rec.Header().Get("HX-Redirect") != "/" {
		t.Errorf("HX-Redirect = %q", rec.Header().Get("HX-Redirect"))
	}
	if len(fs.collections) != 0 {
		t.Errorf("collection not deleted: %+v", fs.collections)
	}
}

func TestCollectionNotFound(t *testing.T) {
	srv := newServer(t, newFake(sampleQuote(1)))
	rec := do(t, srv, "GET", "/collections/999", "")
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestRailRightListsCollections(t *testing.T) {
	fs, cid := fakeWithCollection(t)
	srv := newServer(t, fs)
	rec := do(t, srv, "GET", "/rail/right", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `id="right-rail"`) {
		t.Error("right rail fragment missing root element")
	}
	if !strings.Contains(body, "Collections") {
		t.Error("right rail missing Collections heading")
	}
	if !strings.Contains(body, fmt.Sprintf("Col %d", cid)) {
		t.Error("right rail missing collection link")
	}
}

func TestRenameCollectionHandler(t *testing.T) {
	fs, cid := fakeWithCollection(t)
	srv := newServer(t, fs)
	rec := do(t, srv, "POST", fmt.Sprintf("/collections/%d/rename", cid), "name=Keepsakes",
		"Content-Type", "application/x-www-form-urlencoded")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Keepsakes") {
		t.Error("rename response should show the new name in the rail")
	}
	got, _ := fs.GetCollection(cid)
	if got.Name != "Keepsakes" {
		t.Errorf("collection name = %q, want Keepsakes", got.Name)
	}
}

func TestRenameCollectionUnknown(t *testing.T) {
	srv := newServer(t, newFake(sampleQuote(1)))
	rec := do(t, srv, "POST", "/collections/999/rename", "name=x",
		"Content-Type", "application/x-www-form-urlencoded")
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestInsertCollectionItemsHandler(t *testing.T) {
	fs, cid := fakeWithCollection(t)
	srv := newServer(t, fs)
	rec := do(t, srv, "POST", fmt.Sprintf("/collections/%d/insert", cid), "id=3&pos=2",
		"Content-Type", "application/x-www-form-urlencoded")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}

	qs, _ := fs.CollectionQuotes(cid)
	want := []int64{1, 3, 2}
	if len(qs) != len(want) {
		t.Fatalf("len = %d, want %d (%+v)", len(qs), len(want), qs)
	}
	for i, q := range qs {
		if q.ID != want[i] {
			t.Errorf("pos %d = %d, want %d", i, q.ID, want[i])
		}
	}
	if !strings.Contains(rec.Body.String(), `id="collection-zone"`) {
		t.Error("insert should return the refreshed collection zone")
	}
}

func TestInsertCollectionItemsUnknown(t *testing.T) {
	srv := newServer(t, newFake(sampleQuote(1)))
	rec := do(t, srv, "POST", "/collections/999/insert", "id=1&pos=1",
		"Content-Type", "application/x-www-form-urlencoded")
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestCollectionPaneFragment(t *testing.T) {
	fs, cid := fakeWithCollection(t)
	srv := newServer(t, fs)
	rec := do(t, srv, "GET", fmt.Sprintf("/pane/collection?col=%d", cid), "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `id="collection-zone"`) {
		t.Error("pane should render the collection zone")
	}
	if !strings.Contains(body, "Human beings are shady") {
		t.Error("collection pane should show the collection's quotes")
	}
	if !strings.Contains(body, "insert-gap") {
		t.Error("collection pane should render insert-gap affordances")
	}
}

func TestCollectionPaneEmpty(t *testing.T) {
	srv := newServer(t, newFake(sampleQuote(1)))
	rec := do(t, srv, "GET", "/pane/collection", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `id="collection-zone"`) {
		t.Error("empty pane should still render the collection zone")
	}
	if !strings.Contains(body, "No collection selected") {
		t.Error("empty pane should show the placeholder")
	}
}

func TestRootBlockShowsCollectionChips(t *testing.T) {
	fs, cid := fakeWithCollection(t)
	srv := newServer(t, fs)
	rec := do(t, srv, "GET", "/", "")
	body := rec.Body.String()
	if !strings.Contains(body, `class="collection-chip"`) {
		t.Error("root blocks should show collection-membership chips")
	}
	if !strings.Contains(body, fmt.Sprintf(`col=%d`, cid)) {
		t.Error("collection chip should target the collection pane")
	}
}

func TestCollectionReorderBadJSON(t *testing.T) {
	fs, cid := fakeWithCollection(t)
	srv := newServer(t, fs)
	rec := do(t, srv, "POST", fmt.Sprintf("/collections/%d/reorder", cid),
		`{not json`, "Content-Type", "application/json")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestCollectionReorderUnknownCollection(t *testing.T) {
	srv := newServer(t, newFake(sampleQuote(1)))
	rec := do(t, srv, "POST", "/collections/999/reorder", `{"ids":[1]}`, "Content-Type", "application/json")
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestInsertCollectionItemsNonNumericPos(t *testing.T) {
	fs, cid := fakeWithCollection(t)
	srv := newServer(t, fs)

	rec := do(t, srv, "POST", fmt.Sprintf("/collections/%d/insert", cid), "id=3&pos=abc",
		"Content-Type", "application/x-www-form-urlencoded")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	qs, _ := fs.CollectionQuotes(cid)
	if len(qs) != 3 || qs[0].ID != 3 {
		t.Errorf("non-numeric pos should fall back to 1 (prepend): %+v", qs)
	}
}

func TestIndexUnknownCollectionClearsCol(t *testing.T) {
	srv := newServer(t, newFake(sampleQuote(1)))
	rec := do(t, srv, "GET", "/?col=999", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "No collection selected") {
		t.Error("unknown col should fall back to the placeholder")
	}
}

func TestRenameCollectionEmptyName(t *testing.T) {
	fs, cid := fakeWithCollection(t)
	srv := newServer(t, fs)
	rec := do(t, srv, "POST", fmt.Sprintf("/collections/%d/rename", cid), "name=%20%20",
		"Content-Type", "application/x-www-form-urlencoded")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestCollectionViewInsertGapPositions(t *testing.T) {
	fs, cid := fakeWithCollection(t)
	srv := newServer(t, fs)
	rec := do(t, srv, "GET", fmt.Sprintf("/collections/%d", cid), "")
	body := rec.Body.String()
	for _, want := range []string{`data-pos="1"`, `data-pos="2"`, `data-pos="3"`} {
		if !strings.Contains(body, want) {
			t.Errorf("collection view missing insert gap %s", want)
		}
	}
	if strings.Contains(body, `data-pos="-1"`) || strings.Contains(body, `data-pos="0"`) {
		t.Error("collection view has out-of-range insert gap positions")
	}
}
