package cli

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/lavantien/quotes-manager/internal/config"
	"github.com/lavantien/quotes-manager/internal/coverbadge"
)

func init() {
	Register(Command{Name: "coverage", Usage: "refresh the readme coverage badge: --profile --readme", Run: runCoverage})
}

func runCoverage(_ context.Context, _ *config.Config, args []string) error {
	fs := flag.NewFlagSet("coverage", flag.ContinueOnError)
	profile := fs.String("profile", "coverage.out", "Go cover profile path")
	readme := fs.String("readme", "readme.md", "README path to update")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("coverage: unexpected argument %q", fs.Arg(0))
	}
	if _, err := coverageRun(*profile, *readme); err != nil {
		return fmt.Errorf("coverage: %w", err)
	}
	return nil
}

func coverageRun(profilePath, readmePath string) (float64, error) {
	profile, err := os.ReadFile(profilePath)
	if err != nil {
		return 0, err
	}
	pct, err := coverbadge.Pct(string(profile))
	if err != nil {
		return 0, err
	}
	readme, err := os.ReadFile(readmePath)
	if err != nil {
		return 0, err
	}
	hasMarkers := strings.Contains(string(readme), coverbadge.StartMarker) &&
		strings.Contains(string(readme), coverbadge.EndMarker)
	updated := coverbadge.RenderBadge(string(readme), pct)
	switch {
	case !hasMarkers:
		printLine("coverage %.1f%%: readme has no coverage markers; nothing written\n", pct)
	case updated == string(readme):
		printLine("coverage %.1f%%: readme badge already current\n", pct)
	default:
		if err := os.WriteFile(readmePath, []byte(updated), 0o644); err != nil {
			return pct, err
		}
		printLine("coverage %.1f%%: readme badge updated\n", pct)
	}
	return pct, nil
}
