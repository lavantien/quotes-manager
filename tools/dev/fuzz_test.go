package main

import (
	"strings"
	"testing"
)

func FuzzGuardPkg(f *testing.F) {
	f.Add("./internal/config")
	f.Add("")
	f.Add("..\\..\\windows")
	f.Add("C:\\abs")
	f.Add("a/../b")
	f.Add("0..")
	f.Fuzz(func(t *testing.T, s string) {
		if len(s) > 200 {
			return
		}
		err := guardPkg(s)
		if err == nil && pkgIsUnsafe(s) {
			t.Fatalf("guardPkg(%q) accepted an unsafe package path", s)
		}
	})
}

func pkgIsUnsafe(s string) bool {
	if s == "" || strings.HasPrefix(s, "/") || strings.HasPrefix(s, `\`) || filepathIsAbsPortable(s) {
		return true
	}
	for _, part := range strings.FieldsFunc(s, func(r rune) bool { return r == '/' || r == '\\' }) {
		if part == ".." {
			return true
		}
	}
	return false
}

func filepathIsAbsPortable(s string) bool {
	if len(s) < 2 {
		return false
	}
	return s[1] == ':' && ((s[0] >= 'a' && s[0] <= 'z') || (s[0] >= 'A' && s[0] <= 'Z'))
}

func FuzzSplitComma(f *testing.F) {
	f.Add("a,b")
	f.Add(",")
	f.Add("")
	f.Fuzz(func(t *testing.T, s string) {
		got := splitComma(s)
		joined := strings.Join(got, ",")
		if joined != "" {
			for _, part := range got {
				if part == "" || strings.Contains(part, ",") {
					t.Fatalf("splitComma(%q) = %v", s, got)
				}
			}
		}
	})
}

func FuzzCompareVersions(f *testing.F) {
	f.Add("1.2.3", "1.2.4")
	f.Add("", "")
	f.Add("x", "1")
	f.Fuzz(func(t *testing.T, a, b string) {
		c := compareVersions(a, b)
		if c != 0 && c != 1 && c != -1 {
			t.Fatalf("compareVersions returned %d", c)
		}
		if c > 0 && compareVersions(b, a) >= 0 {
			t.Fatalf("compareVersions not antisymmetric for %q %q", a, b)
		}
	})
}
