package triangulate

import (
	"strings"
	"testing"
)

func FuzzSplitEnKey(f *testing.F) {
	f.Add("en/sujato:mn1:0.1")
	f.Add("en/:mn1:0.1")
	f.Add("en/sujato::0.1")
	f.Add("en/sujato:mn1:")
	f.Fuzz(func(t *testing.T, key string) {
		author, muid, ok := splitEnKey(key)
		if !ok {
			return
		}
		if author == "" || muid == "" {
			t.Fatalf("ok with empty parts: author=%q muid=%q (%q)", author, muid, key)
		}
		if !strings.HasPrefix(key, prefixEn+author+":"+muid+":") {
			t.Fatalf("parts not a prefix of key: %q %q %q", key, author, muid)
		}
	})
}

func FuzzParseDateTolerant(f *testing.F) {
	f.Add("2026-01-02")
	f.Add("2026-01-02T15:04:05Z")
	f.Add("2026-01-02 15:04:05")
	f.Add("garbage")
	f.Add("")
	f.Fuzz(func(t *testing.T, s string) {
		got, err := parseDateTolerant(s)
		if err != nil {
			if !got.IsZero() {
				t.Fatalf("error with non-zero time: %v (%q)", got, s)
			}
			return
		}
		if got.IsZero() {
			t.Fatalf("zero time without error (%q)", s)
		}
		if _, err := parseDateTolerant(got.Format("2006-01-02")); err != nil {
			t.Fatalf("date-only reparse failed: %v (%q)", got, s)
		}
	})
}

func FuzzCapJoin(f *testing.F) {
	f.Add("a", "bb", "ccc", 10)
	f.Add("", "", "", 5)
	f.Add("mn1:1.1", "mn1:2.2", "mn1:3.3", 0)
	f.Fuzz(func(t *testing.T, a, b, c string, max int) {
		if max < 0 {
			max = -max
		}
		items := []string{a, b, c}
		got := capJoin(items, max)
		plain := strings.Join(items, ", ")
		if len(plain) <= max {
			if got != plain {
				t.Fatalf("fitting join altered: %q vs %q", got, plain)
			}
			return
		}
		if len(got) > max+32 {
			t.Fatalf("capped join %d exceeds budget %d: %q", len(got), max, got)
		}
	})
}
