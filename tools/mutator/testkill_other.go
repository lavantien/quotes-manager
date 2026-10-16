//go:build !windows

package main

import (
	"os/exec"
	"syscall"
)

func attachKillTree(cmd *exec.Cmd) (release func() error, err error) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	return func() error {
		if cmd.Process == nil {
			return nil
		}
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}, nil
}
