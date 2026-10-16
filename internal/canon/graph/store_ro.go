package graph

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

func OpenRO(path string) (*Store, error) {
	if strings.ContainsRune(path, '?') {
		return nil, fmt.Errorf("open sqlite ro: path %q contains %q, unusable as a driver DSN", path, "?")
	}
	dsn := "file:" + filepath.ToSlash(path) + "?mode=ro&_query_only=1&_busy_timeout=5000"
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite ro %s: %w", path, err)
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("open sqlite ro %s: %w", path, err)
	}
	return &Store{db: db}, nil
}

func (s *Store) Checkpoint() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if _, err := s.db.ExecContext(ctx, `PRAGMA wal_checkpoint(TRUNCATE)`); err != nil {
		return fmt.Errorf("checkpoint: %w", err)
	}
	return nil
}
