package store

import (
	"sort"
	"testing"
)

// TestCategorySchemaCreatesTables confirms Open provisions the categories and
// category_items tables alongside the existing quotes/collections tables.
func TestCategorySchemaCreatesTables(t *testing.T) {
	s := newTestStore(t)
	rows, err := s.db.Query(`SELECT name FROM sqlite_master WHERE type='table' AND name IN ('categories', 'category_items')`)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer rows.Close()
	var got []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		got = append(got, name)
	}
	sort.Strings(got)
	want := []string{"categories", "category_items"}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("tables = %v, want %v", got, want)
	}
}
