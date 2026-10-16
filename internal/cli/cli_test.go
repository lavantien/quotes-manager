package cli

import (
	"testing"
)

func TestVersionRegistered(t *testing.T) {
	if _, ok := Lookup("version"); !ok {
		t.Fatal("version command must be registered")
	}
}

func TestMainUsageExit(t *testing.T) {
	if code := Main(nil); code != 2 {
		t.Fatalf("no args must be usage exit 2, got %d", code)
	}
}

func TestMainUnknownCommand(t *testing.T) {
	if code := Main([]string{"definitely-not-a-command"}); code != 2 {
		t.Fatalf("unknown command must exit 2, got %d", code)
	}
}

func TestMainVersionRuns(t *testing.T) {
	if code := Main([]string{"version"}); code != 0 {
		t.Fatalf("version must exit 0, got %d", code)
	}
}
