package server_test

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	"github.com/lavantien/quotes-manager/internal/quote"
	"github.com/lavantien/quotes-manager/internal/server"
	"github.com/lavantien/quotes-manager/internal/store"
)

// fakeStore is an in-memory store.Store for handler tests.
type fakeStore struct {
	quotes      []store.Quote
	nextID      int64
	collections []store.Collection
	items       map[int64][]int64
	categories  []store.Category
	tags        map[int64][]int64 // quote_id -> category_ids
}

func newFake(quotes ...store.Quote) *fakeStore {
	maxID := int64(0)
	for _, q := range quotes {
		if q.ID > maxID {
			maxID = q.ID
		}
	}
	return &fakeStore{
		quotes: append([]store.Quote{}, quotes...),
		nextID: maxID,
		items:  map[int64][]int64{},
		tags:   map[int64][]int64{},
	}
}

func (f *fakeStore) List() ([]store.Quote, error) {
	out := make([]store.Quote, len(f.quotes))
	copy(out, f.quotes)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].CharCount != out[j].CharCount {
			return out[i].CharCount < out[j].CharCount
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}

func (f *fakeStore) Get(id int64) (store.Quote, error) {
	for _, q := range f.quotes {
		if q.ID == id {
			return q, nil
		}
	}
	return store.Quote{}, store.ErrNotFound
}

func (f *fakeStore) Create(q *quote.Quote) (int64, error) {
	f.nextID++
	row := store.Quote{
		ID: f.nextID, SuttaID: q.SuttaID, Citation: q.Citation,
		BodyMD: q.BodyMD(), BodyText: q.BodyText(), LineCount: q.LineCount(),
		CharCount: q.CharCount(), Sources: append([]string(nil), q.Sources...),
	}
	f.quotes = append(f.quotes, row)
	return row.ID, nil
}

func (f *fakeStore) Update(id int64, q *quote.Quote) error {
	for i := range f.quotes {
		if f.quotes[i].ID == id {
			f.quotes[i].SuttaID = q.SuttaID
			f.quotes[i].Citation = q.Citation
			f.quotes[i].BodyMD = q.BodyMD()
			f.quotes[i].BodyText = q.BodyText()
			f.quotes[i].LineCount = q.LineCount()
			f.quotes[i].CharCount = q.CharCount()
			return nil
		}
	}
	return store.ErrNotFound
}

func (f *fakeStore) Delete(id int64) error {
	for i, q := range f.quotes {
		if q.ID == id {
			f.quotes = append(f.quotes[:i], f.quotes[i+1:]...)
			return nil
		}
	}
	return store.ErrNotFound
}

func (f *fakeStore) DeleteMany(ids []int64) error {
	keep := f.quotes[:0]
	drop := make(map[int64]bool)
	for _, id := range ids {
		drop[id] = true
	}
	for _, q := range f.quotes {
		if !drop[q.ID] {
			keep = append(keep, q)
		}
	}
	f.quotes = keep
	return nil
}

func (f *fakeStore) ListCollections() ([]store.Collection, error) {
	return append([]store.Collection{}, f.collections...), nil
}

func (f *fakeStore) CreateCollection(quoteIDs []int64) (int64, error) {
	var cid int64 = 1
	if n := len(f.collections); n > 0 {
		cid = f.collections[n-1].ID + 1
	}
	f.collections = append(f.collections, store.Collection{ID: cid, Count: len(quoteIDs)})
	f.items[cid] = append([]int64{}, quoteIDs...)
	return cid, nil
}

func (f *fakeStore) GetCollection(id int64) (store.Collection, error) {
	for _, c := range f.collections {
		if c.ID == id {
			return c, nil
		}
	}
	return store.Collection{}, store.ErrNotFound
}

func (f *fakeStore) CollectionQuotes(id int64) ([]store.Quote, error) {
	var out []store.Quote
	for _, qid := range f.items[id] {
		for _, q := range f.quotes {
			if q.ID == qid {
				out = append(out, q)
			}
		}
	}
	return out, nil
}

