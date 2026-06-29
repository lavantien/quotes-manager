// Package seed ensures the quotes database carries the canonical seed data and
// the user-owned sort_order column. EnsureSeeded is idempotent and seeds exactly
// once: it never drops a populated table or re-seeds a database a user has emptied.
package seed

import (
	"database/sql"
	"fmt"

	"github.com/lavantien/quotes-manager/database"
)

// EnsureSeeded brings db up to the expected state:
//   - Fresh database (no quotes table, or empty): load the canonical seed and
//     add sort_order (seeded from id).
//   - Legacy database (quotes table from the old seed.sql, no sort_order): keep
//     all rows and edits, add sort_order = id.
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
			// Canonical seed (re)creates the table (without sort_order) + rows.
			if _, err := db.Exec(database.SeedSQL); err != nil {
				return err
			}
		}
		hasSort, err := columnExists(db, "quotes", "sort_order")
		if err != nil {
			return err
		}
		if !hasSort {
			if _, err := db.Exec("ALTER TABLE quotes ADD COLUMN sort_order INTEGER NOT NULL DEFAULT 0"); err != nil {
				return err
			}
		}
		if _, err := db.Exec("UPDATE quotes SET sort_order = id WHERE sort_order = 0"); err != nil {
			return err
		}
		if err := setMeta(db, "seeded", "1"); err != nil {
			return err
		}
	}

	if _, err := db.Exec("CREATE INDEX IF NOT EXISTS idx_quotes_sort_order ON quotes(sort_order)"); err != nil {
		return err
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

func columnExists(db *sql.DB, table, column string) (bool, error) {
	rows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%q)", table))
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var cid, notnull, pk int
		var name, typ string
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
			return false, err
		}
		if name == column {
			return true, nil
		}
	}
	return false, rows.Err()
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
