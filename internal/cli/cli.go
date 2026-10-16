package cli

import (
	"context"
	"flag"
	"fmt"
	"os"
	"sort"

	"github.com/lavantien/quotes-manager/internal/config"
)

type Command struct {
	Name  string
	Usage string
	Run   func(ctx context.Context, cfg *config.Config, args []string) error
}

var registry = map[string]Command{}

func Register(c Command) { registry[c.Name] = c }

func Lookup(name string) (Command, bool) {
	c, ok := registry[name]
	return c, ok
}

func Names() []string {
	out := make([]string, 0, len(registry))
	for n := range registry {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

func Main(args []string) int {
	gf := flag.NewFlagSet("qm", flag.ContinueOnError)
	gf.SetOutput(os.Stderr)
	cfgPath := gf.String("config", "", "path to config json")
	verbose := gf.Bool("v", false, "verbose")
	if err := gf.Parse(args); err != nil {
		return 2
	}
	if gf.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "usage: qm [-v] [--config file] <command> [args]\ncommands: %v\n", Names())
		return 2
	}
	cfg, err := config.Load(*cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		return 2
	}
	name := gf.Arg(0)
	cmd, ok := Lookup(name)
	if !ok {
		fmt.Fprintf(os.Stderr, "qm %s: not implemented\n", name)
		return 2
	}
	ctx := context.Background()
	if *verbose {
		ctx = context.WithValue(ctx, verboseKey{}, true)
	}
	if err := cmd.Run(ctx, cfg, gf.Args()[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "qm %s: %v\n", name, err)
		return 1
	}
	return 0
}

type verboseKey struct{}

func Verbose(ctx context.Context) bool {
	v, _ := ctx.Value(verboseKey{}).(bool)
	return v
}