func (f *fakeStore) DeleteCollection(id int64) error {
	for i, c := range f.collections {
		if c.ID == id {
			f.collections = append(f.collections[:i], f.collections[i+1:]...)
			delete(f.items, id)
			return nil
		}
	}
	return store.ErrNotFound
}

func (f *fakeStore) ReorderCollection(cid int64, orderedQuoteIDs []int64) error {
	members, ok := f.items[cid]
	if !ok {
		return store.ErrNotFound
	}
	belong := make(map[int64]bool, len(members))
	for _, qid := range members {
		belong[qid] = true
	}
	for _, qid := range orderedQuoteIDs {
		if !belong[qid] {
			return store.ErrNotFound
		}
	}
	f.items[cid] = append([]int64{}, orderedQuoteIDs...)
	return nil
}

func (f *fakeStore) AddToCollection(cid int64, quoteIDs []int64) error {
	members, ok := f.items[cid]
	if !ok {
		return store.ErrNotFound
	}
	belong := make(map[int64]bool, len(members))
	for _, qid := range members {
		belong[qid] = true
	}
	seen := make(map[int64]bool)
	var add []int64
	for _, qid := range quoteIDs {
		if qid <= 0 || seen[qid] || belong[qid] {
			continue
		}
		seen[qid] = true
		add = append(add, qid)
	}
	f.items[cid] = append(add, members...)
	for i := range f.collections {
		if f.collections[i].ID == cid {
			f.collections[i].Count = len(f.items[cid])
		}
	}
	return nil
}

