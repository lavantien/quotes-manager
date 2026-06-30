// Package coverbadge computes Go test coverage from a cover profile and renders
// a shields.io static badge into a README between marker comments.
package coverbadge

import (
	"bufio"
	"fmt"
	"strconv"
	"strings"
)

// Marker comments delimit the badge in the README; whatever lies between them is
// replaced when the badge is refreshed.
const (
	StartMarker = "<!-- coverage:START -->"
	EndMarker   = "<!-- coverage:END -->"
)

// Pct computes total statement coverage from a Go cover profile (the text written
// by `go test -coverprofile=...`). Each non-mode line is
// `<file>:<start>,<end> <numStmt> <count>`; a block is covered when count > 0.
// Returns an error on an empty/unparseable profile.
func Pct(profile string) (float64, error) {
	var total, covered int
	scanner := bufio.NewScanner(strings.NewReader(profile))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "mode:") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		numStmt, err := strconv.Atoi(fields[len(fields)-2])
		if err != nil {
			return 0, fmt.Errorf("parse numStmt %q: %w", fields[len(fields)-2], err)
		}
		count, err := strconv.Atoi(fields[len(fields)-1])
		if err != nil {
			return 0, fmt.Errorf("parse count %q: %w", fields[len(fields)-1], err)
		}
		total += numStmt
		if count > 0 {
			covered += numStmt
		}
	}
	if err := scanner.Err(); err != nil {
		return 0, err
	}
	if total == 0 {
		return 0, fmt.Errorf("empty coverage profile")
	}
	return float64(covered) / float64(total) * 100, nil
}

// RenderBadge returns readme with the line between the coverage markers replaced
// by a shields.io badge reflecting pct. If the markers are absent the readme is
// returned unchanged.
func RenderBadge(readme string, pct float64) string {
	start := strings.Index(readme, StartMarker)
	end := strings.Index(readme, EndMarker)
	if start < 0 || end < 0 || end < start {
		return readme
	}
	badge := fmt.Sprintf("![coverage](%s)", BadgeURL(pct))
	return readme[:start+len(StartMarker)] + "\n" + badge + "\n" + readme[end:]
}

// BadgeURL builds a shields.io static badge URL for the given percentage.
func BadgeURL(pct float64) string {
	return fmt.Sprintf("https://img.shields.io/badge/coverage-%.1f%%25-%s", pct, Color(pct))
}

// Color picks a shields.io color by coverage tier.
func Color(pct float64) string {
	switch {
	case pct >= 80:
		return "brightgreen"
	case pct >= 60:
		return "yellow"
	default:
		return "red"
	}
}
