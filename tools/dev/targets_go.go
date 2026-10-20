package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

func envDefault(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}

func newFlagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	return fs
}

func parseFlags(fs *flag.FlagSet, args []string) error {
	if err := fs.Parse(args); err != nil {
		return errUsage
	}
	if len(fs.Args()) > 0 {
		fmt.Fprintf(os.Stderr, "dev: %s: unexpected arguments %v\n", fs.Name(), fs.Args())
		return errUsage
	}
	return nil
}

func binName(root string) string {
	suffix := ""
	if runtime.GOOS == "windows" {
		suffix = ".exe"
	}
	return filepath.Join(root, "dist", "qm"+suffix)
}

func goCommands() []command {
	return []command{
		{"build", "build the qm binary into dist/", runBuild},
		{"test", "run the full test suite", runTestAll},
		{"test-pkg", "run one package's tests: -pkg ./internal/config", runTestPkg},
		{"test-one", "run one named test: -pkg ./internal/cli -test TestLiveEnabled", runTestOne},
		{"race", "go test -race over the merged-app core", runRace},
		{"vet", "go vet", runVet},
		{"fmt", "gofumpt all Go files", runFmt},
		{"lint", "golangci-lint run", runLint},
		{"lint-pkg", "lint one package: -pkg ./internal/config", runLintPkg},
		{"typecheck", "compile every package, then go vet", runTypecheck},
		{"sloc", "enforce the 1000-line ceiling on every Go file", runSloc},
		{"tidy", "go mod tidy", runTidy},
	}
}

func runBuild(args []string) error {
	if err := parseFlags(newFlagSet("build"), args); err != nil {
		return err
	}
	r, err := newDevRunner()
	if err != nil {
		return err
	}
	return r.goRun("build", nil, "build", "-tags", "sqlite_fts5", "-o", binName(r.Root), "./cmd/qm")
}

func runTestAll(args []string) error {
	if err := parseFlags(newFlagSet("test"), args); err != nil {
		return err
	}
	r, err := newDevRunner()
	if err != nil {
		return err
	}
	return r.goRunWrapped("test", softMemEnv(), goTestArgs("./...")...)
}

func runTestPkg(args []string) error {
	fs := newFlagSet("test-pkg")
	pkg := fs.String("pkg", envDefault("PKG", ""), "package to test")
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	if err := guardPkg(*pkg); err != nil {
		return err
	}
	r, err := newDevRunner()
	if err != nil {
		return err
	}
	return r.goRunWrapped("test-pkg", softMemEnv(), goTestArgs(*pkg)...)
}

func runTestOne(args []string) error {
	fs := newFlagSet("test-one")
	pkg := fs.String("pkg", envDefault("PKG", ""), "package holding the test")
	test := fs.String("test", envDefault("TEST", ""), "test name to run")
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	if err := guardPkg(*pkg); err != nil {
		return err
	}
	if *test == "" {
		fmt.Fprintln(os.Stderr, "dev: test-one: -test required")
		return errUsage
	}
	r, err := newDevRunner()
	if err != nil {
		return err
	}
	return r.goRunWrapped("test-one", softMemEnv(), testOneArgs(*pkg, *test)...)
}

func runRace(args []string) error {
	fs := newFlagSet("race")
	pkgs := fs.String("pkgs", envDefault("RACE_PKGS", "./internal/quote ./internal/search ./internal/store ./internal/server ./internal/cli ./internal/e2e"), "space separated packages")
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	r, err := newDevRunner()
	if err != nil {
		return err
	}
	raceArgs := append([]string{"test", "-tags", "sqlite_fts5", "-race", "-count=1"}, strings.Fields(*pkgs)...)
	return r.goRunWrapped("race", softMemEnv(), raceArgs...)
}

func runVet(args []string) error {
	if err := parseFlags(newFlagSet("vet"), args); err != nil {
		return err
	}
	r, err := newDevRunner()
	if err != nil {
		return err
	}
	return r.goRun("vet", nil, "vet", "-tags", "sqlite_fts5", "./...")
}

func runFmt(args []string) error {
	if err := parseFlags(newFlagSet("fmt"), args); err != nil {
		return err
	}
	r, err := newDevRunner()
	if err != nil {
		return err
	}
	return r.exec("fmt", nil, "gofumpt", "-w", ".")
}

func runLint(args []string) error {
	if err := parseFlags(newFlagSet("lint"), args); err != nil {
		return err
	}
	r, err := newDevRunner()
	if err != nil {
		return err
	}
	return r.exec("lint", nil, "golangci-lint", "run")
}

func runLintPkg(args []string) error {
	fs := newFlagSet("lint-pkg")
	pkg := fs.String("pkg", envDefault("PKG", ""), "package to lint")
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	if err := guardPkg(*pkg); err != nil {
		return err
	}
	r, err := newDevRunner()
	if err != nil {
		return err
	}
	return r.exec("lint-pkg", nil, "golangci-lint", "run", *pkg)
}

func runTypecheck(args []string) error {
	if err := parseFlags(newFlagSet("typecheck"), args); err != nil {
		return err
	}
	r, err := newDevRunner()
	if err != nil {
		return err
	}
	if err := r.goRun("typecheck-build", nil, "build", "-tags", "sqlite_fts5", "./..."); err != nil {
		return err
	}
	return r.goRun("typecheck-vet", nil, "vet", "-tags", "sqlite_fts5", "./...")
}

func runSloc(args []string) error {
	if err := parseFlags(newFlagSet("sloc"), args); err != nil {
		return err
	}
	r, err := newDevRunner()
	if err != nil {
		return err
	}
	return r.goRun("sloc", nil, "run", "./tools/sloc")
}

func runTidy(args []string) error {
	if err := parseFlags(newFlagSet("tidy"), args); err != nil {
		return err
	}
	r, err := newDevRunner()
	if err != nil {
		return err
	}
	return r.goRun("tidy", nil, "mod", "tidy")
}