func (f *fakeStore) ListCategories() ([]store.Category, error) {
	out := make([]store.Category, 0, len(f.categories))
	for _, c := range f.categories {
		out = append(out, store.Category{ID: c.ID, Name: c.Name, Count: f.catCount(c.ID)})
	}
	sort.SliceStable(out, func(i, j int) bool {
		li, lj := strings.ToLower(out[i].Name), strings.ToLower(out[j].Name)
		if li != lj {
			return li < lj
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}

func (f *fakeStore) CreateCategory(name string) (int64, error) {
	for _, c := range f.categories {
		if strings.EqualFold(c.Name, name) {
			return 0, fmt.Errorf("%w: category %q", store.ErrDuplicate, name)
		}
	}
	var id int64 = 1
	if n := len(f.categories); n > 0 {
		id = f.categories[n-1].ID + 1
	}
	f.categories = append(f.categories, store.Category{ID: id, Name: name})
	return id, nil
}

func (f *fakeStore) GetCategory(id int64) (store.Category, error) {
	for _, c := range f.categories {
		if c.ID == id {
			return store.Category{ID: c.ID, Name: c.Name, Count: f.catCount(c.ID)}, nil
		}
	}
	return store.Category{}, store.ErrNotFound
}

func (f *fakeStore) RenameCategory(id int64, name string) error {
	for i, c := range f.categories {
		if c.ID == id {
			for _, other := range f.categories {
				if other.ID != id && strings.EqualFold(other.Name, name) {
					return fmt.Errorf("%w: category %q", store.ErrDuplicate, name)
				}
			}
			f.categories[i].Name = name
			return nil
		}
	}
	return store.ErrNotFound
}

func (f *fakeStore) DeleteCategory(id int64) error {
	for i, c := range f.categories {
		if c.ID == id {
			f.categories = append(f.categories[:i], f.categories[i+1:]...)
			for qid, cids := range f.tags {
				f.tags[qid] = removeID(cids, id)
			}
			return nil
		}
	}
	return store.ErrNotFound
}

func (f *fakeStore) CategoryQuotes(id int64) ([]store.Quote, error) {
	var out []store.Quote
	for _, q := range f.quotes {
		if containsID(f.tags[q.ID], id) {
			out = append(out, q)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].CharCount != out[j].CharCount {
			return out[i].CharCount < out[j].CharCount
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}

func (f *fakeStore) SetQuoteCategories(quoteID int64, categoryIDs []int64) error {
	found := false
	for _, q := range f.quotes {
		if q.ID == quoteID {
			found = true
			break
		}
	}
	if !found {
		return store.ErrNotFound
	}
	known := make(map[int64]bool, len(f.categories))
	for _, c := range f.categories {
		known[c.ID] = true
	}
	seen := make(map[int64]bool)
	var ids []int64
	for _, cid := range categoryIDs {
		if cid <= 0 || seen[cid] {
			continue
		}
		if !known[cid] {
			return store.ErrNotFound
		}
		seen[cid] = true
		ids = append(ids, cid)
	}
	f.tags[quoteID] = ids
	return nil
}

func (f *fakeStore) QuoteCategoryMap() (map[int64][]store.Category, error) {
	out := make(map[int64][]store.Category)
	for qid, cids := range f.tags {
		var cs []store.Category
		for _, cid := range cids {
			for _, c := range f.categories {
				if c.ID == cid {
					cs = append(cs, store.Category{ID: c.ID, Name: c.Name})
				}
			}
		}
		sort.SliceStable(cs, func(i, j int) bool {
			return strings.ToLower(cs[i].Name) < strings.ToLower(cs[j].Name)
		})
		if len(cs) > 0 {
			out[qid] = cs
		}
	}
	return out, nil
}

// catCount reports how many quotes are tagged with the category.
func (f *fakeStore) catCount(cid int64) int {
	n := 0
	for _, cids := range f.tags {
		if containsID(cids, cid) {
			n++
		}
	}
	return n
}

func containsID(xs []int64, x int64) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

func removeID(xs []int64, x int64) []int64 {
	out := xs[:0]
	for _, v := range xs {
		if v != x {
			out = append(out, v)
		}
	}
	return out
}

func (f *fakeStore) Close() error { return nil }

// --- helpers ---

func newServer(t *testing.T, s store.Store) *server.Server {
	t.Helper()
	return server.New(s)
}

func do(t *testing.T, srv *server.Server, method, target, body string, hdrs ...string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	for i := 0; i+1 < len(hdrs); i += 2 {
		req.Header.Set(hdrs[i], hdrs[i+1])
	}
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	return rec
}

func sampleQuote(id int64) store.Quote {
	q := quote.New("MN 22", "the Buddha, MN 22", []string{`"Human beings are shady."`})
	return store.Quote{
		ID: id, SuttaID: q.SuttaID, Citation: q.Citation,
		BodyMD: q.BodyMD(), BodyText: q.BodyText(), LineCount: q.LineCount(), CharCount: q.CharCount(),
	}
}

func TestIndexListsQuotesAndLinksSuttacentral(t *testing.T) {
	srv := newServer(t, newFake(sampleQuote(1)))
	rec := do(t, srv, "GET", "/", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Human beings are shady") {
		t.Error("index missing quote body")
	}
	if !strings.Contains(body, `href="https://suttacentral.net/mn22"`) {
		t.Error("index missing suttacentral link")
	}
	if !strings.Contains(body, `<strong>MN 22</strong>`) {
		t.Error("index missing bolded sutta id")
	}
}

func TestIndexShowsBlockCount(t *testing.T) {
	srv := newServer(t, newFake(sampleQuote(1), sampleQuote(2)))
	rec := do(t, srv, "GET", "/", "")
	if !strings.Contains(rec.Body.String(), "2 blocks") {
		t.Error("index missing block count badge")
	}
}

func TestCollectionViewShowsBlockCount(t *testing.T) {
	fs, cid := fakeWithCollection(t) // holds quotes 1, 2
	srv := newServer(t, fs)
	rec := do(t, srv, "GET", fmt.Sprintf("/collections/%d", cid), "")
	if !strings.Contains(rec.Body.String(), "2 blocks") {
		t.Error("collection view missing block count badge")
	}
}

func TestCreateAddsQuote(t *testing.T) {
	fs := newFake()
	srv := newServer(t, fs)
	body := "content=%22Be+your+own+island.%22&attribution=the+Buddha&text_id=MN+44"
	rec := do(t, srv, "POST", "/quotes", body, "Content-Type", "application/x-www-form-urlencoded", "HX-Request", "true")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if len(fs.quotes) != 1 {
		t.Fatalf("store has %d quotes, want 1", len(fs.quotes))
	}
	q := fs.quotes[0]
	if q.SuttaID != "MN 44" {
		t.Errorf("SuttaID = %q", q.SuttaID)
	}
	if q.Citation != "the Buddha, MN 44" {
		t.Errorf("Citation = %q", q.Citation)
	}
	if !strings.Contains(rec.Body.String(), `href="https://suttacentral.net/mn44"`) {
		t.Error("create response missing link to new quote")
	}
}

func TestCreateDefaultsAttribution(t *testing.T) {
	fs := newFake()
	srv := newServer(t, fs)
	body := "content=%22x%22&text_id=SN+22.59"
	do(t, srv, "POST", "/quotes", body, "Content-Type", "application/x-www-form-urlencoded", "HX-Request", "true")

	if len(fs.quotes) != 1 {
		t.Fatal("no quote created")
	}
	if want := "the Buddha, SN 22.59"; fs.quotes[0].Citation != want {
		t.Errorf("Citation = %q, want %q", fs.quotes[0].Citation, want)
	}
}

func TestUpdateChangesQuote(t *testing.T) {
	fs := newFake(sampleQuote(1))
	srv := newServer(t, fs)
	body := "content=%22new+text%22&attribution=the+Buddha&text_id=DN+16"
	rec := do(t, srv, "POST", "/quotes/1", body, "Content-Type", "application/x-www-form-urlencoded", "HX-Request", "true")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if fs.quotes[0].SuttaID != "DN 16" || fs.quotes[0].Citation != "the Buddha, DN 16" {
		t.Errorf("not updated: %+v", fs.quotes[0])
	}
}

func TestUpdateNotFound(t *testing.T) {
	srv := newServer(t, newFake())
	body := "content=x&text_id=MN+1"
	rec := do(t, srv, "POST", "/quotes/999", body, "Content-Type", "application/x-www-form-urlencoded")
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestDeleteOne(t *testing.T) {
	fs := newFake(sampleQuote(1))
	srv := newServer(t, fs)
	rec := do(t, srv, "DELETE", "/quotes/1", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if len(fs.quotes) != 0 {
		t.Errorf("store still has %d quotes", len(fs.quotes))
	}
}

func TestBulkDelete(t *testing.T) {
	fs := newFake(sampleQuote(1), sampleQuote(2), sampleQuote(3))
	srv := newServer(t, fs)
	body := "id=1&id=3"
	rec := do(t, srv, "POST", "/quotes/delete", body, "Content-Type", "application/x-www-form-urlencoded")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if len(fs.quotes) != 1 || fs.quotes[0].ID != 2 {
		t.Errorf("after bulk delete = %+v", fs.quotes)
	}
}

func TestCreateRendersSortedList(t *testing.T) {
	fs := newFake()
	srv := newServer(t, fs)
	body := "content=%22Be+your+own+island.%22&attribution=the+Buddha&text_id=MN+44"
	rec := do(t, srv, "POST", "/quotes", body, "Content-Type", "application/x-www-form-urlencoded", "HX-Request", "true")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	// create re-renders the whole list so the new quote lands in char_count order.
	if !strings.Contains(rec.Body.String(), `id="quote-list"`) {
		t.Error("create should re-render the full quote list, not a single block")
	}
}

func TestExportTxt(t *testing.T) {
	srv := newServer(t, newFake(sampleQuote(1), sampleQuote(2)))
	rec := do(t, srv, "GET", "/export.txt", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") {
		t.Errorf("Content-Type = %q", ct)
	}
	// The dot divider (".  \n.  \n.") joins consecutive quotes in the export.
	if !strings.Contains(rec.Body.String(), ".  \n.  \n.") {
		t.Error("export missing dot separator")
	}
}

func TestCopyOne(t *testing.T) {
	srv := newServer(t, newFake(sampleQuote(1)))
	rec := do(t, srv, "GET", "/quotes/1/copy", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	want := `*"Human beings are shady."* - **the Buddha, MN 22**`
	if b, _ := io.ReadAll(rec.Body); string(b) != want {
		t.Errorf("copy = %q, want %q", string(b), want)
	}
}

func TestCopyOneNotFound(t *testing.T) {
	srv := newServer(t, newFake())
	rec := do(t, srv, "GET", "/quotes/999/copy", "")
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestEditFormPrefilled(t *testing.T) {
	srv := newServer(t, newFake(sampleQuote(1)))
	rec := do(t, srv, "GET", "/quotes/1/edit", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `value="MN 22"`) {
		t.Error("edit form missing text_id value")
	}
	if !strings.Contains(body, `Human beings are shady`) {
		t.Error("edit form missing content")
	}
	// The edit form keeps the block's id so HTMX can target it on submit after
	// the form replaced the block via outerHTML.
	if !strings.Contains(body, `id="quote-1"`) {
		t.Error("edit form missing id=quote-1 handle")
	}
}

func TestStaticAssetsServed(t *testing.T) {
	srv := newServer(t, newFake())
	for _, p := range []string{"/static/app.css", "/static/app.js", "/static/htmx.min.js"} {
		rec := do(t, srv, "GET", p, "")
		if rec.Code != http.StatusOK {
			t.Errorf("%s: status = %d", p, rec.Code)
		}
	}
	if rec := do(t, srv, "GET", "/static/app.css", ""); !strings.Contains(rec.Body.String(), "--paper") {
		t.Error("app.css not served correctly")
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
	loc := rec.Header().Get("HX-Redirect")
	if !strings.HasPrefix(loc, "/collections/") {
		t.Errorf("HX-Redirect = %q", loc)
	}
	if len(fs.collections) != 1 || fs.collections[0].Count != 2 {
		t.Errorf("collections = %+v", fs.collections)
	}
}

func TestAddToCollectionItems(t *testing.T) {
	fs, cid := fakeWithCollection(t) // collection holds quotes 1, 2; quote 3 also exists
	srv := newServer(t, fs)
	rec := do(t, srv, "POST", fmt.Sprintf("/collections/%d/items", cid), "id=3",
		"Content-Type", "application/x-www-form-urlencoded")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if loc := rec.Header().Get("HX-Redirect"); loc != fmt.Sprintf("/collections/%d", cid) {
		t.Errorf("HX-Redirect = %q, want /collections/%d", loc, cid)
	}
	// New item (3) lands on top; existing order (1, 2) preserved.
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
	if strings.Contains(body, "+ New") {
		t.Error("collection view must not show + New")
	}
	if !strings.Contains(body, "Delete collection") {
		t.Error("collection view missing delete-collection button")
	}
	// Read-only for content (no edit/delete/new) but still sortable by drag.
	if !strings.Contains(body, `draggable="true"`) {
		t.Error("collection blocks should be draggable to reorder")
	}
	if strings.Contains(body, `/edit"`) {
		t.Error("collection blocks must not be editable")
	}
	if !strings.Contains(body, `data-action="copy"`) {
		t.Error("collection blocks should be copyable")
	}
}

func TestCollectionReorder(t *testing.T) {
	fs, cid := fakeWithCollection(t) // collection holds quotes 1, 2 (in that order)
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

func fakeWithCategory(t *testing.T) (*fakeStore, int64) {
	t.Helper()
	fs := newFake(sampleQuote(1), sampleQuote(2), sampleQuote(3))
	cid, err := fs.CreateCategory("wisdom")
	if err != nil {
		t.Fatal(err)
	}
	for _, qid := range []int64{1, 2} {
		if err := fs.SetQuoteCategories(qid, []int64{cid}); err != nil {
			t.Fatal(err)
		}
	}
	return fs, cid
}

func TestCategoryView(t *testing.T) {
	fs, cid := fakeWithCategory(t) // category tags quotes 1 and 2
	srv := newServer(t, fs)
	rec := do(t, srv, "GET", fmt.Sprintf("/categories/%d", cid), "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Human beings are shady") {
		t.Error("category view missing its quotes")
	}
	if !strings.Contains(body, "#wisdom") {
		t.Error("category view missing #name title")
	}
	if strings.Contains(body, "+ New") {
		t.Error("category view must not show + New")
	}
	if !strings.Contains(body, "Delete category") {
		t.Error("category view missing delete-category button")
	}
	// Read-only like a collection: copyable, not editable.
	if strings.Contains(body, `/edit"`) {
		t.Error("category blocks must not be editable")
	}
	if !strings.Contains(body, `data-action="copy"`) {
		t.Error("category blocks should be copyable")
	}
}

func TestCategoryViewNotFound(t *testing.T) {
	srv := newServer(t, newFake(sampleQuote(1)))
	rec := do(t, srv, "GET", "/categories/999", "")
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestSidebarListsCollectionsAndCategories(t *testing.T) {
	fs := newFake(sampleQuote(1))
	if _, err := fs.CreateCategory("wisdom"); err != nil {
		t.Fatal(err)
	}
	srv := newServer(t, fs)
	rec := do(t, srv, "GET", "/sidebar", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `<aside class="sidebar">`) {
		t.Error("sidebar fragment missing root element")
	}
	if !strings.Contains(body, "Collections") || !strings.Contains(body, "Categories") {
		t.Error("sidebar missing section headings")
	}
	if !strings.Contains(body, "wisdom") {
		t.Error("sidebar missing category link")
	}
}

func TestCreateCategory(t *testing.T) {
	fs := newFake(sampleQuote(1))
	srv := newServer(t, fs)
	rec := do(t, srv, "POST", "/categories", "name=wisdom", "Content-Type", "application/x-www-form-urlencoded")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "wisdom") {
		t.Error("create response should include the new category in the sidebar")
	}
	if len(fs.categories) != 1 || fs.categories[0].Name != "wisdom" {
		t.Errorf("categories = %+v", fs.categories)
	}
}

func TestCreateCategoryDuplicate(t *testing.T) {
	fs := newFake(sampleQuote(1))
	if _, err := fs.CreateCategory("wisdom"); err != nil {
		t.Fatal(err)
	}
	srv := newServer(t, fs)
	rec := do(t, srv, "POST", "/categories", "name=wisdom", "Content-Type", "application/x-www-form-urlencoded")
	if rec.Code != http.StatusConflict {
		t.Errorf("status = %d, want 409", rec.Code)
	}
	if len(fs.categories) != 1 {
		t.Errorf("duplicate create should not add a category: %+v", fs.categories)
	}
}

func TestCreateCategoryEmptyName(t *testing.T) {
	srv := newServer(t, newFake(sampleQuote(1)))
	rec := do(t, srv, "POST", "/categories", "name=%20%20", "Content-Type", "application/x-www-form-urlencoded")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestRenameCategory(t *testing.T) {
	fs := newFake(sampleQuote(1))
	cid, err := fs.CreateCategory("wisdom")
	if err != nil {
		t.Fatal(err)
	}
	srv := newServer(t, fs)
	rec := do(t, srv, "POST", fmt.Sprintf("/categories/%d/rename", cid), "name=insight",
		"Content-Type", "application/x-www-form-urlencoded")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "insight") {
		t.Error("rename response should include the renamed category")
	}
	if fs.categories[0].Name != "insight" {
		t.Errorf("name = %q, want insight", fs.categories[0].Name)
	}
}

func TestRenameCategoryDuplicate(t *testing.T) {
	fs := newFake(sampleQuote(1))
	cid, err := fs.CreateCategory("wisdom")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fs.CreateCategory("joy"); err != nil {
		t.Fatal(err)
	}
	srv := newServer(t, fs)
	rec := do(t, srv, "POST", fmt.Sprintf("/categories/%d/rename", cid), "name=joy",
		"Content-Type", "application/x-www-form-urlencoded")
	if rec.Code != http.StatusConflict {
		t.Errorf("status = %d, want 409", rec.Code)
	}
	if fs.categories[0].Name != "wisdom" {
		t.Errorf("failed rename changed the name: %q", fs.categories[0].Name)
	}
}

func TestDeleteCategoryHandler(t *testing.T) {
	fs := newFake(sampleQuote(1))
	cid, err := fs.CreateCategory("wisdom")
	if err != nil {
		t.Fatal(err)
	}
	srv := newServer(t, fs)
	rec := do(t, srv, "DELETE", fmt.Sprintf("/categories/%d", cid), "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if rec.Header().Get("HX-Redirect") != "/" {
		t.Errorf("HX-Redirect = %q, want /", rec.Header().Get("HX-Redirect"))
	}
	if len(fs.categories) != 0 {
		t.Errorf("category not deleted: %+v", fs.categories)
	}
}

func TestQuoteChips(t *testing.T) {
	fs, _ := fakeWithCategory(t) // quote 1 tagged "wisdom"
	srv := newServer(t, fs)
	rec := do(t, srv, "GET", "/quotes/1/categories", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `id="chips-1"`) {
		t.Error("chips fragment missing chips-1 handle")
	}
	if !strings.Contains(body, "wisdom") {
		t.Error("chips fragment missing the category chip")
	}
}

func TestEditQuoteCategories(t *testing.T) {
	fs, _ := fakeWithCategory(t) // quote 1 tagged "wisdom" (category id 1)
	srv := newServer(t, fs)
	rec := do(t, srv, "GET", "/quotes/1/categories/edit", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `name="id"`) {
		t.Error("editor missing category checkbox")
	}
	if !strings.Contains(body, `value="1" checked`) {
		t.Error("editor should pre-check the quote's current category")
	}
	if !strings.Contains(body, `name="new_name"`) {
		t.Error("editor missing new-category field")
	}
}

func TestSetQuoteCategories(t *testing.T) {
	fs, cid := fakeWithCategory(t) // quote 3 exists, untagged
	cid2, err := fs.CreateCategory("joy")
	if err != nil {
		t.Fatal(err)
	}
	srv := newServer(t, fs)
	rec := do(t, srv, "POST", "/quotes/3/categories", fmt.Sprintf("id=%d&id=%d", cid, cid2),
		"Content-Type", "application/x-www-form-urlencoded")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "joy") {
		t.Error("set response should render the updated chip row")
	}
	m, _ := fs.QuoteCategoryMap()
	if len(m[3]) != 2 {
		t.Errorf("quote 3 tags = %+v, want 2", m[3])
	}
}

func TestSetQuoteCategoriesWithNewName(t *testing.T) {
	fs := newFake(sampleQuote(1))
	srv := newServer(t, fs)
	rec := do(t, srv, "POST", "/quotes/1/categories", "new_name=insight",
		"Content-Type", "application/x-www-form-urlencoded")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "insight") {
		t.Error("chip row should include the newly created category")
	}
	if len(fs.categories) != 1 || fs.categories[0].Name != "insight" {
		t.Errorf("category not created: %+v", fs.categories)
	}
	m, _ := fs.QuoteCategoryMap()
	if len(m[1]) != 1 || m[1][0].Name != "insight" {
		t.Errorf("quote 1 not tagged: %+v", m[1])
	}
}

func TestSetQuoteCategoriesUnknownQuote(t *testing.T) {
	fs := newFake(sampleQuote(1))
	cid, err := fs.CreateCategory("wisdom")
	if err != nil {
		t.Fatal(err)
	}
	srv := newServer(t, fs)
	rec := do(t, srv, "POST", "/quotes/999/categories", fmt.Sprintf("id=%d", cid),
		"Content-Type", "application/x-www-form-urlencoded")
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestIndexSidebarAndChips(t *testing.T) {
	fs, _ := fakeWithCategory(t) // quote 1 tagged "wisdom"
	srv := newServer(t, fs)
	rec := do(t, srv, "GET", "/", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `<aside class="sidebar">`) {
		t.Error("home should render the sidebar")
	}
	if !strings.Contains(body, "Categories") {
		t.Error("sidebar should show the Categories heading")
	}
	if !strings.Contains(body, "wisdom") {
		t.Error("sidebar should list the category")
	}
	if !strings.Contains(body, `class="category-chip"`) {
		t.Error("quote block should render category chips")
	}
	if !strings.Contains(body, `/quotes/1/categories/edit`) {
		t.Error("editable block should expose the category editor trigger")
	}
}
