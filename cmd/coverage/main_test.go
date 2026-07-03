package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseFlags(t *testing.T) {
	prof, readme := parseFlags([]string{"-profile", "x.out", "-readme", "y.md"})
	if prof != "x.out" || readme != "y.md" {
		t.Errorf("parseFlags = %q, %q; want x.out, y.md", prof, readme)
	}
	prof, readme = parseFlags(nil)
	if prof != "coverage.out" || readme != "readme.md" {
		t.Errorf("defaults = %q, %q; want coverage.out, readme.md", prof, readme)
	}
}

func TestRunUpdatesBadge(t *testing.T) {
	dir := t.TempDir()
	profile := filepath.Join(dir, "coverage.out")
	readme := filepath.Join(dir, "readme.md")
	if err := os.WriteFile(profile, []byte("mode: set\nfoo.go:1.1,2.2 5 1\nbar.go:3.1,4.2 5 0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(readme, []byte("<!-- coverage:START -->\nold\n<!-- coverage:END -->\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	pct, err := run(profile, readme)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if pct != 50.0 {
		t.Errorf("pct = %.1f, want 50", pct)
	}
	got, _ := os.ReadFile(readme)
	if !strings.Contains(string(got), "50.0%25") {
		t.Errorf("badge not updated:\n%s", got)
	}
}

func TestRunBadgeAlreadyCurrentIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	profile := filepath.Join(dir, "coverage.out")
	readme := filepath.Join(dir, "readme.md")
	if err := os.WriteFile(profile, []byte("mode: set\nfoo.go:1.1,2.2 5 1\nbar.go:3.1,4.2 5 0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(readme, []byte("<!-- coverage:START -->\nold\n<!-- coverage:END -->\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// First run writes the badge at 50%.
	if _, err := run(profile, readme); err != nil {
		t.Fatalf("run: %v", err)
	}
	first, _ := os.ReadFile(readme)
	// Second run: the badge is already current, so the README must not change.
	if _, err := run(profile, readme); err != nil {
		t.Fatalf("run: %v", err)
	}
	second, _ := os.ReadFile(readme)
	if string(first) != string(second) {
		t.Errorf("readme changed on a no-op refresh:\nbefore: %s\nafter:  %s", first, second)
	}
}

func TestRunNoMarkersLeavesReadmeUntouched(t *testing.T) {
	dir := t.TempDir()
	profile := filepath.Join(dir, "coverage.out")
	readme := filepath.Join(dir, "readme.md")
	if err := os.WriteFile(profile, []byte("mode: set\nfoo.go:1.1,2.2 5 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(readme, []byte("no markers here\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := run(profile, readme); err != nil {
		t.Fatalf("run: %v", err)
	}
	got, _ := os.ReadFile(readme)
	if string(got) != "no markers here\n" {
		t.Errorf("readme changed without markers:\n%s", got)
	}
}

func TestRunMissingProfileErrors(t *testing.T) {
	if _, err := run(filepath.Join(t.TempDir(), "nope"), "readme.md"); err == nil {
		t.Error("missing profile should error")
	}
}

func TestRunBadProfileErrors(t *testing.T) {
	dir := t.TempDir()
	profile := filepath.Join(dir, "coverage.out")
	readme := filepath.Join(dir, "readme.md")
	if err := os.WriteFile(profile, []byte("mode: set\nfoo.go:1.1,2.2 x 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(readme, []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := run(profile, readme); err == nil {
		t.Error("unparseable profile should error")
	}
}
