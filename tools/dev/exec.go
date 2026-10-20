package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

type devRunner struct {
	Root   string
	Stdout io.Writer
	Stderr io.Writer
}

func newDevRunner() (*devRunner, error) {
	root, err := findRepoRoot(".")
	if err != nil {
		return nil, err
	}
	return &devRunner{Root: root, Stdout: os.Stdout, Stderr: os.Stderr}, nil
}

func findRepoRoot(start string) (string, error) {
	abs, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	dir := abs
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no go.mod found upward from %s", abs)
		}
		dir = parent
	}
}

func devEnv(goos string, environ []string, extra map[string]string) []string {
	env := map[string]string{}
	for _, kv := range environ {
		if i := strings.IndexByte(kv, '='); i > 0 {
			env[kv[:i]] = kv[i+1:]
		}
	}
	env["CGO_ENABLED"] = "1"
	if goos == "windows" {
		home := env["USERPROFILE"]
		base := filepath.ToSlash(home) + "/scoop/apps/mingw-winlibs-ucrt/current/bin"
		env["CC"] = base + "/gcc.exe"
		env["CXX"] = base + "/g++.exe"
	}
	for k, v := range extra {
		env[k] = v
	}
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]string, 0, len(env))
	for _, k := range keys {
		out = append(out, k+"="+env[k])
	}
	return out
}

func wrapPath(root, goos string) string {
	if goos != "windows" {
		return ""
	}
	return filepath.Join(root, "dist", "tools", "jobwrap.exe")
}

func jobwrapBuildArgs(root string) []string {
	return []string{"build", "-o", filepath.Join(root, "dist", "tools", "jobwrap.exe"), "./tools/jobwrap"}
}

func (r *devRunner) env(extra map[string]string) []string {
	return devEnv(runtime.GOOS, os.Environ(), extra)
}

func (r *devRunner) ensureWrap() (string, error) {
	path := wrapPath(r.Root, runtime.GOOS)
	if path == "" {
		return "", nil
	}
	if _, err := os.Stat(path); err == nil {
		return path, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	if err := r.exec("build-jobwrap", nil, "go", jobwrapBuildArgs(r.Root)...); err != nil {
		return "", err
	}
	return path, nil
}

func (r *devRunner) exec(desc string, extra map[string]string, name string, args ...string) error {
	bin, err := exec.LookPath(name)
	if err != nil {
		return fmt.Errorf("%s: %w", desc, err)
	}
	return r.execAbs(desc, extra, bin, args...)
}

func (r *devRunner) execLook(name string) (string, error) {
	bin, err := exec.LookPath(name)
	if err != nil {
		return "", err
	}
	return bin, nil
}

func (r *devRunner) execAbs(desc string, extra map[string]string, bin string, args ...string) error {
	if len(extra) == 0 {
		fmt.Fprintf(r.Stderr, "dev: run cmd=%s bin=%s args=%s\n", desc, bin, strings.Join(args, " "))
	} else {
		fmt.Fprintf(r.Stderr, "dev: run cmd=%s bin=%s args=%s env=%s\n", desc, bin, strings.Join(args, " "), envSummary(extra))
	}
	start := time.Now()
	cmd := exec.Command(bin, args...)
	cmd.Dir = r.Root
	cmd.Env = r.env(extra)
	cmd.Stdin = os.Stdin
	cmd.Stdout = r.Stdout
	cmd.Stderr = r.Stderr
	err := cmd.Run()
	dur := time.Since(start).Round(time.Millisecond)
	if err != nil {
		var exitErr *exec.ExitError
		if ok := asExitError(err, &exitErr); ok {
			fmt.Fprintf(r.Stderr, "dev: fail cmd=%s code=%d dur=%s\n", desc, exitErr.ExitCode(), dur)
		} else {
			fmt.Fprintf(r.Stderr, "dev: fail cmd=%s dur=%s err=%v\n", desc, dur, err)
		}
		return err
	}
	fmt.Fprintf(r.Stderr, "dev: exit cmd=%s code=0 dur=%s\n", desc, dur)
	return nil
}

func (r *devRunner) wrapExec(desc string, extra map[string]string, name string, args ...string) error {
	wrap, err := r.ensureWrap()
	if err != nil {
		return err
	}
	if wrap == "" {
		return r.exec(desc, extra, name, args...)
	}
	full := append([]string{name}, args...)
	return r.execAbs(desc, extra, wrap, full...)
}

func (r *devRunner) goRun(desc string, extra map[string]string, args ...string) error {
	return r.exec(desc, extra, "go", args...)
}

func (r *devRunner) goRunWrapped(desc string, extra map[string]string, args ...string) error {
	return r.wrapExec(desc, extra, "go", args...)
}

func envSummary(extra map[string]string) string {
	keys := make([]string, 0, len(extra))
	for k := range extra {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+extra[k])
	}
	return strings.Join(parts, ",")
}

func asExitError(err error, target **exec.ExitError) bool {
	if e, ok := err.(*exec.ExitError); ok {
		*target = e
		return true
	}
	return false
}
