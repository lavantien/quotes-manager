package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunHelpListsEverySubcommand(t *testing.T) {
	var out, errOut bytes.Buffer
	code := run([]string{"help"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("help exit = %d, want 0", code)
	}
	got := out.String()
	if !strings.HasPrefix(got, "Usage: go run ./tools/dev <subcommand>") {
		t.Fatalf("help prefix = %q", got[:40])
	}
	for _, name := range []string{"build", "test", "test-pkg", "test-one", "race", "vet", "fmt", "lint", "lint-pkg", "typecheck", "sloc", "fuzz", "fuzz-one", "mutate", "memtest", "e2e-hermetic", "e2e", "verify", "vendor-assets", "diagrams", "stage", "run", "server", "extract", "seed", "tidy", "clean", "coverage", "screenshots", "fixture", "monitor", "all"} {
		if !strings.Contains(got, "\n  "+name+" ") {
			t.Fatalf("help output missing subcommand %q", name)
		}
	}
}

func TestRunNoArgsShowsHelp(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := run(nil, &out, &errOut); code != 0 {
		t.Fatalf("no-args exit = %d, want 0", code)
	}
	if !strings.Contains(out.String(), "Usage:") {
		t.Fatal("no-args must print usage")
	}
}

func TestRunUnknownSubcommandExitsTwo(t *testing.T) {
	var out, errOut bytes.Buffer
	code := run([]string{"nope"}, &out, &errOut)
	if code != 2 {
		t.Fatalf("unknown exit = %d, want 2", code)
	}
	if !strings.Contains(errOut.String(), "unknown subcommand") {
		t.Fatalf("stderr = %q", errOut.String())
	}
}

func TestCommandRegistryHasNoDuplicates(t *testing.T) {
	seen := map[string]bool{}
	for _, c := range commands() {
		if seen[c.Name] {
			t.Fatalf("duplicate subcommand %q", c.Name)
		}
		seen[c.Name] = true
		if c.Desc == "" {
			t.Fatalf("subcommand %q lacks description", c.Name)
		}
		if c.Run == nil {
			t.Fatalf("subcommand %q lacks run", c.Name)
		}
	}
}
