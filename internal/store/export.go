package store

import (
	"strings"
)

// DumpVersion is the current backup format version.
const DumpVersion = 1

// Dump is a portable, whole-database snapshot: every quote plus the named
// collections and categories with their ordered memberships. It is the
// serialization format for backup/restore.
type Dump struct {
	Version     int              `json:"version"`
	Quotes      []Quote          `json:"quotes"`
	Collections []CollectionDump `json:"collections"`
	Categories  []CategoryDump   `json:"categories"`
}

// CollectionDump is a collection with its ordered quote ids.
type CollectionDump struct {
	ID    int64   `json:"id"`
	Name  string  `json:"name"`
	Items []int64 `json:"items"`
}

// CategoryDump is a category with its member quote ids.
type CategoryDump struct {
	ID    int64   `json:"id"`
	Name  string  `json:"name"`
	Items []int64 `json:"items"`
}

// Export snapshots the whole database: every quote (in home order), each
// collection with its ordered members, and each category with its members.
func (s *SQLiteStore) Export() (*Dump, error) {
	qs, err := s.List()
	if err != nil {
		return nil, err
	}
	out := &Dump{Version: DumpVersion, Quotes: qs}
	if cols, err := s.ListCollections(); err != nil {
		return nil, err
	} else {
		for _, c := range cols {
			items, err := s.CollectionQuotes(c.ID)
			if err != nil {
				return nil, err
			}
			out.Collections = append(out.Collections, CollectionDump{ID: c.ID, Name: c.Name, Items: quoteIDsOf(items)})
		}
	}
	if cats, err := s.ListCategories(); err != nil {
		return nil, err
	} else {
		for _, ca := range cats {
			items, err := s.CategoryQuotes(ca.ID)
			if err != nil {
				return nil, err
			}
			out.Categories = append(out.Categories, CategoryDump{ID: ca.ID, Name: ca.Name, Items: quoteIDsOf(items)})
		}
	}
	return out, nil
}

// Import replaces every data table with the dump's contents in a single
// transaction. Explicit ids are preserved so the canonical shortest-first
// ranking, collection positions, and category memberships survive the
// round-trip. ErrUnsupportedDump is returned (and the transaction rolled back)
// for a nil dump or an unknown version.
func (s *SQLiteStore) Import(d *Dump) error {
	if d == nil || d.Version != DumpVersion {
		return ErrUnsupportedDump
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	for _, t := range []string{"collection_items", "category_items", "quotes", "collections", "categories"} {
		if _, err := tx.Exec("DELETE FROM " + t); err != nil {
			return rollback(tx, err)
		}
	}
	for _, q := range d.Quotes {
		if _, err := tx.Exec(
			"INSERT INTO quotes (id, sutta_id, citation, body_md, body_text, line_count, char_count, sources) VALUES (?,?,?,?,?,?,?,?)",
			q.ID, q.SuttaID, q.Citation, q.BodyMD, q.BodyText, q.LineCount, q.CharCount, strings.Join(q.Sources, ";")); err != nil {
			return rollback(tx, err)
		}
	}
	for _, c := range d.Collections {
		if _, err := tx.Exec("INSERT INTO collections (id, name) VALUES (?,?)", c.ID, c.Name); err != nil {
			return rollback(tx, err)
		}
		for pos, qid := range c.Items {
			if _, err := tx.Exec("INSERT INTO collection_items (collection_id, quote_id, position) VALUES (?,?,?)", c.ID, qid, pos+1); err != nil {
				return rollback(tx, err)
			}
		}
	}
	for _, ca := range d.Categories {
		if _, err := tx.Exec("INSERT INTO categories (id, name) VALUES (?,?)", ca.ID, ca.Name); err != nil {
			return rollback(tx, err)
		}
		for _, qid := range ca.Items {
			if _, err := tx.Exec("INSERT INTO category_items (category_id, quote_id) VALUES (?,?)", ca.ID, qid); err != nil {
				return rollback(tx, err)
			}
		}
	}
	return tx.Commit()
}

func quoteIDsOf(qs []Quote) []int64 {
	out := make([]int64, len(qs))
	for i, q := range qs {
		out[i] = q.ID
	}
	return out
}
