//go:build windows

package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/lavantien/quotes-manager/internal/canon/winjob"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: jobwrap command [args...]")
		os.Exit(2)
	}
	job, err := winjob.New(0)
	if err != nil {
		fmt.Fprintln(os.Stderr, "jobwrap:", err)
		os.Exit(1)
	}
	if err := winjob.AssignSelf(job); err != nil {
		fmt.Fprintln(os.Stderr, "jobwrap:", err)
		os.Exit(1)
	}

	cmd := exec.Command(os.Args[1], os.Args[2:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
			os.Exit(exitErr.ExitCode())
		}
		fmt.Fprintln(os.Stderr, "jobwrap:", err)
		os.Exit(1)
	}
}
