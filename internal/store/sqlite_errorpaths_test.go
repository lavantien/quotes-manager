package store

import (
	"database/sql"
	"errors"
	"testing"

	"github.com/mattn/go-sqlite3"

	"github.com/lavantien/quotes-manager/internal/quote"
)

// TestClosedDBMethodsError drives every SQLiteStore method against a closed
// connection pool so the top-of-method Query/QueryRow/Exec/Begin error returns
// (the bulk of sqlite.go's uncovered branches) are exercised.
func TestClosedDBMethodsError(t *testing.T) {
	s := newTestStore(t)
	q := mustCreate(t, s, quote.New("MN 22", "the Buddha, MN 22", []string{`"hi"`}))
	cid, _ := s.CreateCollection([]int64{q})
	catID, _ := s.CreateCategory("wisdom")

	if err := s.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	qp := quote.New("X", "X", []string{"x"})
	checks := []struct {
		name string
		fn   func() error
	}{
		{"List", func() error { _, err := s.List(); return err }},
		{"Get", func() error { _, err := s.Get(q); return err }},
		{"Create", func() error { _, err := s.Create(qp); return err }},
		{"Update", func() error { return s.Update(q, qp) }},
		{"Delete", func() error { return s.Delete(q) }},
		{"DeleteMany", func() error { return s.DeleteMany([]int64{q}) }},
		{"ListCollections", func() error { _, err := s.ListCollections(); return err }},
		{"CreateCollection", func() error { _, err := s.CreateCollection([]int64{q}); return err }},
		{"AddToCollection", func() error { return s.AddToCollection(cid, []int64{q}) }},
		{"InsertAtCollection", func() error { return s.InsertAtCollection(cid, []int64{q}, 1) }},
		{"GetCollection", func() error { _, err := s.GetCollection(cid); return err }},
		{"CollectionQuotes", func() error { _, err := s.CollectionQuotes(cid); return err }},
		{"RenameCollection", func() error { return s.RenameCollection(cid, "x") }},
		{"ReorderCollection", func() error { return s.ReorderCollection(cid, []int64{q}) }},
		{"DeleteCollection", func() error { return s.DeleteCollection(cid) }},
		{"ListCategories", func() error { _, err := s.ListCategories(); return err }},
		{"CreateCategory", func() error { _, err := s.CreateCategory("x"); return err }},
		{"GetCategory", func() error { _, err := s.GetCategory(catID); return err }},
		{"RenameCategory", func() error { return s.RenameCategory(catID, "x") }},
		{"DeleteCategory", func() error { return s.DeleteCategory(catID) }},
		{"CategoryQuotes", func() error { _, err := s.CategoryQuotes(catID); return err }},
		{"SetQuoteCategories", func() error { return s.SetQuoteCategories(q, []int64{catID}) }},
		{"QuoteCategoryMap", func() error { _, err := s.QuoteCategoryMap(); return err }},
		{"QuoteCollectionMap", func() error { _, err := s.QuoteCollectionMap(); return err }},
		{"MergeQuotes", func() error { return s.MergeQuotes(q, []int64{q + 1}) }},
		{"Export", func() error { _, err := s.Export(); return err }},
		{"Import", func() error {
			return s.Import(&Dump{Version: DumpVersion, Quotes: []Quote{{ID: q, SuttaID: "X", Citation: "X", BodyMD: "X", BodyText: "X", LineCount: 1, CharCount: 1}}})
		}},
	}
	for _, c := range checks {
		if err := c.fn(); err == nil {
			t.Errorf("%s: expected error after Close, got nil", c.name)
		}
	}
}

func TestDBGetter(t *testing.T) {
	s := newTestStore(t)
	if s.DB() == nil {
		t.Error("DB() = nil, want the underlying *sql.DB")
	}
}

