// Package seed ensures the quotes database carries the canonical seed data.
// EnsureSeeded is idempotent and seeds exactly once: it never drops a populated
// table or re-seeds a database a user has emptied.
package seed

import (
	"database/sql"

	"github.com/lavantien/quotes-manager/database"
)

// EnsureSeeded brings db up to the expected state:
//   - Fresh database (no quotes table, or empty): load the canonical seed.
//   - Already seeded: leave the data untouched.
//
// A persistent app_meta marker records that seeding has happened, so a user who
// deletes every quote is not re-seeded on the next launch.
func EnsureSeeded(db *sql.DB) error {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS app_meta (key TEXT PRIMARY KEY, value TEXT NOT NULL)`); err != nil {
		return err
	}
	seeded, err := metaValue(db, "seeded")
	if err != nil {
		return err
	}

	if seeded == "" {
		hasTable, err := tableExists(db, "quotes")
		if err != nil {
			return err
		}
		empty := !hasTable
		if hasTable {
			var n int
			if err := db.QueryRow("SELECT COUNT(*) FROM quotes").Scan(&n); err != nil {
				return err
			}
			empty = n == 0
		}
		if empty {
			// Canonical seed (re)creates the table + rows.
			if _, err := db.Exec(database.SeedSQL); err != nil {
				return err
			}
		}
		if err := seedCategories(db); err != nil {
			return err
		}
		if err := seedSampleCollection(db); err != nil {
			return err
		}
		if err := setMeta(db, "seeded", "1"); err != nil {
			return err
		}
	}

	return nil
}

// seedCategories tags a handful of the shortest canonical quotes with example
// categories, so a fresh install (and docs/home.png) shows a populated sidebar
// and chip rows. It is a no-op when the categories tables are absent (e.g. the
// raw seed tests) and idempotent via INSERT OR IGNORE, so re-entering the
// seeding path after a partial failure cannot duplicate tags.
func seedCategories(db *sql.DB) error {
	has, err := tableExists(db, "categories")
	if err != nil || !has {
		return err
	}
	samples := []struct {
		name     string
		quoteIDs []int64
	}{
		{"suffering", []int64{4, 5}},
		{"renunciation", []int64{2, 3}},
		{"right view", []int64{6, 7, 8}},
	}
	for _, s := range samples {
		if _, err := db.Exec("INSERT OR IGNORE INTO categories (name) VALUES (?)", s.name); err != nil {
			return err
		}
		var cid int64
		if err := db.QueryRow("SELECT id FROM categories WHERE name = ?", s.name).Scan(&cid); err != nil {
			return err
		}
		for _, qid := range s.quoteIDs {
			if _, err := db.Exec(
				`INSERT OR IGNORE INTO category_items (category_id, quote_id) SELECT ?, id FROM quotes WHERE id = ?`,
				cid, qid); err != nil {
				return err
			}
		}
	}
	return nil
}

// seedSampleCollection creates one numbered collection holding the two shortest
// quotes, so a fresh install (and docs/home.png) shows a populated collection
// column and membership chips on those root blocks. It is a no-op when the
// collections tables are absent (raw seed tests) and when any collection already
// exists, so re-entering the seeding path cannot duplicate it.
func seedSampleCollection(db *sql.DB) error {
	has, err := tableExists(db, "collections")
	if err != nil || !has {
		return err
	}
	var n int
	if err := db.QueryRow("SELECT COUNT(*) FROM collections").Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	res, err := db.Exec("INSERT INTO collections DEFAULT VALUES")
	if err != nil {
		return err
	}
	cid, err := res.LastInsertId()
	if err != nil {
		return err
	}
	for i, qid := range []int64{1, 2} {
		if _, err := db.Exec(
			`INSERT OR IGNORE INTO collection_items (collection_id, quote_id, position) SELECT ?, id, ? FROM quotes WHERE id = ?`,
			cid, i+1, qid); err != nil {
			return err
		}
	}
	return nil
}

func tableExists(db *sql.DB, name string) (bool, error) {
	var n int
	err := db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?", name).Scan(&n)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

func metaValue(db *sql.DB, key string) (string, error) {
	var v string
	err := db.QueryRow("SELECT value FROM app_meta WHERE key = ?", key).Scan(&v)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return v, err
}

func setMeta(db *sql.DB, key, value string) error {
	_, err := db.Exec(`INSERT INTO app_meta (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key, value)
	return err
}
