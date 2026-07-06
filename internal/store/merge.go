package store

import (
	"database/sql"
	"fmt"
)

// MergeQuotes folds each quote in merge into keep within a single transaction:
// keep inherits every collection and category membership of the merged quotes
// (memberships they already share are skipped to respect the composite primary
// keys), and then the merged quotes are deleted. keep itself is ignored if it
// appears in merge, and duplicate merge ids are folded. An empty merge list is
// a no-op. ErrNotFound is returned and the transaction rolled back if keep or
// any merge id does not exist.
func (s *SQLiteStore) MergeQuotes(keep int64, merge []int64) error {
	clean := dedupeMergeList(keep, merge)
	if len(clean) == 0 {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	// Verify every id up front so the whole repoint is atomic: a missing id
	// rolls back any membership already moved.
	if err := assertExists(tx, keep); err != nil {
		return rollback(tx, err)
	}
	for _, id := range clean {
		if err := assertExists(tx, id); err != nil {
			return rollback(tx, err)
		}
	}
	for _, id := range clean {
		if _, err := tx.Exec("UPDATE OR IGNORE collection_items SET quote_id = ? WHERE quote_id = ?", keep, id); err != nil {
			return rollback(tx, err)
		}
		if _, err := tx.Exec("DELETE FROM collection_items WHERE quote_id = ?", id); err != nil {
			return rollback(tx, err)
		}
		if _, err := tx.Exec("UPDATE OR IGNORE category_items SET quote_id = ? WHERE quote_id = ?", keep, id); err != nil {
			return rollback(tx, err)
		}
		if _, err := tx.Exec("DELETE FROM category_items WHERE quote_id = ?", id); err != nil {
			return rollback(tx, err)
		}
		if _, err := tx.Exec("DELETE FROM quotes WHERE id = ?", id); err != nil {
			return rollback(tx, err)
		}
	}
	return tx.Commit()
}

// dedupeMergeList drops keep and any duplicates from the merge list, preserving
// first-seen order.
func dedupeMergeList(keep int64, merge []int64) []int64 {
	seen := map[int64]bool{keep: true}
	out := make([]int64, 0, len(merge))
	for _, id := range merge {
		if seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}

// assertExists returns an ErrNotFound-wrapped error if the given quote id is
// absent.
func assertExists(tx *sql.Tx, id int64) error {
	var one int
	err := tx.QueryRow("SELECT 1 FROM quotes WHERE id = ?", id).Scan(&one)
	if err == sql.ErrNoRows {
		return fmt.Errorf("%w: id %d", ErrNotFound, id)
	}
	return err
}
