package server_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
)

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
	fs, cid := fakeWithCategory(t)
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

	if !strings.Contains(body, `data-action="copy"`) {
		t.Error("root blocks should be copyable")
	}
	if !strings.Contains(body, `/quotes/1/edit`) {
		t.Error("root blocks should be editable")
	}
	if !strings.Contains(body, fmt.Sprintf(`hx-delete="/categories/%d"`, cid)) {
		t.Error("rail should expose a delete-category button")
	}
}

func TestCategoryViewNotFound(t *testing.T) {
	srv := newServer(t, newFake(sampleQuote(1)))
	rec := do(t, srv, "GET", "/categories/999", "")
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestRailLeftListsCategories(t *testing.T) {
	fs := newFake(sampleQuote(1))
	if _, err := fs.CreateCategory("wisdom"); err != nil {
		t.Fatal(err)
	}
	srv := newServer(t, fs)
	rec := do(t, srv, "GET", "/rail/left", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `id="left-rail"`) {
		t.Error("left rail fragment missing root element")
	}
	if !strings.Contains(body, "Categories") {
		t.Error("left rail missing Categories heading")
	}
	if !strings.Contains(body, "wisdom") {
		t.Error("left rail missing category link")
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
	fs, _ := fakeWithCategory(t)
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
	fs, _ := fakeWithCategory(t)
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
	fs, cid := fakeWithCategory(t)
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

func TestSetQuoteCategoriesWithExistingName(t *testing.T) {
	fs := newFake(sampleQuote(1))
	if _, err := fs.CreateCategory("wisdom"); err != nil {
		t.Fatal(err)
	}
	srv := newServer(t, fs)
	rec := do(t, srv, "POST", "/quotes/1/categories", "new_name=wisdom",
		"Content-Type", "application/x-www-form-urlencoded")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	m, _ := fs.QuoteCategoryMap()
	if len(m[1]) != 1 || m[1][0].Name != "wisdom" {
		t.Errorf("quote 1 should be tagged with existing wisdom: %+v", m[1])
	}
	if len(fs.categories) != 1 {
		t.Errorf("should not duplicate the category: %+v", fs.categories)
	}
}

func TestCategoryExport(t *testing.T) {
	fs, cid := fakeWithCategory(t)
	srv := newServer(t, fs)
	rec := do(t, srv, "GET", fmt.Sprintf("/categories/%d/export.txt", cid), "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), ".  \n.  \n.") {
		t.Error("category export missing dot separator")
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
	fs, _ := fakeWithCategory(t)
	srv := newServer(t, fs)
	rec := do(t, srv, "GET", "/", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `id="left-rail"`) {
		t.Error("home should render the left rail")
	}
	if !strings.Contains(body, "Categories") {
		t.Error("left rail should show the Categories heading")
	}
	if !strings.Contains(body, "wisdom") {
		t.Error("left rail should list the category")
	}
	if !strings.Contains(body, `class="category-chip"`) {
		t.Error("quote block should render category chips")
	}
	if !strings.Contains(body, `/quotes/1/categories/edit`) {
		t.Error("editable block should expose the category editor trigger")
	}
}

func TestRootPaneCategoryFilter(t *testing.T) {
	fs, cid := fakeWithCategory(t)
	srv := newServer(t, fs)
	rec := do(t, srv, "GET", fmt.Sprintf("/pane/root?cat=%d", cid), "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `id="root-zone"`) {
		t.Error("pane should render the root zone")
	}
	if !strings.Contains(body, "#wisdom") {
		t.Error("filtered root pane should show the category title")
	}
	if !strings.Contains(body, `data-cat="1"`) {
		t.Error("root zone should carry the active category id")
	}
}

func TestRenameCategoryEmptyName(t *testing.T) {
	fs, cid := fakeWithCategory(t)
	srv := newServer(t, fs)
	rec := do(t, srv, "POST", fmt.Sprintf("/categories/%d/rename", cid), "name=%20%20",
		"Content-Type", "application/x-www-form-urlencoded")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}
