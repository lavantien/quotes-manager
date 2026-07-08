package server_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/lavantien/quotes-manager/internal/store"
)

// newFakeWithCategories returns an empty fakeStore preloaded with categories so
// the new-quote form has existing categories to render as checkboxes.
func newFakeWithCategories(cats ...store.Category) *fakeStore {
	fs := newFake()
	fs.categories = append(fs.categories[:0:0], cats...)
	return fs
}

// baseCreateForm returns the minimal valid create form (content + text_id).
func baseCreateForm() url.Values {
	v := url.Values{}
	v.Set("content", `"a test quote"`)
	v.Set("text_id", "MN 1")
	return v
}

// lastQuoteID is the id of the most recently created quote in the fake store.
func lastQuoteID(t *testing.T, fs *fakeStore) int64 {
	t.Helper()
	if len(fs.quotes) == 0 {
		t.Fatal("no quote created")
	}
	return fs.quotes[len(fs.quotes)-1].ID
}

func TestNewFormListsCategories(t *testing.T) {
	fs := newFakeWithCategories(
		store.Category{ID: 1, Name: "suffering"},
		store.Category{ID: 2, Name: "right view"},
	)
	srv := newServer(t, fs)
	rec := do(t, srv, "GET", "/quotes/new", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `name="category"`) {
		t.Error("new form should render category checkboxes")
	}
	if !strings.Contains(body, `value="1"`) || !strings.Contains(body, "#suffering") {
		t.Error("new form should list existing categories as checkboxes")
	}
	if !strings.Contains(body, `name="new_category"`) {
		t.Error("new form should include a new-category input")
	}
}

func TestCreateWithExistingCategory(t *testing.T) {
	fs := newFakeWithCategories(store.Category{ID: 1, Name: "suffering"})
	srv := newServer(t, fs)
	v := baseCreateForm()
	v.Set("category", "1")
	rec := do(t, srv, "POST", "/quotes", v.Encode(),
		"Content-Type", "application/x-www-form-urlencoded", "HX-Request", "true")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	id := lastQuoteID(t, fs)
	if got := fs.tags[id]; len(got) != 1 || got[0] != 1 {
		t.Errorf("new quote tagged %v, want [1]", got)
	}
}

func TestCreateWithNewCategory(t *testing.T) {
	fs := newFake()
	srv := newServer(t, fs)
	v := baseCreateForm()
	v.Set("new_category", "Wisdom")
	rec := do(t, srv, "POST", "/quotes", v.Encode(),
		"Content-Type", "application/x-www-form-urlencoded", "HX-Request", "true")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if len(fs.categories) != 1 || fs.categories[0].Name != "Wisdom" {
		t.Errorf("category not created: %+v", fs.categories)
	}
	id := lastQuoteID(t, fs)
	if got := fs.tags[id]; len(got) != 1 || got[0] != fs.categories[0].ID {
		t.Errorf("new quote tagged %v, want the new category id", got)
	}
}

func TestCreateWithExistingAndNewCategory(t *testing.T) {
	fs := newFakeWithCategories(store.Category{ID: 1, Name: "suffering"})
	srv := newServer(t, fs)
	v := baseCreateForm()
	v.Set("category", "1")
	v.Set("new_category", "Wisdom")
	rec := do(t, srv, "POST", "/quotes", v.Encode(),
		"Content-Type", "application/x-www-form-urlencoded", "HX-Request", "true")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	id := lastQuoteID(t, fs)
	got := fs.tags[id]
	if len(got) != 2 {
		t.Fatalf("new quote tagged %v, want 2 ids", got)
	}
	want := map[int64]bool{1: true, fs.categories[len(fs.categories)-1].ID: true}
	for _, cid := range got {
		if !want[cid] {
			t.Errorf("unexpected category id %d in %v", cid, got)
		}
	}
}

func TestCreateNewCategoryDedupsExistingName(t *testing.T) {
	fs := newFakeWithCategories(store.Category{ID: 1, Name: "Wisdom"})
	srv := newServer(t, fs)
	v := baseCreateForm()
	v.Set("new_category", "wisdom") // same name, different case
	rec := do(t, srv, "POST", "/quotes", v.Encode(),
		"Content-Type", "application/x-www-form-urlencoded", "HX-Request", "true")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if len(fs.categories) != 1 {
		t.Errorf("duplicate category created: %+v", fs.categories)
	}
	id := lastQuoteID(t, fs)
	if got := fs.tags[id]; len(got) != 1 || got[0] != 1 {
		t.Errorf("new quote tagged %v, want existing id [1]", got)
	}
}

func TestCreateWithoutCategories(t *testing.T) {
	fs := newFake()
	srv := newServer(t, fs)
	rec := do(t, srv, "POST", "/quotes", baseCreateForm().Encode(),
		"Content-Type", "application/x-www-form-urlencoded", "HX-Request", "true")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if len(fs.quotes) != 1 {
		t.Fatalf("store has %d quotes, want 1", len(fs.quotes))
	}
	if id := lastQuoteID(t, fs); len(fs.tags[id]) != 0 {
		t.Errorf("new quote should not be tagged, got %v", fs.tags[id])
	}
}

func TestCreateResolveCategoryError(t *testing.T) {
	srv := newServer(t, &failCreateCategory{newFake()})
	v := baseCreateForm()
	v.Set("new_category", "Wisdom")
	rec := do(t, srv, "POST", "/quotes", v.Encode(),
		"Content-Type", "application/x-www-form-urlencoded", "HX-Request", "true")
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
}

func TestCreateSetQuoteCategoriesError(t *testing.T) {
	fs := newFakeWithCategories(store.Category{ID: 1, Name: "suffering"})
	srv := newServer(t, &failSetQuoteCategories{fs})
	v := baseCreateForm()
	v.Set("category", "1")
	rec := do(t, srv, "POST", "/quotes", v.Encode(),
		"Content-Type", "application/x-www-form-urlencoded", "HX-Request", "true")
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
}
