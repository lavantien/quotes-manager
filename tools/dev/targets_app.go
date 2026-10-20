package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func appCommands() []command {
	return []command{
		{"all", "run the full test suite", runTestAll},
		{"stage", "run one pipeline stage via the built binary: -stage parse", runStage},
		{"run", "run the web server: -addr 127.0.0.1:8080 -db database/quotes.db", runServe},
		{"server", "run the web server: -addr 127.0.0.1:8080 -db database/quotes.db", runServe},
		{"extract", "regenerate database/seed.sql plus exports/shortest-first.md from dumps/", runExtract},
		{"seed", "delete the database so the next run re-seeds it from seed.sql", runSeed},
		{"clean", "remove generated artifacts (database, binaries)", runClean},
		{"coverage", "recompute Go test coverage and refresh the readme coverage badge", runCoverage},
		{"screenshots", "capture the UI journey into project-pictures/ (needs git, Chrome or Edge)", runScreenshots},
		{"fixture", "regenerate the clone-of-main test fixture (internal/store/storetest/testdata)", runFixture},
		{"monitor", "memory guard over project processes: -for 5s", runMonitor},
	}
}

func runStage(args []string) error {
	fs := newFlagSet("stage")
	stage := fs.String("stage", envDefault("STAGE", ""), "pipeline stage: sync prune parse load triangulate export report all")
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	if *stage == "" {
		fmt.Fprintln(os.Stderr, "dev: stage: -stage required: sync prune parse load triangulate export report all")
		return errUsage
	}
	if err := runBuild(nil); err != nil {
		return err
	}
	r, err := newDevRunner()
	if err != nil {
		return err
	}
	return r.execAbs("stage "+*stage, nil, binName(r.Root), *stage)
}

func runServe(args []string) error {
	fs := newFlagSet("run")
	addr := fs.String("addr", envDefault("ADDR", "127.0.0.1:8080"), "listen address")
	db := fs.String("db", envDefault("DB", "database/quotes.db"), "quotes database path")
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	r, err := newDevRunner()
	if err != nil {
		return err
	}
	return r.goRun("serve", nil, "run", "-tags", "sqlite_fts5", "./cmd/qm", "serve", "--addr", *addr, "--db", *db)
}

func runExtract(args []string) error {
	if err := parseFlags(newFlagSet("extract"), args); err != nil {
		return err
	}
	r, err := newDevRunner()
	if err != nil {
		return err
	}
	return r.goRun("extract", nil, "run", "-tags", "sqlite_fts5", "./cmd/qm", "extract")
}

func runSeed(args []string) error {
	if err := parseFlags(newFlagSet("seed"), args); err != nil {
		return err
	}
	db := envDefault("DB", "database/quotes.db")
	if err := os.Remove(db); err != nil && !os.IsNotExist(err) {
		return err
	}
	fmt.Println("database removed; the next run will re-seed it")
	return nil
}

func runClean(args []string) error {
	if err := parseFlags(newFlagSet("clean"), args); err != nil {
		return err
	}
	db := envDefault("DB", "database/quotes.db")
	if err := os.Remove(db); err != nil && !os.IsNotExist(err) {
		return err
	}
	for _, pattern := range []string{"*.exe", "*.test", "*.out"} {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return err
		}
		for _, m := range matches {
			if err := os.Remove(m); err != nil && !os.IsNotExist(err) {
				return err
			}
		}
	}
	return os.RemoveAll("dist")
}

func runCoverage(args []string) error {
	if err := parseFlags(newFlagSet("coverage"), args); err != nil {
		return err
	}
	r, err := newDevRunner()
	if err != nil {
		return err
	}
	if err := r.goRun("coverage-test", nil, "test", "-tags", "sqlite_fts5", "-coverpkg=./...", "-coverprofile=coverage.out", "./..."); err != nil {
		return err
	}
	if err := r.goRun("coverage-badge", nil, "run", "-tags", "sqlite_fts5", "./cmd/qm", "coverage", "-profile=coverage.out", "-readme=readme.md"); err != nil {
		return err
	}
	return os.Remove("coverage.out")
}

func runScreenshots(args []string) error {
	if err := parseFlags(newFlagSet("screenshots"), args); err != nil {
		return err
	}
	if err := runBuild(nil); err != nil {
		return err
	}
	r, err := newDevRunner()
	if err != nil {
		return err
	}
	return r.execAbs("screenshots", nil, binName(r.Root), "screenshot")
}

func runFixture(args []string) error {
	if err := parseFlags(newFlagSet("fixture"), args); err != nil {
		return err
	}
	r, err := newDevRunner()
	if err != nil {
		return err
	}
	return r.goRun("fixture", nil, "run", "-tags", "sqlite_fts5", "./cmd/qm", "fixture")
}

func runMonitor(args []string) error {
	fs := newFlagSet("monitor")
	memcap := fs.String("cap", envDefault("MEMCAP", "16GB"), "combined working-set cap")
	runFor := fs.Duration("for", 0, "stop after this duration, 0 means until killed")
	poll := fs.Duration("poll", 2*time.Second, "sampling interval")
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	r, err := newDevRunner()
	if err != nil {
		return err
	}
	return r.goRun("monitor", nil, "run", "./tools/memcapmon", "-cap", *memcap, "-poll", poll.String(), "-for", runFor.String(), "-root", r.Root)
}
