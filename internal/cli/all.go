package cli

import (
	"context"
	"fmt"

	"github.com/lavantien/quotes-manager/internal/config"
)

func init() {
	Register(Command{Name: "all", Usage: "sync, parse, load, triangulate, export, report", Run: runAll})
}

func runAll(ctx context.Context, cfg *config.Config, args []string) error {
	if err := noArgs("all", args); err != nil {
		return err
	}
	steps := []struct {
		name string
		run  func(context.Context, *config.Config, []string) error
	}{
		{"sync", runSync},
		{"parse", runParse},
		{"load", runLoad},
		{"triangulate", runTriangulate},
		{"export", runExport},
		{"report", runReport},
	}
	for _, s := range steps {
		if err := s.run(ctx, cfg, args); err != nil {
			return fmt.Errorf("%s: %w", s.name, err)
		}
	}
	return nil
}
