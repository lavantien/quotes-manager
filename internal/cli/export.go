package cli

import (
	"context"

	"github.com/lavantien/quotes-manager/internal/canon/export"
	"github.com/lavantien/quotes-manager/internal/config"
)

func init() {
	Register(Command{Name: "export", Usage: "rebuild the duckdb sidecar from the current database", Run: runExport})
}

func runExport(ctx context.Context, cfg *config.Config, args []string) error {
	if err := noArgs("export", args); err != nil {
		return err
	}
	counts, err := export.Rebuild(ctx, cfg)
	if err != nil {
		return err
	}
	if Verbose(ctx) {
		for _, t := range []string{"node", "edge", "run", "discrepancy", "segment_pairs", "dpd_entries", "word_occurrences", "text_stats"} {
			printLine("export %s: %d rows\n", t, counts[t])
		}
	}
	return nil
}
