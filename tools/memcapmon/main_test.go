//go:build windows

package main

import (
	"os/exec"
	"regexp"
	"testing"
	"time"
)

func TestParseCap(t *testing.T) {
	cases := []struct {
		in      string
		want    uint64
		wantErr bool
	}{
		{"8GB", 8 << 30, false},
		{"8 GB", 8 << 30, false},
		{"512MiB", 512 << 20, false},
		{"16KiB", 16 << 10, false},
		{"2B", 2, false},
		{"1024", 1024, false},
		{"8XB", 0, true},
		{"", 0, true},
		{"-1GB", 0, true},
		{"GB", 0, true},
	}
	for _, tc := range cases {
		got, err := parseCap(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Fatalf("parseCap(%q) = %d, want error", tc.in, got)
			}
			continue
		}
		if err != nil {
			t.Fatalf("parseCap(%q): %v", tc.in, err)
		}
		if got != tc.want {
			t.Fatalf("parseCap(%q) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

func TestShouldMatch(t *testing.T) {
	re := regexp.MustCompile(defaultMatch)
	has := func(name, cmd string) {
		t.Helper()
		if !shouldMatch(procInfo{name: name, cmd: cmd}, re) {
			t.Errorf("(%q,%q) must match", name, cmd)
		}
	}
	lacks := func(name, cmd string) {
		t.Helper()
		if shouldMatch(procInfo{name: name, cmd: cmd}, re) {
			t.Errorf("(%q,%q) must not match", name, cmd)
		}
	}
	has("go.exe", "")
	has("compile64.exe", "")
	has("link.exe", "")
	has("gcc.exe", "")
	has("make.exe", "")
	has("mutator.exe", "")
	has("store.test.exe", "")
	has("qm.exe", "serve -addr :8080")
	has("pwsh.exe", "go run ./tools/mutator -packages internal/store")
	has("pwsh.exe", `C:\repo\quotes-manager\mutation-artifacts\slot3\store.test.exe -test.run x`)
	has("pwsh.exe", `cd C:\dev\quotes-manager; make test`)
	lacks("pwsh.exe", "Get-ChildItem")
	lacks("code.exe", "C:\\Users\\me\\code.exe")
	lacks("msedge.exe", "")
	custom := regexp.MustCompile(`^special[.]exe$`)
	if !shouldMatch(procInfo{name: "special.exe"}, custom) {
		t.Fatal("custom regex must match by name")
	}
	if shouldMatch(procInfo{name: "go.exe", cmd: "go build"}, custom) {
		t.Fatal("custom regex must not match go.exe")
	}
}

func TestPickLargest(t *testing.T) {
	if got := pickLargest(nil); got.valid {
		t.Fatalf("empty hits = %+v, want invalid zero", got)
	}
	hits := []procInfo{
		{pid: 1, ws: 100},
		{pid: 2, ws: 500},
		{pid: 3, ws: 500},
		{pid: 4, ws: 200},
	}
	got := pickLargest(hits)
	if got.pid != 2 {
		t.Fatalf("pickLargest = pid %d, want 2 (tie broken by lower pid)", got.pid)
	}
}

func TestProtectedSetCoversAncestors(t *testing.T) {
	self := uint32(30)
	snap := map[uint32]*procInfo{
		10: {pid: 10, ppid: 4},
		20: {pid: 20, ppid: 10},
		30: {pid: 30, ppid: 20},
		99: {pid: 99, ppid: 10},
	}
	got := protectedSetWithSelf(snap, self)
	for _, pid := range []uint32{10, 20, 30} {
		if !got[pid] {
			t.Fatalf("pid %d must be protected, got %v", pid, got)
		}
	}
	if got[99] {
		t.Fatal("unrelated descendant of root need not be protected")
	}
}

func TestKillTreeRemovesDescendants(t *testing.T) {
	if _, err := exec.LookPath("pwsh"); err != nil {
		t.Skip("pwsh not on PATH")
	}
	parent := exec.Command("pwsh", "-NoProfile", "-Command",
		`$c = Start-Process pwsh -ArgumentList '-NoProfile','-Command','Start-Sleep','-Seconds','30' -PassThru; Wait-Process -Id $c.Id`)
	if err := parent.Start(); err != nil {
		t.Fatalf("spawn parent: %v", err)
	}
	defer func() {
		if parent.Process != nil {
			killTree(map[uint32]*procInfo{}, uint32(parent.Process.Pid))
		}
	}()
	var snap map[uint32]*procInfo
	for range 40 {
		var err error
		snap, err = snapshot()
		if err != nil {
			t.Fatalf("snapshot: %v", err)
		}
		if len(childrenOf(snap, uint32(parent.Process.Pid))) > 0 {
			break
		}
		time.Sleep(250 * time.Millisecond)
	}
	kids := childrenOf(snap, uint32(parent.Process.Pid))
	if len(kids) == 0 {
		t.Fatal("grandchild never appeared under parent")
	}
	killed := killTree(snap, uint32(parent.Process.Pid))
	if len(killed) < 2 {
		t.Fatalf("killTree targeted %d pids, want >= 2 (parent + child): %v", len(killed), killed)
	}
	time.Sleep(500 * time.Millisecond)
	after, err := snapshot()
	if err != nil {
		t.Fatalf("snapshot after kill: %v", err)
	}
	for _, pid := range killed {
		if after[pid] != nil {
			t.Fatalf("SURVIVOR: pid %d still present after killTree", pid)
		}
	}
}

func childrenOf(snap map[uint32]*procInfo, pid uint32) []uint32 {
	var out []uint32
	for _, p := range snap {
		if p.ppid == pid {
			out = append(out, p.pid)
		}
	}
	return out
}
