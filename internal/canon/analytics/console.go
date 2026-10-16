package analytics

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

var ErrSQLRejected = errors.New("analytics: statement rejected")

func gateStatement(statement string) (string, error) {
	s := strings.TrimSpace(statement)
	s = strings.TrimSpace(strings.TrimRight(s, ";"))
	if s == "" {
		return "", fmt.Errorf("%w: empty statement", ErrSQLRejected)
	}
	if strings.Contains(s, ";") {
		return "", fmt.Errorf("%w: multiple statements", ErrSQLRejected)
	}
	first := s
	if i := strings.IndexAny(s, " \t\r\n("); i >= 0 {
		first = s[:i]
	}
	switch strings.ToUpper(first) {
	case "SELECT", "WITH":
		return s, nil
	default:
		return "", fmt.Errorf("%w: statement must start with SELECT or WITH", ErrSQLRejected)
	}
}

func (d *DB) Query(ctx context.Context, statement string, limit int) (Table, error) {
	s, err := gateStatement(statement)
	if err != nil {
		return Table{}, err
	}
	q := fmt.Sprintf("SELECT * FROM (%s) LIMIT %d", s, clampLimit(limit))
	return d.queryTable(ctx, q)
}
