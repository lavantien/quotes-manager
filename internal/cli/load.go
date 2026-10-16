package cli

import (
	"context"

	"github.com/lavantien/quotes-manager/internal/canon/pipeline"
	"github.com/lavantien/quotes-manager/internal/config"
)

func init() {
	Register(Command{Name: "load", Usage: "load the parse artifacts into a published graph database", Run: runLoad})
}

func runLoad(ctx context.Context, cfg *config.Config, args []string) error {
	if err := noArgs("load", args); err != nil {
		return err
	}
	res, err := pipeline.Load(ctx, cfg)
	if err != nil {
		return err
	}
	if Verbose(ctx) {
		printLine("load: run %s at %s\n", res.RunID, res.DBPath)
	}
	return nil
}
