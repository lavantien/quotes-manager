package graph

import (
	"fmt"
)

type Migration struct {
	Version int
	SQL     string
}

var migrations = []Migration{
	{Version: 1, SQL: schemaV1},
	{Version: 2, SQL: `ALTER TABLE run ADD COLUMN dpd_tag TEXT;`},
}

func LatestVersion() int {
	return migrations[len(migrations)-1].Version
}

func (s *Store) Migrate() error {
	var cur int
	if err := s.db.QueryRow(`PRAGMA user_version`).Scan(&cur); err != nil {
		return fmt.Errorf("read user_version: %w", err)
	}
	want := LatestVersion()
	if cur == want {
		return nil
	}
	if cur > want {
		return fmt.Errorf("database schema version %d is newer than binary version %d", cur, want)
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for _, m := range migrations {
		if m.Version > cur {
			if _, err := tx.Exec(m.SQL); err != nil {
				return fmt.Errorf("migration %d: %w", m.Version, err)
			}
		}
	}
	if _, err := tx.Exec(fmt.Sprintf(`PRAGMA user_version = %d`, want)); err != nil {
		return fmt.Errorf("set user_version: %w", err)
	}
	return tx.Commit()
}
