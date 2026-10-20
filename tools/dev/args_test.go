package main

import (
	"reflect"
	"testing"
)

func TestGoTestArgsCarryTags(t *testing.T) {
	got := goTestArgs("./internal/config")
	want := []string{"test", "-tags", "sqlite_fts5", "./internal/config"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("goTestArgs = %v, want %v", got, want)
	}
}

func TestGoTestArgsExtraAppended(t *testing.T) {
	got := goTestArgs("./internal/cli", "-count=1", "-v")
	want := []string{"test", "-tags", "sqlite_fts5", "./internal/cli", "-count=1", "-v"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("goTestArgs = %v, want %v", got, want)
	}
}

func TestTestOneArgsAnchorTheRunPattern(t *testing.T) {
	got := testOneArgs("./internal/cli", "TestLiveEnabled")
	want := []string{"test", "-tags", "sqlite_fts5", "-run", "^TestLiveEnabled$", "-count=1", "-v", "./internal/cli"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("testOneArgs = %v, want %v", got, want)
	}
}

func TestFuzzOneArgs(t *testing.T) {
	got := fuzzOneArgs("./internal/quote", "FuzzParseCanonical", "20s", 4)
	want := []string{"test", "-tags", "sqlite_fts5", "-run", "^$", "-fuzz", "FuzzParseCanonical", "-fuzztime", "20s", "-parallel", "4", "./internal/quote"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("fuzzOneArgs = %v, want %v", got, want)
	}
}

func TestFuzzTargetsMatchMakefileInventory(t *testing.T) {
	if len(fuzzTargets) != 25 {
		t.Fatalf("fuzzTargets len = %d, want 25", len(fuzzTargets))
	}
	if fuzzTargets[0].pkg != "./internal/canon/analytics" || fuzzTargets[0].name != "FuzzAnalyticsSQLGate" {
		t.Fatalf("first fuzz target = %+v, want analytics FuzzAnalyticsSQLGate", fuzzTargets[0])
	}
	if fuzzTargets[24].pkg != "./internal/search" || fuzzTargets[24].name != "FuzzSearchParse" {
		t.Fatalf("last fuzz target = %+v, want search FuzzSearchParse", fuzzTargets[24])
	}
	seen := map[string]bool{}
	for _, ft := range fuzzTargets {
		if seen[ft.pkg+" "+ft.name] {
			t.Fatalf("duplicate fuzz target %+v", ft)
		}
		seen[ft.pkg+" "+ft.name] = true
	}
}

func TestMutateDefaultPackagesMatchMakefile(t *testing.T) {
	want := "internal/canon/norm,internal/canon/source/bilara,internal/canon/source/editions,internal/canon/source/dpd,internal/canon/triangulate,internal/canon/server,internal/canon/jobs,internal/canon/analytics,internal/canon/export,internal/quote,internal/search,internal/store,internal/server,internal/cli"
	if mutateDefaultPackages != want {
		t.Fatalf("mutateDefaultPackages = %q", mutateDefaultPackages)
	}
}

func TestVerifyOrder(t *testing.T) {
	want := []string{"test", "memtest", "fmt", "lint", "typecheck", "sloc", "fuzz", "mutate", "e2e-hermetic"}
	if !reflect.DeepEqual(verifyOrder, want) {
		t.Fatalf("verifyOrder = %v, want %v", verifyOrder, want)
	}
}

func TestMemtestArgs(t *testing.T) {
	got := memtestArgs()
	want := []string{"test", "-tags", "sqlite_fts5", "-count=1", "-run", "TestPipelineMemoryBounded|TestE2EHermetic|TestMergedHermeticE2E", "./internal/cli", "./internal/e2e", "-v", "-timeout", "15m"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("memtestArgs = %v, want %v", got, want)
	}
}

func TestE2eArgs(t *testing.T) {
	got := e2eArgs("30m")
	want := []string{"test", "-tags", "sqlite_fts5", "-count=1", "./internal/cli", "-run", "TestE2ELive", "-v", "-timeout", "30m"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("e2eArgs = %v, want %v", got, want)
	}
}

func TestE2eHermeticSteps(t *testing.T) {
	steps := e2eHermeticSteps()
	if len(steps) != 2 {
		t.Fatalf("steps len = %d, want 2", len(steps))
	}
	if !reflect.DeepEqual(steps[1], []string{"test", "-tags", "sqlite_fts5", "-count=1", "./internal/cli", "-run", "TestE2EHermetic", "-v", "-timeout", "10m"}) {
		t.Fatalf("second hermetic step = %v", steps[1])
	}
}

func TestGuardPkg(t *testing.T) {
	for _, ok := range []string{"./internal/config", "internal/config", "./...", "./internal/e2e"} {
		if err := guardPkg(ok); err != nil {
			t.Fatalf("guardPkg(%q) = %v, want nil", ok, err)
		}
	}
	for _, bad := range []string{"", "..", "../evil", "..\\evil", "C:\\Windows\\System32", "/etc", "./internal/../../secret"} {
		if err := guardPkg(bad); err == nil {
			t.Fatalf("guardPkg(%q) = nil, want error", bad)
		}
	}
}
