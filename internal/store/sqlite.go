package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/mattn/go-sqlite3"

	"github.com/lavantien/quotes-manager/internal/quote"
)

// schemaSQL creates the quotes table. char_count is the rune-count sort key
// (home is ordered by char_count, then id); id is the seed's shortest-first rank.
const schemaSQL = `CREATE TABLE IF NOT EXISTS quotes (
    id          INTEGER PRIMARY KEY,
    sutta_id    TEXT    NOT NULL,
    citation    TEXT    NOT NULL,
    body_md     TEXT    NOT NULL,
    body_text   TEXT    NOT NULL,
    line_count  INTEGER NOT NULL,
    char_count  INTEGER NOT NULL,
    sources     TEXT    NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_quotes_char_count ON quotes(char_count);
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
CREATE INDEX IF NOT EXISTS idx_collection_items_quote ON collection_items(quote_id);
CREATE TABLE IF NOT EXISTS categories (
    id   INTEGER PRIMARY KEY,
    name TEXT NOT NULL UNIQUE COLLATE NOCASE
);
CREATE TABLE IF NOT EXISTS category_items (
    category_id INTEGER NOT NULL,
    quote_id    INTEGER NOT NULL,
    PRIMARY KEY (category_id, quote_id)
);
CREATE INDEX IF NOT EXISTS idx_category_items_category ON category_items(category_id);
CREATE INDEX IF NOT EXISTS idx_category_items_quote ON category_items(quote_id);`