func TestSplitSources(t *testing.T) {
	if got := splitSources(""); got != nil {
		t.Errorf("splitSources(\"\") = %#v, want nil", got)
	}
	got := splitSources("a;b;c")
	want := []string{"a", "b", "c"}
	if len(got) != len(want) {
		t.Fatalf("splitSources = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("splitSources[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestIsUniqueViolation(t *testing.T) {
	if !isUniqueViolation(sqlite3.Error{Code: sqlite3.ErrConstraint, ExtendedCode: sqlite3.ErrConstraintUnique}) {
		t.Error("ErrConstraintUnique should be a unique violation")
	}
	if !isUniqueViolation(sqlite3.Error{Code: sqlite3.ErrConstraint}) {
		t.Error("ErrConstraint should be treated as a violation")
	}
	if isUniqueViolation(errors.New("not a sqlite error")) {
		t.Error("plain error should not be a unique violation")
	}
}

type fakeResult struct {
	rows int64
	err  error
}

func (f fakeResult) LastInsertId() (int64, error) { return 0, f.err }
func (f fakeResult) RowsAffected() (int64, error) { return f.rows, f.err }

func TestRowsAffected(t *testing.T) {
	if err := rowsAffected(fakeResult{rows: 0}, 5); !errors.Is(err, ErrNotFound) {
		t.Errorf("zero rows = %v, want ErrNotFound", err)
	}
	if err := rowsAffected(fakeResult{rows: 1}, 5); err != nil {
		t.Errorf("one row = %v, want nil", err)
	}
	boom := errors.New("rows unavailable")
	if err := rowsAffected(fakeResult{err: boom}, 5); !errors.Is(err, boom) {
		t.Errorf("driver error = %v, want %v", err, boom)
	}
}

type errScanner struct{ err error }

func (e errScanner) Scan(dest ...any) error { return e.err }

func TestScanQuoteError(t *testing.T) {
	if _, err := scanQuote(errScanner{errors.New("scan failed")}); err == nil {
		t.Error("scanQuote should surface the scan error")
	}
}

// TestSQLDriverTypes compiles a check that the sql package is reachable (keeps
// the database/sql import meaningful alongside fakeResult).
var _ sql.Result = fakeResult{}

// TestStoreMidTransactionErrors drops the membership tables so the mid-tx
// DELETE/UPDATE/SELECT statements fail, exercising each method's rollback path.
func TestStoreMidTransactionErrors(t *testing.T) {
	s := newTestStore(t)
	q := mustCreate(t, s, quote.New("X", "X", []string{"x"}))
	cid, _ := s.CreateCollection([]int64{q})
	catID, _ := s.CreateCategory("wisdom")

	for _, stmt := range []string{"DROP TABLE collection_items", "DROP TABLE category_items"} {
		if _, err := s.DB().Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}

	checks := []struct {
		name string
		fn   func() error
	}{
		{"Delete", func() error { return s.Delete(q) }},
		{"DeleteMany", func() error { return s.DeleteMany([]int64{q}) }},
		{"CreateCollection", func() error { _, err := s.CreateCollection([]int64{q}); return err }},
		{"AddToCollection", func() error { return s.AddToCollection(cid, []int64{q}) }},
		{"InsertAtCollection", func() error { return s.InsertAtCollection(cid, []int64{q}, 1) }},
		{"ReorderCollection", func() error { return s.ReorderCollection(cid, []int64{q}) }},
		{"DeleteCollection", func() error { return s.DeleteCollection(cid) }},
		{"DeleteCategory", func() error { return s.DeleteCategory(catID) }},
		{"SetQuoteCategories", func() error { return s.SetQuoteCategories(q, []int64{catID}) }},
	}
	for _, c := range checks {
		if err := c.fn(); err == nil {
			t.Errorf("%s: expected mid-transaction error, got nil", c.name)
		}
	}
}

// TestStoreExistenceCheckScanErrors drives the non-ErrNoRows branches of the
// per-method existence checks by dropping the table they query.
func TestStoreExistenceCheckScanErrors(t *testing.T) {
	t.Run("collections existence", func(t *testing.T) {
		s := newTestStore(t)
		q := mustCreate(t, s, quote.New("X", "X", []string{"x"}))
		if _, err := s.DB().Exec("DROP TABLE collections"); err != nil {
			t.Fatal(err)
		}
		for _, c := range []struct {
			name string
			fn   func() error
		}{
			{"AddToCollection", func() error { return s.AddToCollection(1, []int64{q}) }},
			{"InsertAtCollection", func() error { return s.InsertAtCollection(1, []int64{q}, 1) }},
			{"ReorderCollection", func() error { return s.ReorderCollection(1, []int64{q}) }},
			{"RenameCollection", func() error { return s.RenameCollection(1, "x") }},
			{"GetCollection", func() error { _, err := s.GetCollection(1); return err }},
			{"ListCollections", func() error { _, err := s.ListCollections(); return err }},
		} {
			if err := c.fn(); err == nil {
				t.Errorf("%s: expected error (collections dropped)", c.name)
			}
		}
	})
	t.Run("quote existence in SetQuoteCategories", func(t *testing.T) {
		s := newTestStore(t)
		if _, err := s.DB().Exec("DROP TABLE quotes"); err != nil {
			t.Fatal(err)
		}
		if err := s.SetQuoteCategories(1, nil); err == nil {
			t.Error("SetQuoteCategories: expected error (quotes dropped)")
		}
	})
	t.Run("category membership dropped", func(t *testing.T) {
		s := newTestStore(t)
		mustCreate(t, s, quote.New("X", "X", []string{"x"}))
		if _, err := s.DB().Exec("DROP TABLE category_items"); err != nil {
			t.Fatal(err)
		}
		// CategoryQueries joins category_items; with it gone, the query errors.
		if _, err := s.CategoryQuotes(1); err == nil {
			t.Error("CategoryQueries: expected error (category_items dropped)")
		}
		if _, err := s.QuoteCategoryMap(); err == nil {
			t.Error("QuoteCategoryMap: expected error (category_items dropped)")
		}
	})
}

// TestStoreScanErrors inserts a quote with a non-numeric line_count so scanQuote
// fails mid-iteration, exercising the scan-error return in each reader.
func TestStoreScanErrors(t *testing.T) {
	s := newTestStore(t)
	for _, stmt := range []string{
		`INSERT INTO quotes (id, sutta_id, citation, body_md, body_text, line_count, char_count, sources)
		 VALUES (1, 'X', 'X', 'x', 'x', 'NOTANINT', 1, 's')`,
		`INSERT INTO collection_items (collection_id, quote_id, position) VALUES (1, 1, 1)`,
		`INSERT INTO category_items (category_id, quote_id) VALUES (1, 1)`,
	} {
		if _, err := s.DB().Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	for _, c := range []struct {
		name string
		fn   func() error
	}{
		{"List", func() error { _, err := s.List(); return err }},
		{"Get", func() error { _, err := s.Get(1); return err }},
		{"CollectionQuotes", func() error { _, err := s.CollectionQuotes(1); return err }},
		{"CategoryQuotes", func() error { _, err := s.CategoryQuotes(1); return err }},
	} {
		if err := c.fn(); err == nil {
			t.Errorf("%s: expected scan error from malformed row", c.name)
		}
	}
}

// TestStoreMapScanErrors inserts membership rows with a non-numeric quote_id so
// the map scans fail mid-iteration.
func TestStoreMapScanErrors(t *testing.T) {
	s := newTestStore(t)
	for _, stmt := range []string{
		`INSERT INTO categories (id, name) VALUES (1, 'x')`,
		`INSERT INTO category_items (category_id, quote_id) VALUES (1, 'NOTANINT')`,
		`INSERT INTO collections (id, name) VALUES (1, 'x')`,
		`INSERT INTO collection_items (collection_id, quote_id, position) VALUES (1, 'NOTANINT', 1)`,
	} {
		if _, err := s.DB().Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.QuoteCategoryMap(); err == nil {
		t.Error("QuoteCategoryMap: expected scan error")
	}
	if _, err := s.QuoteCollectionMap(); err == nil {
		t.Error("QuoteCollectionMap: expected scan error")
	}
}

// TestCollectionMembersScanError drives collectionMembers' scan-error return via
// a membership row with a non-numeric quote_id.
func TestCollectionMembersScanError(t *testing.T) {
	s := newTestStore(t)
	mustCreate(t, s, quote.New("X", "X", []string{"x"}))
	for _, stmt := range []string{
		`INSERT INTO collections (id, name) VALUES (1, 'x')`,
		`INSERT INTO collection_items (collection_id, quote_id, position) VALUES (1, 'NOTANINT', 1)`,
	} {
		if _, err := s.DB().Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.AddToCollection(1, []int64{2}); err == nil {
		t.Error("AddToCollection: expected collectionMembers scan error")
	}
}

// TestStoreCategoryItemsRollback drops only category_items so Delete/DeleteMany
// get past the collection_items delete and hit the category_items rollback.
func TestStoreCategoryItemsRollback(t *testing.T) {
	s := newTestStore(t)
	q := mustCreate(t, s, quote.New("X", "X", []string{"x"}))
	if _, err := s.DB().Exec("DROP TABLE category_items"); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(q); err == nil {
		t.Error("Delete: expected category_items rollback error")
	}
	if err := s.DeleteMany([]int64{q}); err == nil {
		t.Error("DeleteMany: expected category_items rollback error")
	}
}
