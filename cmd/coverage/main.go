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

	profile, err := os.ReadFile(*profilePath)
	if err != nil {
		log.Fatalf("read profile %s: %v", *profilePath, err)
	}
	pct, err := coverbadge.Pct(string(profile))
	if err != nil {
		log.Fatalf("parse profile: %v", err)
	}

	readme, err := os.ReadFile(*readmePath)
	if err != nil {
		log.Fatalf("read readme %s: %v", *readmePath, err)
	}
	updated := coverbadge.RenderBadge(string(readme), pct)
	if updated == string(readme) {
		log.Printf("coverage %.1f%% — README has no coverage markers; nothing written", pct)
		return
	}
	if err := os.WriteFile(*readmePath, []byte(updated), 0o644); err != nil {
		log.Fatalf("write readme: %v", err)
	}
	fmt.Printf("coverage %.1f%% — README badge updated\n", pct)
}
