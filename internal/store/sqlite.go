package store

import (
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/mattn/go-sqlite3"

	"github.com/lavantien/quotes-manager/internal/quote"
)

// schemaSQL creates the quotes table including the user-owned sort_order column.
// (Legacy databases created by database/seed.sql lack sort_order; those are
// migrated by internal/seed.)
const schemaSQL = `CREATE TABLE IF NOT EXISTS quotes (
    id          INTEGER PRIMARY KEY,
    sutta_id    TEXT    NOT NULL,
    citation    TEXT    NOT NULL,
    body_md     TEXT    NOT NULL,
    body_text   TEXT    NOT NULL,
    line_count  INTEGER NOT NULL,
    char_count  INTEGER NOT NULL,
    sources     TEXT    NOT NULL,
    sort_order  INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_quotes_sort_order ON quotes(sort_order);
CREATE TABLE IF NOT EXISTS collections (
    id INTEGER PRIMARY KEY
);
CREATE TABLE IF NOT EXISTS collection_items (
    collection_id INTEGER NOT NULL,
    quote_id      INTEGER NOT NULL,
    position      INTEGER NOT NULL,
    PRIMARY KEY (collection_id, quote_id)
);
CREATE INDEX IF NOT EXISTS idx_collection_items_collection ON collection_items(collection_id, position);
CREATE INDEX IF NOT EXISTS idx_collection_items_quote ON collection_items(quote_id);`

var quoteColumns = []string{"id", "sort_order", "sutta_id", "citation", "body_md", "body_text", "line_count", "char_count", "sources"}

var columns = strings.Join(quoteColumns, ", ")

// SQLiteStore is a Store backed by a single SQLite file.
type SQLiteStore struct {
	db *sql.DB
}

// Open opens (creating if needed) the SQLite file and ensures the schema exists.
func Open(path string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}
	// A single connection serializes writes and avoids "database is locked".
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(schemaSQL); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &SQLiteStore{db: db}, nil
}

// DB exposes the underlying connection so internal/seed can run migrations.
func (s *SQLiteStore) DB() *sql.DB { return s.db }

func (s *SQLiteStore) Close() error { return s.db.Close() }

