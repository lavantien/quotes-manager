//go:build windows

package winjob

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

func TestKillOnClose(t *testing.T) {
	job, err := New(0)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	helper := exec.Command(os.Args[0], "-test.run=TestWinjobTreeHelperProcess")
	helper.Env = append(os.Environ(), "GO_WANT_WINJOB_HELPER=1")
	stdin, err := helper.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := helper.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := helper.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = helper.Process.Kill() })

	if err := AssignPID(job, uint32(helper.Process.Pid)); err != nil {
		t.Fatalf("AssignPID: %v", err)
	}
	if _, err := stdin.Write([]byte("go\n")); err != nil {
		t.Fatal(err)
	}
	line, err := bufio.NewReader(stdout).ReadString('\n')
	if err != nil {
		t.Fatalf("read grandchild pid: %v", err)
	}
	grandPID, err := strconv.Atoi(strings.TrimSpace(line))
	if err != nil {
		t.Fatalf("grandchild pid line %q: %v", line, err)
	}

	if err := windows.CloseHandle(job); err != nil {
		t.Fatalf("close job handle: %v", err)
	}
	deadline := time.Now().Add(15 * time.Second)
	for _, pid := range []uint32{uint32(helper.Process.Pid), uint32(grandPID)} {
		if !waitGone(pid, deadline) {
			t.Fatalf("pid %d survived the job close, want death-with-parent", pid)
		}
	}
}

func TestWinjobTreeHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_WINJOB_HELPER") != "1" {
		t.Skip()
	}
	if _, err := fmt.Fscan(os.Stdin); err != nil {
		os.Exit(2)
	}
	kid := exec.Command(os.Args[0], "-test.run=TestWinjobSleeperProcess")
	kid.Env = append(os.Environ(), "GO_WANT_WINJOB_SLEEPER=1")
	if err := kid.Start(); err != nil {
		os.Exit(3)
	}
	fmt.Println(kid.Process.Pid)
	time.Sleep(10 * time.Minute)
	os.Exit(0)
}

func TestWinjobSleeperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_WINJOB_SLEEPER") != "1" {
		t.Skip()
	}
	time.Sleep(10 * time.Minute)
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
