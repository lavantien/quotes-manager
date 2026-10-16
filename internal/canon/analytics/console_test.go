package analytics

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestGateStatementAcceptsSelectsAndCTEs(t *testing.T) {
	for _, s := range []string{
		"SELECT 1",
		"  select 1  ",
		"SELECT 1;",
		"SELECT 1 ;",
		"WITH t AS (SELECT 1 AS x) SELECT * FROM t",
		"with c as (select 2) select * from c;",
	} {
		clean, err := gateStatement(s)
		if err != nil {
			t.Errorf("gateStatement(%q) = %v; want accepted", s, err)
		} else if strings.Contains(clean, ";") || clean != strings.TrimSpace(clean) {
			t.Errorf("gateStatement(%q) cleaned to %q", s, clean)
		}
	}
}

func TestGateStatementRejectsEverythingElse(t *testing.T) {
	for _, s := range []string{
		"",
		"   ",
		";",
		";;",
		"UPDATE word_occurrences SET headword = 'x'",
		"DELETE FROM word_occurrences",
		"INSERT INTO word_occurrences VALUES (1)",
		"DROP TABLE word_occurrences",
		"CREATE TABLE smuggled (x INT)",
		"ALTER TABLE word_occurrences DROP COLUMN root",
		"TRUNCATE word_occurrences",
		"PRAGMA memory_limit='1GB'",
		"ATTACH 'x.db' AS x",
		"COPY word_occurrences TO 'x.parquet' (FORMAT PARQUET)",
		"SELECT 1; SELECT 2",
		"SELECT 1; DROP TABLE word_occurrences",
		"select 1 where x = 1; -- comment",
		"EXPLAIN SELECT 1",
		"(SELECT 1)",
		"/* comment */ SELECT 1",
	} {
		if _, err := gateStatement(s); err == nil {
			t.Errorf("gateStatement(%q) accepted; want rejected", s)
		}
	}
}

func TestConsoleQueryRunsSelects(t *testing.T) {
	d := openAnalytics(t, buildSidecar(t))

	tbl, err := d.Query(t.Context(), "SELECT 42 AS v", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(tbl.Cols) != 1 || tbl.Cols[0] != "v" || len(tbl.Rows) != 1 ||
		fmt.Sprint(tbl.Rows[0][0]) != "42" {
		t.Errorf("console select = %v %v", tbl.Cols, tbl.Rows)
	}

	tbl, err = d.Query(t.Context(),
		"WITH c AS (SELECT headword FROM word_occurrences) SELECT COUNT(*) AS n FROM c", 0)
	if err != nil {
		t.Fatal(err)
	}
	if tbl.Rows[0][0] != int64(6) {
		t.Errorf("cte count = %v; want 6", tbl.Rows[0][0])
	}
}

func TestConsoleQueryRejectsWrites(t *testing.T) {
	d := openAnalytics(t, buildSidecar(t))
	for _, s := range []string{
		"DELETE FROM word_occurrences",
		"UPDATE text_stats SET covered = true",
		"SELECT 1; DROP TABLE text_stats",
	} {
		if _, err := d.Query(t.Context(), s, 0); !errors.Is(err, ErrSQLRejected) {
			t.Errorf("console query %q = %v; want ErrSQLRejected", s, err)
		}
	}
}

func TestConsoleQueryEnforcesRowCap(t *testing.T) {
	d := openAnalytics(t, buildSidecar(t))

	tbl, err := d.Query(t.Context(), "SELECT * FROM range(1000)", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(tbl.Rows) != defaultRows {
		t.Errorf("default cap returned %d rows; want %d", len(tbl.Rows), defaultRows)
	}

	tbl, err = d.Query(t.Context(), "SELECT * FROM range(1000)", 1_000_000)
	if err != nil {
		t.Fatal(err)
	}
	if len(tbl.Rows) != maxRows {
		t.Errorf("oversized limit returned %d rows; want %d", len(tbl.Rows), maxRows)
	}
}

func FuzzAnalyticsSQLGate(f *testing.F) {
	for _, s := range []string{
		"SELECT 1", "", ";", ";;", "SELECT 1; SELECT 2",
		"UPDATE t SET x = 1", "WITH c AS (SELECT 1) SELECT * FROM c",
		"  select 1 ", "select 1;--", "DROP TABLE x; SELECT 1",
		"(select 1)", "PRAGMA version", "/* c */ select 1",
		"SELECT * FROM t WHERE x = ';'",
		"with x as (select ';') select * from x",
		"SELECT 'with' AS kw", "select\ta\nfrom t",
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, statement string) {
		_, err := gateStatement(statement)
		accepted := err == nil
		if !accepted && !errors.Is(err, ErrSQLRejected) {
			t.Fatalf("gate returned a foreign error: %v", err)
		}
		if accepted {
			trimmed := strings.TrimRight(strings.TrimSpace(statement), ";")
			lower := strings.ToLower(trimmed)
			reads := strings.HasPrefix(lower, "select") || strings.HasPrefix(lower, "with")
			if !reads || strings.Contains(trimmed, ";") {
				t.Fatalf("gate accepted non-conforming %q", statement)
			}
		}
	})
}
