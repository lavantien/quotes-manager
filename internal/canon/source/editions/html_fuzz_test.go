package editions

import (
	"strings"
	"testing"
)

func FuzzHTMLSegments(f *testing.F) {
	f.Add(`<p id="mn1:1.1">text</p>`)
	f.Add(`<article id="mn1"><p id="mn1:1.1"><a role="doc-noteref">1</a>x</p></article>`)
	f.Add(`<p id=":`)
	f.Add("\xef\xbb\xbf<p id=\"an1.1-2:3.4\">\xff\xfe")
	f.Add(`<div id="mn1:1.1"><span><b>deep</b></span></div><p id="mn2:0.1">t`)
	f.Fuzz(func(t *testing.T, in string) {
		n := 0
		err := ParseHTMLSegments(strings.NewReader(in), func(muid, segkey, text string) error {
			n++
			if muid == "" || segkey == "" || strings.ContainsAny(muid+segkey, "{} \t\n\r") {
				t.Fatalf("malformed record %q:%q from %q", muid, segkey, in)
			}
			return nil
		})
		if err != nil {
			t.Fatalf("parser must not error on arbitrary input: %v", err)
		}
		_ = n
	})
}