func (s *SQLiteStore) List() ([]Quote, error) {
	rows, err := s.db.Query("SELECT " + columns + " FROM quotes ORDER BY sort_order ASC, id ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Quote
	for rows.Next() {
		q, err := scanQuote(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, q)
	}
	return out, rows.Err()
}

func (s *SQLiteStore) Get(id int64) (Quote, error) {
	row := s.db.QueryRow("SELECT "+columns+" FROM quotes WHERE id = ?", id)
	q, err := scanQuote(row)
	if err == sql.ErrNoRows {
		return Quote{}, ErrNotFound
	}
	return q, err
}

func (s *SQLiteStore) Create(q *quote.Quote) (int64, error) {
	res, err := s.db.Exec(
		`INSERT INTO quotes (sutta_id, citation, body_md, body_text, line_count, char_count, sources, sort_order)
		 VALUES (?, ?, ?, ?, ?, ?, ?, (SELECT COALESCE(MAX(sort_order), 0) + 1 FROM quotes))`,
		q.SuttaID, q.Citation, q.BodyMD(), q.BodyText(), q.LineCount(), q.CharCount(),
		strings.Join(q.Sources, ";"),
	)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	return id, nil
}

func (s *SQLiteStore) Update(id int64, q *quote.Quote) error {
	res, err := s.db.Exec(
		`UPDATE quotes SET sutta_id = ?, citation = ?, body_md = ?, body_text = ?, line_count = ?, char_count = ?, sources = ? WHERE id = ?`,
		q.SuttaID, q.Citation, q.BodyMD(), q.BodyText(), q.LineCount(), q.CharCount(),
		strings.Join(q.Sources, ";"), id,
	)
	if err != nil {
		return err
	}
	return rowsAffected(res, id)
}

func (s *SQLiteStore) Delete(id int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM collection_items WHERE quote_id = ?", id); err != nil {
		return rollback(tx, err)
	}
	res, err := tx.Exec("DELETE FROM quotes WHERE id = ?", id)
	if err != nil {
		return rollback(tx, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return rollback(tx, fmt.Errorf("%w: id %d", ErrNotFound, id))
	}
	return tx.Commit()
}

func (s *SQLiteStore) DeleteMany(ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	for _, id := range ids {
		if _, err := tx.Exec("DELETE FROM collection_items WHERE quote_id = ?", id); err != nil {
			return rollback(tx, err)
		}
		if _, err := tx.Exec("DELETE FROM quotes WHERE id = ?", id); err != nil {
			return rollback(tx, err)
		}
	}
	return tx.Commit()
}

func (s *SQLiteStore) Reorder(orderedIDs []int64) error {
	if len(orderedIDs) == 0 {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	for i, id := range orderedIDs {
		res, err := tx.Exec("UPDATE quotes SET sort_order = ? WHERE id = ?", i+1, id)
		if err != nil {
			return rollback(tx, err)
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return rollback(tx, fmt.Errorf("%w: id %d", ErrNotFound, id))
		}
	}
	return tx.Commit()
}

func (s *SQLiteStore) ListCollections() ([]Collection, error) {
	rows, err := s.db.Query(
		`SELECT c.id, (SELECT COUNT(*) FROM collection_items ci WHERE ci.collection_id = c.id)
		 FROM collections c ORDER BY c.id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Collection
	for rows.Next() {
		var c Collection
		if err := rows.Scan(&c.ID, &c.Count); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *SQLiteStore) CreateCollection(quoteIDs []int64) (int64, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	res, err := tx.Exec("INSERT INTO collections DEFAULT VALUES")
	if err != nil {
		return 0, rollback(tx, err)
	}
	cid, err := res.LastInsertId()
	if err != nil {
		return 0, rollback(tx, err)
	}
	for i, qid := range quoteIDs {
		if _, err := tx.Exec(
			"INSERT INTO collection_items (collection_id, quote_id, position) VALUES (?, ?, ?)",
			cid, qid, i+1); err != nil {
			return 0, rollback(tx, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return cid, nil
}

func (s *SQLiteStore) GetCollection(id int64) (Collection, error) {
	var c Collection
	err := s.db.QueryRow(
		`SELECT id, (SELECT COUNT(*) FROM collection_items ci WHERE ci.collection_id = id)
		 FROM collections WHERE id = ?`, id).Scan(&c.ID, &c.Count)
	if err == sql.ErrNoRows {
		return Collection{}, ErrNotFound
	}
	return c, err
}

func (s *SQLiteStore) CollectionQuotes(id int64) ([]Quote, error) {
	rows, err := s.db.Query(
		"SELECT q."+strings.Join(quoteColumns, ", q.")+
			" FROM quotes q JOIN collection_items ci ON ci.quote_id = q.id"+
			" WHERE ci.collection_id = ? ORDER BY ci.position ASC", id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Quote
	for rows.Next() {
		q, err := scanQuote(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, q)
	}
	return out, rows.Err()
}

func (s *SQLiteStore) ReorderCollection(id int64, orderedQuoteIDs []int64) error {
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
	for i, qid := range orderedQuoteIDs {
		res, err := tx.Exec(
			"UPDATE collection_items SET position = ? WHERE collection_id = ? AND quote_id = ?",
			i+1, id, qid)
		if err != nil {
			return rollback(tx, err)
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return rollback(tx, fmt.Errorf("%w: quote %d not in collection %d", ErrNotFound, qid, id))
		}
	}
	return tx.Commit()
}

func (s *SQLiteStore) DeleteCollection(id int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM collection_items WHERE collection_id = ?", id); err != nil {
		return rollback(tx, err)
	}
	res, err := tx.Exec("DELETE FROM collections WHERE id = ?", id)
	if err != nil {
		return rollback(tx, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return rollback(tx, fmt.Errorf("%w: id %d", ErrNotFound, id))
	}
	return tx.Commit()
}

type scanner interface {
	Scan(dest ...any) error
}

func scanQuote(sc scanner) (Quote, error) {
	var q Quote
	var sources string
	err := sc.Scan(&q.ID, &q.SortOrder, &q.SuttaID, &q.Citation, &q.BodyMD, &q.BodyText, &q.LineCount, &q.CharCount, &sources)
	if err != nil {
		return Quote{}, err
	}
	q.Sources = splitSources(sources)
	return q, nil
}

func splitSources(joined string) []string {
	if joined == "" {
		return nil
	}
	return strings.Split(joined, ";")
}

func rowsAffected(res sql.Result, id int64) error {
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("%w: id %d", ErrNotFound, id)
	}
	return nil
}

func rollback(tx *sql.Tx, err error) error {
	_ = tx.Rollback()
	return err
}
