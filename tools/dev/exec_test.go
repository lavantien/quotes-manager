package main

import (
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestDevEnvSetsCgoAlways(t *testing.T) {
	env := devEnv("linux", []string{"PATH=/bin", "CGO_ENABLED=0"}, nil)
	if envValue(env, "CGO_ENABLED") != "1" {
		t.Fatalf("CGO_ENABLED = %q, want 1", envValue(env, "CGO_ENABLED"))
	}
}

func TestDevEnvWindowsForcesScoopToolchain(t *testing.T) {
	env := devEnv("windows", []string{"PATH=x", "USERPROFILE=C:\\Users\\u", "CC=keep-me-not"}, nil)
	if got := envValue(env, "CC"); got != "C:/Users/u/scoop/apps/mingw-winlibs-ucrt/current/bin/gcc.exe" {
		t.Fatalf("CC = %q", got)
	}
	if got := envValue(env, "CXX"); got != "C:/Users/u/scoop/apps/mingw-winlibs-ucrt/current/bin/g++.exe" {
		t.Fatalf("CXX = %q", got)
	}
}

func TestDevEnvOtherOsLeavesToolchain(t *testing.T) {
	env := devEnv("linux", []string{"PATH=/bin"}, nil)
	if envValue(env, "CC") != "" {
		t.Fatalf("CC = %q, want empty", envValue(env, "CC"))
	}
}

func TestDevEnvExtraOverrides(t *testing.T) {
	env := devEnv("linux", []string{"PATH=/bin", "GOMEMLIMIT=1GiB"}, map[string]string{"GOMEMLIMIT": "4GiB"})
	if envValue(env, "GOMEMLIMIT") != "4GiB" {
		t.Fatalf("GOMEMLIMIT = %q, want 4GiB", envValue(env, "GOMEMLIMIT"))
	}
	if envValue(env, "PATH") != "/bin" {
		t.Fatalf("PATH = %q, want /bin", envValue(env, "PATH"))
	}
}

func envValue(env []string, key string) string {
	for _, kv := range env {
		if strings.HasPrefix(kv, key+"=") {
			return strings.TrimPrefix(kv, key+"=")
		}
	}
	return ""
}

func TestFindRepoRootWalksUpToGoMod(t *testing.T) {
	root, err := findRepoRoot(".")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(root + string(os.PathSeparator) + "go.mod"); err != nil {
		t.Fatalf("root %q lacks go.mod: %v", root, err)
	}
	if !strings.HasSuffix(root, "quotes-manager") {
		t.Fatalf("root = %q, want quotes-manager suffix", root)
	}
}

func TestWrapPathWindowsOnly(t *testing.T) {
	w := wrapPath(`C:\repo`, "windows")
	if !strings.HasSuffix(w, `dist\tools\jobwrap.exe`) {
		t.Fatalf("wrapPath windows = %q", w)
	}
	if got := wrapPath("/repo", "linux"); got != "" {
		t.Fatalf("wrapPath linux = %q, want empty", got)
	}
}

func TestJobwrapBuildArgs(t *testing.T) {
	got := jobwrapBuildArgs("C:\\repo")
	want := []string{"build", "-o", `C:\repo\dist\tools\jobwrap.exe`, "./tools/jobwrap"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("jobwrapBuildArgs = %v, want %v", got, want)
	}
}
