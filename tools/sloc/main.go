package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const maxSLOC = 1000

var skipDirs = map[string]bool{
	".git": true, "data": true, "dist": true, "playground": true,
	"docs": true, "testdata": true,
}

func main() {
	root := "."
	if len(os.Args) > 1 {
		root = os.Args[1]
	}
	failed := false
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		n, err := count(path)
		if err != nil {
			return err
		}
		if n > maxSLOC {
			failed = true
			fmt.Printf("SLOC FAIL %s: %d > %d\n", path, n, maxSLOC)
		}
		return nil
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if failed {
		os.Exit(1)
	}
	fmt.Println("sloc ok")
}

func count(path string) (int, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	lines := strings.Split(string(b), "\n")
	n := 0
	inBlock := false
	for _, ln := range lines {
		t := strings.TrimSpace(ln)
		switch {
		case t == "":
		case inBlock:
			if strings.Contains(t, "*/") {
				inBlock = false
			}
		case strings.HasPrefix(t, "//"):
		case strings.HasPrefix(t, "/*"):
			if !strings.Contains(t, "*/") {
				inBlock = true
			}
		default:
			n++
		}
	}
	return n, nil
}
