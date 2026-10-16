//go:build windows

package main

import (
	"fmt"

	"github.com/lavantien/quotes-manager/internal/canon/winjob"
)

const memCapBytes = 4 << 30

func capSelfMemory() error {
	job, err := winjob.New(memCapBytes)
	if err != nil {
		return err
	}
	if err := winjob.AssignSelf(job); err != nil {
		return fmt.Errorf("cap self memory: %w", err)
	}
	return nil
}
