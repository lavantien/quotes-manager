package main

import (
	"path/filepath"
	"testing"
)

// TestServeOpenFails: a database path whose parent directory does not exist
// cannot be opened, so serve returns before binding the listener.
func TestServeOpenFails(t *testing.T) {
	bad := filepath.Join(t.TempDir(), "no", "such", "dir", "db.sqlite")
	if err := serve(":0", bad); err == nil {
		t.Error("serve should fail when the DB path cannot be opened")
	}
}
