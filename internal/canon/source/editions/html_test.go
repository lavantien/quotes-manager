package editions

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/net/html"
)

func writeHTMLTree(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for rel, body := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestFindHTML(t *testing.T) {
	root := t.TempDir()
	writeHTMLTree(t, root, map[string]string{
		"en/sujato/mn/html/Middle-Discourses-sujato-2026-08-26.html":     "a",
		"en/sujato/an/html/Numbered-Discourses-sujato-2026-08-26.html":   "b",
		"en/brahmali/pli-tv-vi/html/Collection-brahmali-2026-09-26.html": "c",
		"en/sujato/mn/paperback/x.zip":                                   "not html",
		"en/sujato/mn/html/notes.txt":                                    "not html",
		"en/sujato/mn/html/sub/x.html":                                   "too deep, skipped",
		"en/sujato/mn/other/x.html":                                      "wrong dir, skipped",
	})
	got, err := FindHTML(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("html files = %d, want 3: %+v", len(got), got)
	}
	byPath := map[string]HTMLInfo{}
	for _, h := range got {
		byPath[filepath.ToSlash(h.Path)] = h
	}
	mn := byPath[filepath.ToSlash(filepath.Join(root, "en/sujato/mn/html/Middle-Discourses-sujato-2026-08-26.html"))]
	if mn.Author != "sujato" || mn.Collection != "mn" || mn.BuildDate != "2026-08-26" {
		t.Errorf("mn html info = %+v", mn)
	}

	if _, err := FindHTML(filepath.Join(root, "missing")); err == nil {
		t.Error("missing en/ tree must error")
	}
	writeHTMLTree(t, root, map[string]string{"en/sujato/mn/html/undated.html": "x"})
	if _, err := FindHTML(root); err == nil {
		t.Error("undated html name must error")
	}
}

func TestParseHTMLSegments(t *testing.T) {
	doc := `<!doctype html>
<html><head><title>Ignored</title></head>
<body>
<section id="frontmatter"><h1 id="halftitlepage">Half title</h1></section>
<article id="mn1">
<h3 class="sutta-title heading"><span class="sutta-heading acronym">MN 1</span></h3>
<p id="mn1:0.1">Middle Discourses 1</p>
<p id="mn1:1.1"><span class="evam">So I have heard.<a epub:type="noteref" href="#note-1" id="noteref-1" role="doc-noteref">1</a> </span>At one time.<br>New line.</p>
<p id="mn1:2.1">Deep <em>indeed</em>.</p>
</article>
<article id="an11.992-1151">
<p id="an11.992-1151:3.5">Grouped prefix &amp; entity folded later.</p>
</article>
</body></html>`
	type rec struct{ muid, seg, text string }
	var got []rec
	err := ParseHTMLSegments(strings.NewReader(doc), func(muid, segkey, text string) error {
		got = append(got, rec{muid, segkey, text})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []rec{
		{"mn1", "0.1", "Middle Discourses 1"},
		{"mn1", "1.1", "So I have heard. At one time. New line."},
		{"mn1", "2.1", "Deep indeed."},
		{"an11.992-1151", "3.5", "Grouped prefix & entity folded later."},
	}
	if len(got) != len(want) {
		t.Fatalf("records = %+v; want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("record[%d] = %+v; want %+v", i, got[i], want[i])
		}
	}
}

func TestParseHTMLSegmentsToleratesMalformed(t *testing.T) {
	for _, doc := range []string{
		"",
		"<p id=",
		`<p id="mn1:1.1">unterminated`,
		`<p id="mn1:1.1"><a role="doc-noteref">2`,
		`<div id=":1.1">no muid</div>`,
		`<div id="mn1:">empty segkey</div>`,
		`<div id="Noteref-1">upper case no match</div>`,
		`<img id="mn1:1.1" src="x.png">`,
	} {
		var n int
		if err := ParseHTMLSegments(strings.NewReader(doc), func(muid, segkey, text string) error {
			n++
			if muid == "" || segkey == "" {
				t.Errorf("malformed doc %q yielded empty key %q:%q", doc, muid, segkey)
			}
			return nil
		}); err != nil {
			t.Errorf("malformed doc %q must not error: %v", doc, err)
		}
		_ = n
	}
}

func TestStemDateSharedGrammar(t *testing.T) {
	if d, err := StemDate("Middle-Discourses-sujato-2026-08-26"); err != nil || d != "2026-08-26" {
		t.Errorf("html stem date = %q/%v", d, err)
	}
	if d, err := StemDate("Two-dates-2026-01-01-and-2026-02-02"); err != nil || d != "2026-02-02" {
		t.Errorf("last date must win: %q/%v", d, err)
	}
	if _, err := StemDate("12345-01-01-in-digits"); err == nil {
		t.Error("digit-run date must not count")
	}
}

func TestIsNoteref(t *testing.T) {
	tok := func(attrs ...html.Attribute) html.Token {
		return html.Token{Type: html.StartTagToken, Data: "a", Attr: attrs}
	}
	cases := []struct {
		name string
		tok  html.Token
		want bool
	}{
		{"role", tok(html.Attribute{Key: "role", Val: "doc-noteref"}), true},
		{"epub", tok(html.Attribute{Key: "epub:type", Val: "noteref"}), true},
		{"href before role", tok(
			html.Attribute{Key: "href", Val: "#n1"},
			html.Attribute{Key: "role", Val: "doc-noteref"}), true},
		{"plain link", tok(html.Attribute{Key: "href", Val: "#n1"}), false},
		{"other role", tok(html.Attribute{Key: "role", Val: "x"}), false},
		{"other epub", tok(html.Attribute{Key: "epub:type", Val: "x"}), false},
		{"no attrs", tok(), false},
	}
	for _, c := range cases {
		if got := isNoteref(c.tok); got != c.want {
			t.Errorf("%s: isNoteref = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestParseHTMLSegmentsSelfClosing(t *testing.T) {
	doc := `<p id="mn1:1.1"/>after
<p id="mn1:2.1">x <span id="mn1:3.1"/> y</p>
<span id="mn1:4.1"/>
<img id="mn1:5.1" src="t"/>`
	type rec struct{ muid, seg, text string }
	var got []rec
	err := ParseHTMLSegments(strings.NewReader(doc), func(muid, segkey, text string) error {
		got = append(got, rec{muid, segkey, text})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []rec{
		{"mn1", "1.1", ""},
		{"mn1", "2.1", "x y"},
		{"mn1", "4.1", ""},
	}
	if len(got) != len(want) {
		t.Fatalf("records = %+v; want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("record[%d] = %+v; want %+v", i, got[i], want[i])
		}
	}
}
