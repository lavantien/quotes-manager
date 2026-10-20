package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

func softMemEnv() map[string]string {
	return map[string]string{"GOMEMLIMIT": envDefault("MEMSOFT", "4GiB")}
}

func hardMemEnv() map[string]string {
	return map[string]string{"GOMEMLIMIT": envDefault("GOMEMLIMIT", "512MiB")}
}

func qualityCommands() []command {
	return []command{
		{"fuzz", "fuzz every target in the repo for -fuzztime each", runFuzzAll},
		{"fuzz-one", "fuzz one target: -pkg ./internal/quote -target FuzzParseCanonical", runFuzzOne},
		{"mutate", "mutation-test -pkgs, every surviving mutant fails the build", runMutate},
		{"memtest", "hermetic pipeline plus merged e2e under a hard GOMEMLIMIT", runMemtest},
		{"e2e-hermetic", "merged-app hermetic e2e (internal/e2e plus the cli hermetic pass)", runE2EHermetic},
		{"e2e", "live end-to-end pipeline (network, QM_E2E_LIVE gate)", runE2ELive},
		{"verify", "run the full quality chain, fail-fast", runVerify},
	}
}

func fuzzFlags(fs *flag.FlagSet) (*string, *int) {
	time := fs.String("fuzztime", envDefault("FUZZ_TIME", "20s"), "duration per fuzz target")
	parallel := fs.Int("parallel", 4, "fuzz workers")
	return time, parallel
}

func runFuzzAll(args []string) error {
	fs := newFlagSet("fuzz")
	fuzzTime, parallel := fuzzFlags(fs)
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	r, err := newDevRunner()
	if err != nil {
		return err
	}
	for _, ft := range fuzzTargets {
		desc := "fuzz " + ft.name
		if err := r.goRunWrapped(desc, softMemEnv(), fuzzAllArgs(ft, *fuzzTime, *parallel)...); err != nil {
			return err
		}
	}
	return nil
}

func runFuzzOne(args []string) error {
	fs := newFlagSet("fuzz-one")
	pkg := fs.String("pkg", envDefault("PKG", ""), "package holding the fuzz target")
	target := fs.String("target", envDefault("FUZZ_TARGET", ""), "fuzz target name")
	fuzzTime, parallel := fuzzFlags(fs)
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	if err := guardPkg(*pkg); err != nil {
		return err
	}
	if *target == "" {
		fmt.Fprintln(os.Stderr, "dev: fuzz-one: -target required")
		return errUsage
	}
	r, err := newDevRunner()
	if err != nil {
		return err
	}
	return r.goRunWrapped("fuzz-one", softMemEnv(), fuzzOneArgs(*pkg, *target, *fuzzTime, *parallel)...)
}

func runMutate(args []string) error {
	fs := newFlagSet("mutate")
	pkgs := fs.String("pkgs", envDefault("MUTATE_PKGS", mutateDefaultPackages), "comma separated package list")
	parallel := fs.Int("parallel", 8, "mutation workers")
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	r, err := newDevRunner()
	if err != nil {
		return err
	}
	return r.goRun("mutate", softMemEnv(), mutateArgs(*pkgs, *parallel)...)
}

func runMemtest(args []string) error {
	if err := parseFlags(newFlagSet("memtest"), args); err != nil {
		return err
	}
	r, err := newDevRunner()
	if err != nil {
		return err
	}
	return r.goRunWrapped("memtest", hardMemEnv(), memtestArgs()...)
}

func runE2EHermetic(args []string) error {
	if err := parseFlags(newFlagSet("e2e-hermetic"), args); err != nil {
		return err
	}
	r, err := newDevRunner()
	if err != nil {
		return err
	}
	for i, step := range e2eHermeticSteps() {
		if err := r.goRunWrapped(fmt.Sprintf("e2e-hermetic-%d", i+1), nil, step...); err != nil {
			return err
		}
	}
	return nil
}

func runE2ELive(args []string) error {
	fs := newFlagSet("e2e")
	timeout := fs.String("timeout", envDefault("E2E_TIMEOUT", "30m"), "overall timeout")
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	r, err := newDevRunner()
	if err != nil {
		return err
	}
	env := map[string]string{"QM_E2E_LIVE": "1"}
	return r.goRunWrapped("e2e", env, e2eArgs(*timeout)...)
}

func runVerify(args []string) error {
	fs := newFlagSet("verify")
	only := fs.String("only", "", "comma separated subset of the chain steps")
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	steps := verifyOrder
	if *only != "" {
		steps = splitComma(*only)
	}
	for _, name := range steps {
		fmt.Fprintf(os.Stderr, "dev: verify step=%s\n", name)
		if err := dispatch(name, nil); err != nil {
			return fmt.Errorf("verify step %s: %w", name, err)
		}
	}
	return nil
}

func splitComma(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
