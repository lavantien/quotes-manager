package store

import (
	"database/sql"
	"fmt"
	"strings"
)

func (s *SQLiteStore) ListSuttas() ([]Sutta, error) {
	rows, err := s.db.Query("SELECT muid, display, link FROM suttas ORDER BY muid ASC")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []Sutta
	for rows.Next() {
		var su Sutta
		if err := rows.Scan(&su.Muid, &su.Display, &su.Link); err != nil {
			return nil, err
		}
		out = append(out, su)
	}
	return out, rows.Err()
}

func (s *SQLiteStore) ListCollectionGroups(id int64) ([]Group, error) {
	rows, err := s.db.Query(
		`SELECT g.id, g.name,
		        (SELECT COUNT(*) FROM collection_items ci
		         WHERE ci.collection_id = g.collection_id AND ci.group_id = g.id)
		        + (SELECT COUNT(*) FROM collection_sutta_items si
		           WHERE si.collection_id = g.collection_id AND si.group_id = g.id)
		 FROM collection_groups g WHERE g.collection_id = ? ORDER BY g.position ASC`, id)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []Group
	for rows.Next() {
		var g Group
		if err := rows.Scan(&g.ID, &g.Name, &g.Count); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

func (s *SQLiteStore) CreateCollectionGroup(id int64, name string) (int64, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return 0, fmt.Errorf("%w: collection group", ErrEmptyName)
	}
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	var one int64
	if err := tx.QueryRow("SELECT 1 FROM collections WHERE id = ?", id).Scan(&one); err != nil {
		if err == sql.ErrNoRows {
			return 0, rollback(tx, fmt.Errorf("%w: collection %d", ErrNotFound, id))
		}
		return 0, rollback(tx, err)
	}
	res, err := tx.Exec(
		"INSERT INTO collection_groups (collection_id, name, position) VALUES (?, ?, (SELECT COALESCE(MAX(position), 0) + 1 FROM collection_groups WHERE collection_id = ?))",
		id, trimmed, id)
	if err != nil {
		return 0, rollback(tx, err)
	}
	gid, err := res.LastInsertId()
	if err != nil {
		return 0, rollback(tx, err)
	}
	return gid, tx.Commit()
}

func (s *SQLiteStore) RenameCollectionGroup(id, groupID int64, name string) error {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return fmt.Errorf("%w: collection group", ErrEmptyName)
	}
	res, err := s.db.Exec(
		"UPDATE collection_groups SET name = ? WHERE id = ? AND collection_id = ?", trimmed, groupID, id)
	if err != nil {
		return err
	}
	return rowsAffected(res, groupID)
}

func (s *SQLiteStore) DeleteCollectionGroup(id, groupID int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	if _, err := tx.Exec(
		"UPDATE collection_items SET group_id = NULL WHERE collection_id = ? AND group_id = ?", id, groupID); err != nil {
		return rollback(tx, err)
	}
	if _, err := tx.Exec(
		"UPDATE collection_sutta_items SET group_id = NULL WHERE collection_id = ? AND group_id = ?", id, groupID); err != nil {
		return rollback(tx, err)
	}
	res, err := tx.Exec("DELETE FROM collection_groups WHERE collection_id = ? AND id = ?", id, groupID)
	if err != nil {
		return rollback(tx, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return rollback(tx, fmt.Errorf("%w: group %d not in collection %d", ErrNotFound, groupID, id))
	}
	return tx.Commit()
}

func (s *SQLiteStore) ReorderCollectionGroups(id int64, orderedGroupIDs []int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	var one int64
	if err := tx.QueryRow("SELECT 1 FROM collections WHERE id = ?", id).Scan(&one); err != nil {
		if err == sql.ErrNoRows {
			return rollback(tx, fmt.Errorf("%w: collection %d", ErrNotFound, id))
		}
		return rollback(tx, err)
	}
	for i, gid := range orderedGroupIDs {
		res, err := tx.Exec(
			"UPDATE collection_groups SET position = ? WHERE collection_id = ? AND id = ?",
			i+1, id, gid)
		if err != nil {
			return rollback(tx, err)
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return rollback(tx, fmt.Errorf("%w: group %d not in collection %d", ErrNotFound, gid, id))
		}
	}
	return tx.Commit()
}
