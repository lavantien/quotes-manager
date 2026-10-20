package seed

import (
	"database/sql"

	"github.com/lavantien/quotes-manager/database"
)

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
			if _, err := db.Exec(database.SeedSQL); err != nil {
				return err
			}
		}
		if err := seedCategories(db); err != nil {
			return err
		}
		if err := setMeta(db, "seeded", "1"); err != nil {
			return err
		}
	}

	return nil
}

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
