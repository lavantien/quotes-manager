package store

import (
	"strings"
)

// DumpVersion is the current backup format version.
const DumpVersion = 2

// Dump is a portable, whole-database snapshot: every quote plus the named
// collections and categories with their ordered memberships, the suttas
// registry, and the collection groups with their memberships. It is the
// serialization format for backup/restore.
type Dump struct {
	Version     int              `json:"version"`
	Quotes      []Quote          `json:"quotes"`
	Collections []CollectionDump `json:"collections"`
	Categories  []CategoryDump   `json:"categories"`
	Suttas      []Sutta          `json:"suttas"`
}

// CollectionDump is a collection with its ordered quote ids, the parallel
// group ids of those items (0 = ungrouped), its groups with their member
// quote ids, and its ordered sutta items with their group ids.
type CollectionDump struct {
	ID         int64           `json:"id"`
	Name       string          `json:"name"`
	Items      []int64         `json:"items"`
	ItemGroups []int64         `json:"item_groups"`
	Groups     []GroupDump     `json:"groups"`
	SuttaItems []SuttaItemDump `json:"sutta_items"`
}

// GroupDump is a collection group with its member quote ids in order.
type GroupDump struct {
	ID    int64   `json:"id"`
	Name  string  `json:"name"`
	Items []int64 `json:"items"`
}

// SuttaItemDump is one sutta membership with its group id (0 = ungrouped).
type SuttaItemDump struct {
	Muid    string `json:"muid"`
	GroupID int64  `json:"group_id"`
}

// CategoryDump is a category with its member quote ids.
type CategoryDump struct {
	ID    int64   `json:"id"`
	Name  string  `json:"name"`
	Items []int64 `json:"items"`
}

// Export snapshots the whole database: every quote (in home order), each
// collection with its ordered members, groups, and sutta items, the suttas
// registry, and each category with its members.
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
			cd, err := s.exportCollection(c.ID)
			if err != nil {
				return nil, err
			}
			cd.ID, cd.Name = c.ID, c.Name
			out.Collections = append(out.Collections, cd)
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
	if suttas, err := s.ListSuttas(); err != nil {
		return nil, err
	} else {
		out.Suttas = suttas
	}
	return out, nil
}

func (s *SQLiteStore) exportCollection(id int64) (CollectionDump, error) {
	var cd CollectionDump
	items, err := s.CollectionQuoteItems(id)
	if err != nil {
		return cd, err
	}
	for _, it := range items {
		cd.Items = append(cd.Items, it.ID)
		cd.ItemGroups = append(cd.ItemGroups, it.GroupID)
	}
	groups, err := s.ListCollectionGroups(id)
	if err != nil {
		return cd, err
	}
	for _, g := range groups {
		gd := GroupDump{ID: g.ID, Name: g.Name}
		for i, it := range items {
			if it.GroupID == g.ID {
				gd.Items = append(gd.Items, cd.Items[i])
			}
		}
		cd.Groups = append(cd.Groups, gd)
	}
	suttas, err := s.CollectionSuttaItems(id)
	if err != nil {
		return cd, err
	}
	for _, it := range suttas {
		cd.SuttaItems = append(cd.SuttaItems, SuttaItemDump{Muid: it.Muid, GroupID: it.GroupID})
	}
	return cd, nil
}

// Import replaces every data table with the dump's contents in a single
// transaction. Explicit ids are preserved so the canonical shortest-first
// ranking, collection positions, group memberships, and category memberships
// survive the round-trip. Version 1 dumps import without groups or suttas.
// ErrUnsupportedDump is returned (and the transaction rolled back) for a nil
// dump or an unknown version.
func (s *SQLiteStore) Import(d *Dump) error {
	if d == nil || (d.Version != 1 && d.Version != DumpVersion) {
		return ErrUnsupportedDump
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	for _, t := range []string{
		"collection_items", "collection_sutta_items", "collection_groups",
		"category_items", "quotes", "collections", "categories", "suttas",
	} {
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
	for _, su := range d.Suttas {
		var length any
		if su.Length > 0 {
			length = su.Length
		}
		if _, err := tx.Exec(
			"INSERT INTO suttas (muid, display, link, length) VALUES (?,?,?,?)", su.Muid, su.Display, su.Link, length); err != nil {
			return rollback(tx, err)
		}
	}
	for _, c := range d.Collections {
		if _, err := tx.Exec("INSERT INTO collections (id, name) VALUES (?,?)", c.ID, c.Name); err != nil {
			return rollback(tx, err)
		}
		for i, qid := range c.Items {
			var groupID any
			if i < len(c.ItemGroups) && c.ItemGroups[i] != 0 {
				groupID = c.ItemGroups[i]
			}
			if _, err := tx.Exec(
				"INSERT INTO collection_items (collection_id, quote_id, position, group_id) VALUES (?,?,?,?)",
				c.ID, qid, i+1, groupID); err != nil {
				return rollback(tx, err)
			}
		}
		for gi, g := range c.Groups {
			if _, err := tx.Exec(
				"INSERT INTO collection_groups (id, collection_id, name, position) VALUES (?,?,?,?)",
				g.ID, c.ID, g.Name, gi+1); err != nil {
				return rollback(tx, err)
			}
		}
		for si, item := range c.SuttaItems {
			var groupID any
			if item.GroupID != 0 {
				groupID = item.GroupID
			}
			if _, err := tx.Exec(
				"INSERT INTO collection_sutta_items (collection_id, muid, position, group_id) VALUES (?,?,?,?)",
				c.ID, item.Muid, si+1, groupID); err != nil {
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
