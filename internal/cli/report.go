package cli

import (
	"context"
	"flag"
	"fmt"
	"time"

	"github.com/lavantien/quotes-manager/internal/canon/pipeline"
	"github.com/lavantien/quotes-manager/internal/config"
)

func init() {
	Register(Command{Name: "report", Usage: "render the triangulation report: --out dir", Run: runReport})
}

func todayUTC() string { return time.Now().UTC().Format("2006-01-02") }

func runReport(ctx context.Context, cfg *config.Config, args []string) error {
	fs := flag.NewFlagSet("report", flag.ContinueOnError)
	out := fs.String("out", "", "output directory (default: dated reports dir)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("report takes no positional arguments: %q", fs.Arg(0))
	}
	if err := pipeline.Report(ctx, cfg, *out); err != nil {
		return err
	}
	if Verbose(ctx) {
		dir := *out
		if dir == "" {
			dir = cfg.Canon.ReportsDirFor(todayUTC())
		}
		printLine("report: %s\n", dir)
	}
	return nil
}
