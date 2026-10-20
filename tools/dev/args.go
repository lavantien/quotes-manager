package main

import (
	"errors"
	"path/filepath"
	"strconv"
	"strings"
)

type fuzzTarget struct {
	pkg  string
	name string
}

var fuzzTargets = []fuzzTarget{
	{"./internal/canon/analytics", "FuzzAnalyticsSQLGate"},
	{"./internal/canon/graph", "FuzzEnsureNode"},
	{"./internal/canon/graph", "FuzzPublishPointer"},
	{"./internal/canon/graph", "FuzzSearchFTSText"},
	{"./internal/canon/graph", "FuzzSegmentsMuid"},
	{"./internal/canon/graph", "FuzzTreeDivisionKey"},
	{"./internal/canon/norm", "FuzzPaliTokenizer"},
	{"./internal/canon/report", "FuzzRenderJSONRoundTrip"},
	{"./internal/canon/server", "FuzzServerGraphCaps"},
	{"./internal/canon/server", "FuzzServerLimits"},
	{"./internal/canon/server", "FuzzServerReportPath"},
	{"./internal/canon/server", "FuzzServerSearch"},
	{"./internal/canon/server", "FuzzJobsStage"},
	{"./internal/canon/source/bilara", "FuzzBilaraSegmentJSON"},
	{"./internal/canon/source/bilara", "FuzzSegmentKey"},
	{"./internal/canon/source/dpd", "FuzzDpdExtract"},
	{"./internal/canon/source/dpd", "FuzzDpdInflections"},
	{"./internal/canon/source/editions", "FuzzHTMLSegments"},
	{"./internal/canon/triangulate", "FuzzDpdKnownForms"},
	{"./internal/canon/triangulate", "FuzzCapJoin"},
	{"./internal/canon/triangulate", "FuzzParseDateTolerant"},
	{"./internal/canon/triangulate", "FuzzSplitEnKey"},
	{"./internal/canon/web", "FuzzWebEscape"},
	{"./internal/quote", "FuzzParseCanonical"},
	{"./internal/search", "FuzzSearchParse"},
}

const mutateDefaultPackages = "internal/canon/norm,internal/canon/source/bilara,internal/canon/source/editions,internal/canon/source/dpd,internal/canon/triangulate,internal/canon/server,internal/canon/jobs,internal/canon/analytics,internal/canon/export,internal/quote,internal/search,internal/store,internal/server,internal/cli"

var verifyOrder = []string{"test", "memtest", "fmt", "lint", "typecheck", "sloc", "fuzz", "mutate", "e2e-hermetic"}

func goTestArgs(pkg string, extra ...string) []string {
	args := append([]string{"test", "-tags", "sqlite_fts5"}, pkg)
	return append(args, extra...)
}

func testOneArgs(pkg, test string) []string {
	return []string{"test", "-tags", "sqlite_fts5", "-run", "^" + test + "$", "-count=1", "-v", pkg}
}

func fuzzOneArgs(pkg, target, fuzzTime string, parallel int) []string {
	return []string{"test", "-tags", "sqlite_fts5", "-run", "^$", "-fuzz", target, "-fuzztime", fuzzTime, "-parallel", strconv.Itoa(parallel), pkg}
}

func fuzzAllArgs(ft fuzzTarget, fuzzTime string, parallel int) []string {
	return fuzzOneArgs(ft.pkg, ft.name, fuzzTime, parallel)
}

func mutateArgs(pkgs string, parallel int) []string {
	return []string{"run", "./tools/mutator", "-packages", pkgs, "-tags", "sqlite_fts5", "-parallel", strconv.Itoa(parallel)}
}

func memtestArgs() []string {
	return []string{"test", "-tags", "sqlite_fts5", "-count=1", "-run", "TestPipelineMemoryBounded|TestE2EHermetic|TestMergedHermeticE2E", "./internal/cli", "./internal/e2e", "-v", "-timeout", "15m"}
}

func e2eArgs(timeout string) []string {
	return []string{"test", "-tags", "sqlite_fts5", "-count=1", "./internal/cli", "-run", "TestE2ELive", "-v", "-timeout", timeout}
}

func e2eHermeticSteps() [][]string {
	return [][]string{
		{"test", "-tags", "sqlite_fts5", "-count=1", "./internal/e2e", "-v", "-timeout", "15m"},
		{"test", "-tags", "sqlite_fts5", "-count=1", "./internal/cli", "-run", "TestE2EHermetic", "-v", "-timeout", "10m"},
	}
}

func guardPkg(pkg string) error {
	if pkg == "" {
		return errors.New("PKG required, example ./internal/config")
	}
	if filepath.IsAbs(pkg) || strings.HasPrefix(pkg, "/") || strings.HasPrefix(pkg, `\`) {
		return errors.New("PKG must be a repo-relative package path: " + pkg)
	}
	for _, part := range strings.FieldsFunc(pkg, func(r rune) bool { return r == '/' || r == '\\' }) {
		if part == ".." {
			return errors.New("PKG must not traverse upward: " + pkg)
		}
	}
	return nil
}
