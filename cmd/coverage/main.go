// Command coverage recomputes the Go test coverage percentage from a cover
// profile and refreshes the README's coverage badge (between the coverage
// markers). Run via `make coverage`, which first generates coverage.out with
// `go test -coverpkg=./... -coverprofile`.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/lavantien/quotes-manager/internal/coverbadge"
)

func main() {
	profilePath := flag.String("profile", "coverage.out", "Go cover profile path")
	readmePath := flag.String("readme", "readme.md", "README path to update")
	flag.Parse()

	if _, err := run(*profilePath, *readmePath); err != nil {
		log.Fatalf("coverage: %v", err)
	}
}

// run reads the cover profile, computes the percentage, and refreshes the README
// badge. It returns the computed percentage. The README is left untouched when it
// has no coverage markers.
func run(profilePath, readmePath string) (float64, error) {
	profile, err := os.ReadFile(profilePath)
	if err != nil {
		return 0, err
	}
	pct, err := coverbadge.Pct(string(profile))
	if err != nil {
		return 0, err
	}
	readme, err := os.ReadFile(readmePath)
	if err != nil {
		return 0, err
	}
	updated := coverbadge.RenderBadge(string(readme), pct)
	if updated == string(readme) {
		log.Printf("coverage %.1f%% — README has no coverage markers; nothing written", pct)
		return pct, nil
	}
	if err := os.WriteFile(readmePath, []byte(updated), 0o644); err != nil {
		return pct, err
	}
	fmt.Printf("coverage %.1f%% — README badge updated\n", pct)
	return pct, nil
}
