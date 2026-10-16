package main

import (
	"fmt"
	"go/scanner"
	"go/token"
	"os"
	"strings"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: stripcomments <file.go>...")
		os.Exit(2)
	}
	for _, path := range os.Args[1:] {
		if err := strip(path); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", path, err)
			os.Exit(1)
		}
	}
}

func strip(path string) error {
	src, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	fset := token.NewFileSet()
	file := fset.AddFile(path, -1, len(src))
	var s scanner.Scanner
	s.Init(file, src, nil, scanner.ScanComments)

	type span struct {
		off, end int
		fullLine bool
	}
	var spans []span
	for {
		pos, tok, lit := s.Scan()
		if tok == token.EOF {
			break
		}
		if tok != token.COMMENT {
			continue
		}
		if isDirective(lit) {
			continue
		}
		origOff := fset.Position(pos).Offset
		off := origOff
		fullLine := off == 0 || src[off-1] == '\n'
		if !fullLine {
			i := off - 1
			for i >= 0 && (src[i] == ' ' || src[i] == '\t') {
				i--
			}
			if i >= 0 && src[i] != '\n' {
				off = i + 1
			}
		}
		end := origOff + len(lit)
		if fullLine && end < len(src) && src[end] == '\r' {
			end++
		}
		if fullLine && end < len(src) && src[end] == '\n' {
			end++
		}
		spans = append(spans, span{off: off, end: end, fullLine: fullLine})
	}
	if len(spans) == 0 {
		return nil
	}

	var b strings.Builder
	prev := 0
	for _, sp := range spans {
		b.Write(src[prev:sp.off])
		prev = sp.end
	}
	b.Write(src[prev:])
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

func isDirective(lit string) bool {
	return strings.HasPrefix(lit, "//go:") || strings.HasPrefix(lit, "//line:")
}
