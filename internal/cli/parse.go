package cli

import (
	"context"

	"github.com/lavantien/quotes-manager/internal/canon/pipeline"
	"github.com/lavantien/quotes-manager/internal/config"
)

func init() {
	Register(Command{Name: "parse", Usage: "parse the mirrors into ndjson artifacts", Run: runParse})
}

func runParse(ctx context.Context, cfg *config.Config, args []string) error {
	if err := noArgs("parse", args); err != nil {
		return err
	}
	stats, err := pipeline.Parse(ctx, cfg)
	if err != nil {
		return err
	}
	if Verbose(ctx) {
		printLine("parse: %d files, %d records, %d names, %d html segments\n",
			stats.Files, stats.Records, stats.Names, stats.HtmlSegments)
		if stats.DpdAbsent {
			printLine("parse dpd: absent\n")
		} else {
			printLine("parse dpd: %d headwords, %d roots, %d forms\n",
				stats.DpdHeadwords, stats.DpdRoots, stats.DpdForms)
		}
	}
	return nil
}
