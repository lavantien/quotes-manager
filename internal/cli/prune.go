package cli

import (
	"context"
	"flag"
	"fmt"

	"github.com/lavantien/quotes-manager/internal/canon/sync"
	"github.com/lavantien/quotes-manager/internal/config"
)

func init() {
	Register(Command{Name: "prune", Usage: "prune the mirrors: --source bilara|editions|dpd|all (default all)", Run: runPrune})
}

func runPrune(_ context.Context, cfg *config.Config, args []string) error {
	fs := flag.NewFlagSet("prune", flag.ContinueOnError)
	source := fs.String("source", "all", "bilara|editions|dpd|all")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("prune takes no positional arguments: %q", fs.Arg(0))
	}
	kinds, err := mirrorSource(*source)
	if err != nil {
		return err
	}
	for _, kind := range kinds {
		if kind == "dpd" {
			continue
		}
		m, err := sync.New(cfg, kind)
		if err != nil {
			return err
		}
		n, err := m.Prune()
		if err != nil {
			return fmt.Errorf("prune %s: %w", kind, err)
		}
		printLine("%s: pruned %d files\n", kind, n)
	}
	return nil
}
