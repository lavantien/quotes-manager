package analytics

import (
	"context"
	"database/sql"
	"encoding/csv"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/lavantien/quotes-manager/internal/config"

	_ "github.com/duckdb/duckdb-go/v2"
)

const (
	defaultRows = 50
	maxRows     = 500
)

var ErrSidecarStale = errors.New("analytics: sidecar tables absent, rerun export")

type DB struct {
	db *sql.DB
}

func Open(cfg *config.Config) (*DB, error) {
	dsn := cfg.Canon.DuckDBFile() +
		"?access_mode=read_only&threads=" + strconv.Itoa(cfg.Server.AnalyticsThreads) +
		"&memory_limit=" + strconv.Itoa(cfg.Server.AnalyticsMemoryGB) + "GB"
	db, err := sql.Open("duckdb", dsn)
	if err != nil {
		return nil, fmt.Errorf("analytics open: %w", err)
	}
	db.SetMaxOpenConns(2)
	d := &DB{db: db}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("analytics ping: %w", err)
	}
	if err := d.checkSidecars(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return d, nil
}

func Wrap(db *sql.DB) (*DB, error) {
	d := &DB{db: db}
	if err := d.checkSidecars(); err != nil {
		return nil, err
	}
	return d, nil
}

func (d *DB) Close() error {
	return d.db.Close()
}

func (d *DB) checkSidecars() error {
	var n int
	err := d.db.QueryRow(`SELECT COUNT(*) FROM information_schema.tables
		WHERE table_schema = 'main'
		AND table_name IN ('word_occurrences', 'text_stats')`).Scan(&n)
	if err != nil {
		return fmt.Errorf("analytics sidecar check: %w", err)
	}
	if n != 2 {
		return ErrSidecarStale
	}
	return nil
}

type Table struct {
	Cols []string
	Rows [][]any
}

func ToCSV(t Table) string {
	var b strings.Builder
	w := csv.NewWriter(&b)
	_ = w.Write(t.Cols)
	for _, row := range t.Rows {
		fields := make([]string, len(row))
		for i, v := range row {
			if v != nil {
				fields[i] = fmt.Sprint(v)
			}
		}
		_ = w.Write(fields)
	}
	w.Flush()
	return b.String()
}

func (d *DB) queryTable(ctx context.Context, query string, args ...any) (Table, error) {
	rows, err := d.db.QueryContext(ctx, query, args...)
	if err != nil {
		return Table{}, err
	}
	defer func() { _ = rows.Close() }()
	cols, err := rows.Columns()
	if err != nil {
		return Table{}, err
	}
	t := Table{Cols: cols}
	for rows.Next() {
		vals := make([]any, len(cols))
		scan := make([]any, len(cols))
		for i := range vals {
			scan[i] = &vals[i]
		}
		if err := rows.Scan(scan...); err != nil {
			return Table{}, err
		}
		for i, v := range vals {
			if b, ok := v.([]byte); ok {
				vals[i] = string(b)
			}
		}
		t.Rows = append(t.Rows, vals)
	}
	return t, rows.Err()
}

func clampLimit(n int) int {
	if n <= 0 {
		return defaultRows
	}
	if n > maxRows {
		return maxRows
	}
	return n
}
