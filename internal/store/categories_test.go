package store

import (
	"errors"
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

func TestCreateCategoryAndList(t *testing.T) {
	s := newTestStore(t)
	id, err := s.CreateCategory("wisdom")
	if err != nil {
		t.Fatal(err)
	}
	if id == 0 {
		t.Error("category id = 0")
	}
	cats, err := s.ListCategories()
	if err != nil {
		t.Fatal(err)
	}
	if len(cats) != 1 || cats[0].ID != id || cats[0].Name != "wisdom" || cats[0].Count != 0 {
		t.Errorf("categories = %+v", cats)
	}
}

// TestListCategoriesOrderedByNameNOCASE verifies the sidebar order is
// alphabetical, case-insensitive (so "Joy" lands between "impermanence" and
// "wisdom"), with id as a stable tiebreaker.
func TestListCategoriesOrderedByNameNOCASE(t *testing.T) {
	s := newTestStore(t)
	for _, name := range []string{"wisdom", "Joy", "impermanence"} {
		if _, err := s.CreateCategory(name); err != nil {
			t.Fatal(err)
		}
	}
	cats, err := s.ListCategories()
	if err != nil {
		t.Fatal(err)
	}
	got := []string{cats[0].Name, cats[1].Name, cats[2].Name}
	want := []string{"impermanence", "Joy", "wisdom"}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("order = %v, want %v", got, want)
		}
	}
}

func TestCreateCategoryDuplicate(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.CreateCategory("wisdom"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateCategory("wisdom"); !errors.Is(err, ErrDuplicate) {
		t.Errorf("err = %v, want ErrDuplicate", err)
	}
	// COLLATE NOCASE makes the unique constraint case-insensitive.
	if _, err := s.CreateCategory("WISDOM"); !errors.Is(err, ErrDuplicate) {
		t.Errorf("err = %v, want ErrDuplicate (case-insensitive)", err)
	}
	// A failed create must not change the list.
	cats, _ := s.ListCategories()
	if len(cats) != 1 {
		t.Errorf("after duplicate create, categories = %+v", cats)
	}
}

func TestGetCategory(t *testing.T) {
	s := newTestStore(t)
	id, _ := s.CreateCategory("wisdom")
	c, err := s.GetCategory(id)
	if err != nil {
		t.Fatal(err)
	}
	if c.ID != id || c.Name != "wisdom" {
		t.Errorf("got %+v", c)
	}
	if _, err := s.GetCategory(999); !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}