var quoteColumns = []string{"id", "sutta_id", "citation", "body_md", "body_text", "line_count", "char_count", "sources"}

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
	rows, err := s.db.Query("SELECT " + columns + " FROM quotes ORDER BY char_count ASC, id ASC")
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
		`INSERT INTO quotes (sutta_id, citation, body_md, body_text, line_count, char_count, sources)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
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

// AddToCollection prepends quoteIDs onto the top of an existing collection in
// the order given, shifting existing members down so their relative order is
// preserved. Quotes already in the collection (and repeats within quoteIDs) are
// skipped — no duplicates, and the existing copy keeps its place. ErrNotFound if
// the collection does not exist.
func (s *SQLiteStore) AddToCollection(cid int64, quoteIDs []int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	var one int64
	if err := tx.QueryRow("SELECT 1 FROM collections WHERE id = ?", cid).Scan(&one); err != nil {
		if err == sql.ErrNoRows {
			return rollback(tx, fmt.Errorf("%w: collection %d", ErrNotFound, cid))
		}
		return rollback(tx, err)
	}
	members, err := collectionMembers(tx, cid)
	if err != nil {
		return rollback(tx, err)
	}
	// De-duplicate the incoming list and drop ids already in the collection,
	// preserving first-seen order.
	seen := make(map[int64]bool)
	var add []int64
	for _, qid := range quoteIDs {
		if qid <= 0 || seen[qid] || members[qid] {
			continue
		}
		seen[qid] = true
		add = append(add, qid)
	}
	if len(add) > 0 {
		if _, err := tx.Exec("UPDATE collection_items SET position = position + ? WHERE collection_id = ?", len(add), cid); err != nil {
			return rollback(tx, err)
		}
		for i, qid := range add {
			if _, err := tx.Exec(
				"INSERT INTO collection_items (collection_id, quote_id, position) VALUES (?, ?, ?)",
				cid, qid, i+1); err != nil {
				return rollback(tx, err)
			}
		}
	}
	return tx.Commit()
}

// collectionMembers returns the set of quote_ids already in a collection.
func collectionMembers(tx *sql.Tx, cid int64) (map[int64]bool, error) {
	rows, err := tx.Query("SELECT quote_id FROM collection_items WHERE collection_id = ?", cid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[int64]bool)
	for rows.Next() {
		var qid int64
		if err := rows.Scan(&qid); err != nil {
			return nil, err
		}
		out[qid] = true
	}
	return out, rows.Err()
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

// --- categories ---

func (s *SQLiteStore) ListCategories() ([]Category, error) {
	rows, err := s.db.Query(
		`SELECT c.id, c.name,
		        (SELECT COUNT(*) FROM category_items ci WHERE ci.category_id = c.id)
		 FROM categories c
		 ORDER BY c.name COLLATE NOCASE ASC, c.id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Category
	for rows.Next() {
		var c Category
		if err := rows.Scan(&c.ID, &c.Name, &c.Count); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *SQLiteStore) CreateCategory(name string) (int64, error) {
	res, err := s.db.Exec("INSERT INTO categories (name) VALUES (?)", name)
	if err != nil {
		if isUniqueViolation(err) {
			return 0, fmt.Errorf("%w: category %q", ErrDuplicate, name)
		}
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	return id, nil
}

func (s *SQLiteStore) GetCategory(id int64) (Category, error) {
	var c Category
	err := s.db.QueryRow(
		`SELECT id, name, (SELECT COUNT(*) FROM category_items ci WHERE ci.category_id = id)
		 FROM categories WHERE id = ?`, id).Scan(&c.ID, &c.Name, &c.Count)
	if err == sql.ErrNoRows {
		return Category{}, ErrNotFound
	}
	return c, err
}

func (s *SQLiteStore) RenameCategory(id int64, name string) error {
	res, err := s.db.Exec("UPDATE categories SET name = ? WHERE id = ?", name, id)
	if err != nil {
		if isUniqueViolation(err) {
			return fmt.Errorf("%w: category %q", ErrDuplicate, name)
		}
		return err
	}
	return rowsAffected(res, id)
}

// DeleteCategory removes a category and untaggs every quote that carried it;
// the quotes themselves are left intact. ErrNotFound if the category is missing.
func (s *SQLiteStore) DeleteCategory(id int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM category_items WHERE category_id = ?", id); err != nil {
		return rollback(tx, err)
	}
	res, err := tx.Exec("DELETE FROM categories WHERE id = ?", id)
	if err != nil {
		return rollback(tx, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return rollback(tx, fmt.Errorf("%w: id %d", ErrNotFound, id))
	}
	return tx.Commit()
}

// CategoryQuotes returns the quotes tagged with a category in home order
// (char_count, then id) — categories are unordered tags, so there is no
// curated position like a collection has.
func (s *SQLiteStore) CategoryQuotes(id int64) ([]Quote, error) {
	rows, err := s.db.Query(
		"SELECT q."+strings.Join(quoteColumns, ", q.")+
			" FROM quotes q JOIN category_items ci ON ci.quote_id = q.id"+
			" WHERE ci.category_id = ? ORDER BY q.char_count ASC, q.id ASC", id)
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

// SetQuoteCategories replaces a quote's category set in a single transaction:
// it validates the quote and every category id first, then deletes the old
// memberships and inserts the new ones. ErrNotFound (and no partial state) on
// an unknown quote or category id.
func (s *SQLiteStore) SetQuoteCategories(quoteID int64, categoryIDs []int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	var one int64
	if err := tx.QueryRow("SELECT 1 FROM quotes WHERE id = ?", quoteID).Scan(&one); err != nil {
		if err == sql.ErrNoRows {
			return rollback(tx, fmt.Errorf("%w: quote %d", ErrNotFound, quoteID))
		}
		return rollback(tx, err)
	}
	// De-duplicate the incoming list, dropping non-positive ids.
	seen := make(map[int64]bool)
	var ids []int64
	for _, cid := range categoryIDs {
		if cid <= 0 || seen[cid] {
			continue
		}
		seen[cid] = true
		ids = append(ids, cid)
	}
	for _, cid := range ids {
		if err := tx.QueryRow("SELECT 1 FROM categories WHERE id = ?", cid).Scan(&one); err != nil {
			if err == sql.ErrNoRows {
				return rollback(tx, fmt.Errorf("%w: category %d", ErrNotFound, cid))
			}
			return rollback(tx, err)
		}
	}
	if _, err := tx.Exec("DELETE FROM category_items WHERE quote_id = ?", quoteID); err != nil {
		return rollback(tx, err)
	}
	for _, cid := range ids {
		if _, err := tx.Exec("INSERT INTO category_items (category_id, quote_id) VALUES (?, ?)", cid, quoteID); err != nil {
			return rollback(tx, err)
		}
	}
	return tx.Commit()
}

// QuoteCategoryMap returns quote_id -> its categories in a single query, for
// rendering chip rows on a list of quotes without an N+1 lookup.
func (s *SQLiteStore) QuoteCategoryMap() (map[int64][]Category, error) {
	rows, err := s.db.Query(
		`SELECT ci.quote_id, c.id, c.name
		 FROM category_items ci
		 JOIN categories c ON c.id = ci.category_id
		 ORDER BY ci.quote_id ASC, c.name COLLATE NOCASE ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[int64][]Category)
	for rows.Next() {
		var qid int64
		var c Category
		if err := rows.Scan(&qid, &c.ID, &c.Name); err != nil {
			return nil, err
		}
		out[qid] = append(out[qid], c)
	}
	return out, rows.Err()
}

// isUniqueViolation reports whether err is a SQLite uniqueness-constraint
// failure (e.g. a duplicate category name).
func isUniqueViolation(err error) bool {
	var se sqlite3.Error
	if errors.As(err, &se) {
		return se.ExtendedCode == sqlite3.ErrConstraintUnique || se.Code == sqlite3.ErrConstraint
	}
	return false
}

type scanner interface {
	Scan(dest ...any) error
}

func scanQuote(sc scanner) (Quote, error) {
	var q Quote
	var sources string
	err := sc.Scan(&q.ID, &q.SuttaID, &q.Citation, &q.BodyMD, &q.BodyText, &q.LineCount, &q.CharCount, &sources)
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
