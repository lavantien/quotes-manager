//go:build windows

package main

import (
	"fmt"
	"os/exec"
	"slices"
	"unsafe"

	"golang.org/x/sys/windows"
)

func attachKillTree(cmd *exec.Cmd) (release func() error, err error) {
	return func() error {
		if cmd.Process == nil {
			return nil
		}
		return killProcessTree(cmd.Process.Pid)
	}, nil
}

func killProcessTree(root int) error {
	procs, err := snapshotParentage()
	if err != nil {
		return fmt.Errorf("snapshot processes: %w", err)
	}
	children := map[uint32][]uint32{}
	for pid, ppid := range procs {
		children[ppid] = append(children[ppid], pid)
	}
	var firstErr error
	for _, pid := range slices.Backward(treeOrder(children, uint32(root))) {
		if err := terminateProcess(pid); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func treeOrder(children map[uint32][]uint32, root uint32) []uint32 {
	var order []uint32
	seen := map[uint32]bool{}
	var walk func(pid uint32)
	walk = func(pid uint32) {
		if seen[pid] {
			return
		}
		seen[pid] = true
		order = append(order, pid)
		for _, c := range children[pid] {
			walk(c)
		}
	}
	walk(root)
	return order
}

func snapshotParentage() (map[uint32]uint32, error) {
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, err
	}
	defer func() { _ = windows.CloseHandle(snap) }()
	var entry windows.ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))
	out := map[uint32]uint32{}
	err = windows.Process32First(snap, &entry)
	for err == nil {
		out[entry.ProcessID] = entry.ParentProcessID
		err = windows.Process32Next(snap, &entry)
	}
	return out, nil
}

func terminateProcess(pid uint32) error {
	h, err := windows.OpenProcess(windows.PROCESS_TERMINATE, false, pid)
	if err != nil {
		return err
	}
	defer func() { _ = windows.CloseHandle(h) }()
	return windows.TerminateProcess(h, 1)
}
