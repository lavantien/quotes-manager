package store

import (
	"database/sql"
	"fmt"
	"strings"
)

func (s *SQLiteStore) ListSuttas() ([]Sutta, error) {
	rows, err := s.db.Query("SELECT muid, display, link, length FROM suttas ORDER BY muid ASC")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []Sutta
	for rows.Next() {
		var su Sutta
		var length sql.NullInt64
		if err := rows.Scan(&su.Muid, &su.Display, &su.Link, &length); err != nil {
			return nil, err
		}
		su.Length = int(length.Int64)
		out = append(out, su)
	}
	return out, rows.Err()
}

func (s *SQLiteStore) SetSuttaLengths(lengths map[string]int) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	for muid, length := range lengths {
		if _, err := tx.Exec("UPDATE suttas SET length = ? WHERE muid = ?", length, muid); err != nil {
			return rollback(tx, err)
		}
	}
	return tx.Commit()
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

func (s *SQLiteStore) CollectionQuoteItems(id int64) ([]QuoteItem, error) {
	rows, err := s.db.Query(
		"SELECT q."+strings.Join(quoteColumns, ", q.")+", ci.group_id"+
			" FROM quotes q JOIN collection_items ci ON ci.quote_id = q.id"+
			" WHERE ci.collection_id = ? ORDER BY ci.position ASC", id)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []QuoteItem
	for rows.Next() {
		var it QuoteItem
		var sources string
		var groupID sql.NullInt64
		if err := rows.Scan(&it.ID, &it.SuttaID, &it.Citation, &it.BodyMD, &it.BodyText, &it.LineCount, &it.CharCount, &sources, &groupID); err != nil {
			return nil, err
		}
		it.Sources = splitSources(sources)
		it.GroupID = groupID.Int64
		out = append(out, it)
	}
	return out, rows.Err()
}

func (s *SQLiteStore) CollectionSuttaItems(id int64) ([]SuttaItem, error) {
	rows, err := s.db.Query(
		`SELECT si.muid, su.display, su.link, su.length, si.position, si.group_id
		 FROM collection_sutta_items si JOIN suttas su ON su.muid = si.muid
		 WHERE si.collection_id = ? ORDER BY si.position ASC`, id)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []SuttaItem
	for rows.Next() {
		var it SuttaItem
		var length, groupID sql.NullInt64
		if err := rows.Scan(&it.Muid, &it.Display, &it.Link, &length, &it.Position, &groupID); err != nil {
			return nil, err
		}
		it.Length = int(length.Int64)
		it.GroupID = groupID.Int64
		out = append(out, it)
	}
	return out, rows.Err()
}

func (s *SQLiteStore) SetQuoteGroup(id, quoteID, groupID int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	if err := ensureGroup(tx, id, groupID); err != nil {
		return rollback(tx, err)
	}
	res, err := tx.Exec(
		"UPDATE collection_items SET group_id = ? WHERE collection_id = ? AND quote_id = ?",
		nullableGroupID(groupID), id, quoteID)
	if err != nil {
		return rollback(tx, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return rollback(tx, fmt.Errorf("%w: quote %d not in collection %d", ErrNotFound, quoteID, id))
	}
	return tx.Commit()
}

func (s *SQLiteStore) SetSuttaGroup(id int64, muid string, groupID int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	if err := ensureGroup(tx, id, groupID); err != nil {
		return rollback(tx, err)
	}
	res, err := tx.Exec(
		"UPDATE collection_sutta_items SET group_id = ? WHERE collection_id = ? AND muid = ?",
		nullableGroupID(groupID), id, muid)
	if err != nil {
		return rollback(tx, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return rollback(tx, fmt.Errorf("%w: sutta %q not in collection %d", ErrNotFound, muid, id))
	}
	return tx.Commit()
}

func ensureGroup(tx *sql.Tx, collectionID, groupID int64) error {
	if groupID == 0 {
		return nil
	}
	var one int64
	if err := tx.QueryRow(
		"SELECT 1 FROM collection_groups WHERE id = ? AND collection_id = ?", groupID, collectionID).Scan(&one); err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("%w: group %d not in collection %d", ErrNotFound, groupID, collectionID)
		}
		return err
	}
	return nil
}

func nullableGroupID(groupID int64) any {
	if groupID == 0 {
		return nil
	}
	return groupID
}

func (s *SQLiteStore) AddSuttaToCollection(id int64, muid, display, link string) error {
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
	if _, err := tx.Exec(
		"INSERT OR IGNORE INTO suttas (muid, display, link) VALUES (?, ?, ?)", muid, display, link); err != nil {
		return rollback(tx, err)
	}
	var exists int64
	if err := tx.QueryRow(
		"SELECT 1 FROM collection_sutta_items WHERE collection_id = ? AND muid = ?", id, muid).Scan(&exists); err == nil {
		return tx.Commit()
	} else if err != sql.ErrNoRows {
		return rollback(tx, err)
	}
	if _, err := tx.Exec(
		"INSERT INTO collection_sutta_items (collection_id, muid, position) VALUES (?, ?, (SELECT COALESCE(MAX(position), 0) + 1 FROM collection_sutta_items WHERE collection_id = ?))",
		id, muid, id); err != nil {
		return rollback(tx, err)
	}
	return tx.Commit()
}

func (s *SQLiteStore) RemoveSuttaFromCollection(id int64, muid string) error {
	res, err := s.db.Exec(
		"DELETE FROM collection_sutta_items WHERE collection_id = ? AND muid = ?", id, muid)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("%w: sutta %q not in collection %d", ErrNotFound, muid, id)
	}
	return nil
}

func (s *SQLiteStore) ReorderCollectionSuttas(id int64, orderedMuids []string) error {
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
	for i, muid := range orderedMuids {
		res, err := tx.Exec(
			"UPDATE collection_sutta_items SET position = ? WHERE collection_id = ? AND muid = ?",
			i+1, id, muid)
		if err != nil {
			return rollback(tx, err)
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return rollback(tx, fmt.Errorf("%w: sutta %q not in collection %d", ErrNotFound, muid, id))
		}
	}
	return tx.Commit()
}
