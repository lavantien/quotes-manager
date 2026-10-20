//go:build windows

package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

func buildJobwrap(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "jobwrap.exe")
	out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput()
	if err != nil {
		t.Fatalf("build jobwrap: %v\n%s", err, out)
	}
	return bin
}

func TestJobwrapExitCodePassesThrough(t *testing.T) {
	bin := buildJobwrap(t)
	cmd := exec.Command(bin, "cmd", "/c", "exit 7")
	if out, err := cmd.CombinedOutput(); err == nil {
		t.Fatalf("exit 7 wrapped = clean exit: %s", out)
	} else if code := cmd.ProcessState.ExitCode(); code != 7 {
		t.Fatalf("wrapped exit code = %d (err %v), want 7", code, err)
	}
}

func TestJobwrapKillsTreeWhenKilled(t *testing.T) {
	bin := buildJobwrap(t)
	wrapped := exec.Command(bin, os.Args[0], "-test.run=TestJobwrapHelperProcess")
	wrapped.Env = append(os.Environ(), "GO_WANT_JOBWRAP_HELPER=1")
	stdout, err := wrapped.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := wrapped.Start(); err != nil {
		t.Fatal(err)
	}
	pids := []*os.Process{wrapped.Process}
	t.Cleanup(func() {
		for _, p := range pids {
			if p != nil {
				_ = p.Kill()
			}
		}
	})
	line, err := bufio.NewReader(stdout).ReadString('\n')
	if err != nil {
		t.Fatalf("read helper pids: %v", err)
	}
	fields := strings.Fields(line)
	if len(fields) != 2 {
		t.Fatalf("helper pid line %q, want helper and child pid", line)
	}
	var kidPID int
	if kidPID, err = strconv.Atoi(fields[1]); err != nil {
		t.Fatalf("child pid %q: %v", fields[1], err)
	}

	if err := wrapped.Process.Kill(); err != nil {
		t.Fatalf("kill jobwrap: %v", err)
	}
	deadline := time.Now().Add(15 * time.Second)
	for _, pid := range []uint32{uint32(wrapped.Process.Pid), uint32(kidPID)} {
		if !waitGone(pid, deadline) {
			t.Fatalf("pid %d survived the jobwrap kill, want death-with-parent", pid)
		}
	}
}

func TestParseJobMemArgs(t *testing.T) {
	cases := []struct {
		in      []string
		mem     uint64
		wantCmd []string
	}{
		{[]string{"cmd", "/c", "exit"}, 0, []string{"cmd", "/c", "exit"}},
		{[]string{"-jobmem", "1024", "cmd"}, 1024, []string{"cmd"}},
		{[]string{"-jobmem", "0", "go", "version"}, 0, []string{"go", "version"}},
	}
	for _, c := range cases {
		mem, cmd := parseJobMemArgs(c.in)
		if mem != c.mem || !equalStrings(cmd, c.wantCmd) {
			t.Fatalf("parseJobMemArgs(%v) = %d %v, want %d %v", c.in, mem, cmd, c.mem, c.wantCmd)
		}
	}
}

func TestJobwrapJobMemCapAllowsFitAndRejectsExcess(t *testing.T) {
	bin := buildJobwrap(t)
	fits := exec.Command(bin, "-jobmem", "1073741824", "cmd", "/c", "exit 0")
	if out, err := fits.CombinedOutput(); err != nil {
		t.Fatalf("1GiB cap wrapping cmd exit 0: %v %s", err, out)
	}
	tooSmall := exec.Command(bin, "-jobmem", "1048576", "go", "version")
	if out, err := tooSmall.CombinedOutput(); err == nil {
		t.Fatalf("1MiB cap wrapping go version = clean exit: %s", out)
	}
}

func TestJobwrapHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_JOBWRAP_HELPER") != "1" {
		t.Skip()
	}
	kid := exec.Command(os.Args[0], "-test.run=TestJobwrapSleeperProcess")
	kid.Env = append(os.Environ(), "GO_WANT_JOBWRAP_SLEEPER=1")
	if err := kid.Start(); err != nil {
		os.Exit(3)
	}
	fmt.Println(os.Getpid(), kid.Process.Pid)
	time.Sleep(10 * time.Minute)
	os.Exit(0)
}

func TestJobwrapSleeperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_JOBWRAP_SLEEPER") != "1" {
		t.Skip()
	}
	time.Sleep(10 * time.Minute)
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func waitGone(pid uint32, deadline time.Time) bool {
	for time.Now().Before(deadline) {
		h, err := windows.OpenProcess(windows.SYNCHRONIZE, false, pid)
		if err != nil {
			return true
		}
		event, _ := windows.WaitForSingleObject(h, 200)
		_ = windows.CloseHandle(h)
		if event == windows.WAIT_OBJECT_0 {
			return true
		}
	}
	return false
}
