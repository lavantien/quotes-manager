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

// TestServeListenFails: a valid (temp) database opens and seeds fine, but an
// invalid bind address makes ListenAndServe fail immediately — exercising the
// happy open+seed path and the listen error return without blocking.
func TestServeListenFails(t *testing.T) {
	db := filepath.Join(t.TempDir(), "db.sqlite")
	if err := serve("not a valid:addr", db); err == nil {
		t.Error("serve should fail to listen on a bad address")
	}
}
