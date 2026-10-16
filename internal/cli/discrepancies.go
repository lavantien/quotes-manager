package cli

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"

	"github.com/lavantien/quotes-manager/internal/canon/graph"
	"github.com/lavantien/quotes-manager/internal/config"
)

func init() {
	Register(Command{Name: "discrepancies", Usage: "list discrepancies: --severity --check --run", Run: runDiscrepancies})
}

var stdout io.Writer = os.Stdout

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func printLine(format string, args ...any) {
	_, _ = fmt.Fprintf(stdout, format, args...)
}

func runDiscrepancies(ctx context.Context, cfg *config.Config, args []string) error {
	fs := flag.NewFlagSet("discrepancies", flag.ContinueOnError)
	severity := fs.String("severity", "", "filter: info, warn, or error")
	check := fs.String("check", "", "filter: check id like B001")
	run := fs.Int64("run", 0, "run id (0 = latest)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	_, dbPath, err := graph.CurrentDB(cfg)
	if err != nil {
		return fmt.Errorf("discrepancies: %w", err)
	}
	store, err := graph.Open(dbPath)
	if err != nil {
		return fmt.Errorf("discrepancies: %w", err)
	}
	defer func() { _ = store.Close() }()

	runID := *run
	if runID == 0 {
		var max sql.NullInt64
		if err := store.DB().QueryRowContext(ctx, `SELECT MAX(id) FROM run`).Scan(&max); err != nil {
			return fmt.Errorf("discrepancies: latest run: %w", err)
		}
		if !max.Valid {
			return fmt.Errorf("discrepancies: current database has no run row")
		}
		runID = max.Int64
	}

	query := `SELECT id, "check", severity, muid, detail
		FROM discrepancy WHERE run_id = ?`
	var params []any
	params = append(params, runID)
	if *severity != "" {
		query += ` AND severity = ?`
		params = append(params, *severity)
	}
	if *check != "" {
		query += ` AND "check" = ?`
		params = append(params, *check)
	}
	query += ` ORDER BY id`

	rows, err := store.DB().QueryContext(ctx, query, params...)
	if err != nil {
		return fmt.Errorf("discrepancies: %w", err)
	}
	defer func() { _ = rows.Close() }()
	n := 0
	for rows.Next() {
		var id int64
		var check, sev, detail string
		var muid sql.NullString
		if err := rows.Scan(&id, &check, &sev, &muid, &detail); err != nil {
			return err
		}
		m := "-"
		if muid.Valid {
			m = muid.String
		}
		printLine("%d %s %s %s %s\n", id, check, sev, m, detail)
		n++
	}
	if err := rows.Err(); err != nil {
		return err
	}
	printLine("%d rows (run %d)\n", n, runID)
	return nil
}
