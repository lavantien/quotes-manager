package cli

import (
	"context"
	"fmt"
	"io"
	"slices"
	"sync"

	"github.com/lavantien/quotes-manager/internal/config"
)

var pipelineStages = []string{"sync", "prune", "parse", "load", "triangulate", "export", "report", "all"}

func Stages() []string { return slices.Clone(pipelineStages) }

var stageMu sync.Mutex

func noArgs(stage string, args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("%s takes no arguments: %q", stage, args[0])
	}
	return nil
}

func RunStage(ctx context.Context, cfg *config.Config, name string, args []string, out io.Writer) error {
	if !slices.Contains(pipelineStages, name) {
		return fmt.Errorf("runstage: %q is not a pipeline stage", name)
	}
	cmd, ok := Lookup(name)
	if !ok {
		return fmt.Errorf("runstage: stage %q not registered", name)
	}
	ctx = context.WithValue(ctx, verboseKey{}, true)

	stageMu.Lock()
	defer stageMu.Unlock()
	old := stdout
	stdout = out
	defer func() { stdout = old }()
	return cmd.Run(ctx, cfg, args)
}
