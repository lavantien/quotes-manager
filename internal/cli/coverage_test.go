package cli

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lavantien/quotes-manager/internal/config"
)

func writeCoverageInputs(t *testing.T, profile, readme string) (string, string) {
	t.Helper()
	dir := t.TempDir()
	profilePath := filepath.Join(dir, "coverage.out")
	readmePath := filepath.Join(dir, "readme.md")
	if err := os.WriteFile(profilePath, []byte(profile), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(readmePath, []byte(readme), 0o644); err != nil {
		t.Fatal(err)
	}
	return profilePath, readmePath
}

func TestRunCoverageUpdatesBadge(t *testing.T) {
	profilePath, readmePath := writeCoverageInputs(t,
		"mode: set\nfoo.go:1.1,2.2 5 1\nbar.go:3.1,4.2 5 0\n",
		"<!-- coverage:START -->\nold\n<!-- coverage:END -->\n")
	oldOut := stdout
	stdout = new(bytes.Buffer)
	_, err := coverageRun(profilePath, readmePath)
	stdout = oldOut
	if err != nil {
		t.Fatalf("coverageRun: %v", err)
	}
	got, _ := os.ReadFile(readmePath)
	if !strings.Contains(string(got), "50.0%25") {
		t.Errorf("badge not updated:\n%s", got)
	}
}

func TestRunCoverageBadgeAlreadyCurrentIsIdempotent(t *testing.T) {
	profilePath, readmePath := writeCoverageInputs(t,
		"mode: set\nfoo.go:1.1,2.2 5 1\nbar.go:3.1,4.2 5 0\n",
		"<!-- coverage:START -->\nold\n<!-- coverage:END -->\n")
	oldOut := stdout
	stdout = io.Discard
	if _, err := coverageRun(profilePath, readmePath); err != nil {
		stdout = oldOut
		t.Fatalf("coverageRun: %v", err)
	}
	first, _ := os.ReadFile(readmePath)
	if _, err := coverageRun(profilePath, readmePath); err != nil {
		stdout = oldOut
		t.Fatalf("coverageRun: %v", err)
	}
	stdout = oldOut
	second, _ := os.ReadFile(readmePath)
	if string(first) != string(second) {
		t.Errorf("readme changed on a no-op refresh:\nbefore: %s\nafter:  %s", first, second)
	}
}

func TestRunCoverageNoMarkersLeavesReadmeUntouched(t *testing.T) {
	profilePath, readmePath := writeCoverageInputs(t,
		"mode: set\nfoo.go:1.1,2.2 5 1\n",
		"no markers here\n")
	oldOut := stdout
	stdout = io.Discard
	_, err := coverageRun(profilePath, readmePath)
	stdout = oldOut
	if err != nil {
		t.Fatalf("coverageRun: %v", err)
	}
	got, _ := os.ReadFile(readmePath)
	if string(got) != "no markers here\n" {
		t.Errorf("readme changed without markers:\n%s", got)
	}
}

func TestRunCoverageMissingProfileErrors(t *testing.T) {
	if _, err := coverageRun(filepath.Join(t.TempDir(), "nope"), "readme.md"); err == nil {
		t.Error("missing profile should error")
	}
}

func TestRunCoverageBadProfileErrors(t *testing.T) {
	profilePath, readmePath := writeCoverageInputs(t,
		"mode: set\nfoo.go:1.1,2.2 x 1\n",
		"x\n")
	if _, err := coverageRun(profilePath, readmePath); err == nil {
		t.Error("unparseable profile should error")
	}
}

func TestRunCoverageRejectsUnexpectedArg(t *testing.T) {
	if err := runCoverage(context.Background(), config.Defaults(), []string{"extra"}); err == nil {
		t.Error("coverage must reject positional arguments")
	}
}

func TestRunCoverageRejectsBadFlag(t *testing.T) {
	if err := runCoverage(context.Background(), config.Defaults(), []string{"-nope"}); err == nil {
		t.Error("coverage must reject unknown flags")
	}
}
