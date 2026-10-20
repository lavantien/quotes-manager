package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

func main() {
	data, err := os.ReadFile("dumps/ariyasavaka-curated-list.md")
	if err != nil {
		panic(err)
	}
	lines := strings.Split(string(data), "\n")
	sectionRe := regexp.MustCompile(`^\*\*([IVXL]+)\.\s+(.*?)\.\*\*\s*$`)
	linkRe := regexp.MustCompile(`\[([^\]]*)\]\(https://suttacentral\.net/([^)]*)\)`)
	sc, lc := 0, 0
	for i, ln := range lines {
		if sectionRe.MatchString(ln) {
			sc++
			if sc <= 2 {
				fmt.Printf("sec line %d: %q\n", i+1, ln[:min(40, len(ln))])
			}
		}
		if linkRe.MatchString(ln) {
			lc++
		}
	}
	fmt.Printf("sections=%d linklines=%d lines=%d\n", sc, lc, len(lines))
	fmt.Printf("line13=%q\n", lines[12][:min(60, len(lines[12]))])
	fmt.Printf("line15=%q\n", lines[14][:min(60, len(lines[14]))])
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
