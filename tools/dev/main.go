package main

import (
	"errors"
	"fmt"
	"io"
	"os"
)

var errUsage = errors.New("usage")

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(argv []string, out, errW io.Writer) int {
	if len(argv) == 0 || argv[0] == "help" {
		printUsage(out, commands())
		return 0
	}
	name, rest := argv[0], argv[1:]
	for _, c := range commands() {
		if c.Name != name {
			continue
		}
		if err := c.Run(rest); err != nil {
			if errors.Is(err, errUsage) {
				return 2
			}
			_, _ = fmt.Fprintf(errW, "dev: %s: %v\n", name, err)
			return 1
		}
		return 0
	}
	_, _ = fmt.Fprintf(errW, "dev: unknown subcommand %q, run without args for the list\n", name)
	return 2
}

func printUsage(out io.Writer, cmds []command) {
	_, _ = fmt.Fprintln(out, "Usage: go run ./tools/dev <subcommand> [flags]")
	_, _ = fmt.Fprintln(out, "")
	_, _ = fmt.Fprintln(out, "Subcommands:")
	for _, c := range cmds {
		_, _ = fmt.Fprintf(out, "  %-14s %s\n", c.Name, c.Desc)
	}
}
