//go:build windows

package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"

	"github.com/lavantien/quotes-manager/internal/canon/winjob"
)

func parseJobMemArgs(args []string) (uint64, []string) {
	if len(args) >= 2 && args[0] == "-jobmem" {
		if mem, err := strconv.ParseUint(args[1], 10, 64); err == nil {
			return mem, args[2:]
		}
		fmt.Fprintf(os.Stderr, "jobwrap: bad -jobmem %q\n", args[1])
		os.Exit(2)
	}
	return 0, args
}

func main() {
	memCap, rest := parseJobMemArgs(os.Args[1:])
	if len(rest) < 1 {
		fmt.Fprintln(os.Stderr, "usage: jobwrap [-jobmem bytes] command [args...]")
		os.Exit(2)
	}
	job, err := winjob.New(0)
	if err != nil {
		fmt.Fprintln(os.Stderr, "jobwrap:", err)
		os.Exit(1)
	}
	if memCap > 0 {
		if err := winjob.SetJobMemoryLimit(job, memCap); err != nil {
			fmt.Fprintln(os.Stderr, "jobwrap:", err)
			os.Exit(1)
		}
	}
	if err := winjob.AssignSelf(job); err != nil {
		fmt.Fprintln(os.Stderr, "jobwrap:", err)
		os.Exit(1)
	}

	cmd := exec.Command(rest[0], rest[1:]...)
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
