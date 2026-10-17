package editions

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

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

func TestReadLastRunDate(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, lastRunFile)
	if err := os.WriteFile(p, []byte("2026-09-30 \r\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := ReadLastRunDate(root)
	if err != nil {
		t.Fatal(err)
	}
	if got != "2026-09-30" {
		t.Errorf("ReadLastRunDate = %q, want 2026-09-30", got)
	}
	if err := os.WriteFile(p, []byte(string(rune(0xFEFF))+"2026-09-30\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, err = ReadLastRunDate(root); err != nil || got != "2026-09-30" {
		t.Errorf("ReadLastRunDate with BOM = %q, %v, want 2026-09-30", got, err)
	}
	if _, err := ReadLastRunDate(t.TempDir()); err == nil {
		t.Error("ReadLastRunDate on missing file: expected error")
	}
}

func TestLatestCommit(t *testing.T) {
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git not in PATH")
	}
	dir := t.TempDir()
	emptyCfg := filepath.Join(t.TempDir(), "gitconfig")
	if err := os.WriteFile(emptyCfg, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	env := append(os.Environ(),
		"GIT_CONFIG_GLOBAL="+emptyCfg,
		"GIT_CONFIG_SYSTEM="+emptyCfg,
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com",
		"GIT_AUTHOR_DATE=2026-01-02T03:04:05Z",
		"GIT_COMMITTER_DATE=2026-01-02T03:04:05Z",
	)
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command(git, append([]string{"-C", dir}, args...)...)
		cmd.Env = env
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q")
	tex := filepath.Join(dir, "en", "x.tex")
	if err := os.MkdirAll(filepath.Dir(tex), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tex, []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-q", "-m", "one")

	got, err := LatestCommit(dir, "en")
	if err != nil {
		t.Fatalf("LatestCommit: %v", err)
	}
	ts, err := time.Parse(time.RFC3339, got)
	if err != nil {
		t.Fatalf("LatestCommit = %q, not RFC3339: %v", got, err)
	}
	if want := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC); !ts.Equal(want) {
		t.Errorf("LatestCommit = %q, want %v", got, want)
	}
	if _, err := LatestCommit(dir, "does-not-exist"); err == nil {
		t.Error("LatestCommit with no matching path: expected error")
	}
}

func TestCellSanitize(t *testing.T) {
	if got := cell("a|b\nc\r"); got != `a\|b c ` {
		t.Errorf("cell = %q, want %q", got, `a\|b c `)
	}
	if got := cell("plain"); got != "plain" {
		t.Errorf("cell = %q, want plain", got)
	}
}

func TestDiscoveryReportHTMLOnly(t *testing.T) {
	root := t.TempDir()
	writeHTMLTree(t, root, map[string]string{
		"en/sujato/mn/html/Middle-Discourses-sujato-2026-08-26.html":     "a",
		"en/sujato/an/html/Numbered-Discourses-sujato-2026-08-26.html":   "b",
		"en/brahmali/pli-tv-vi/html/Collection-brahmali-2026-09-26.html": "c",
	})
	htmls, err := FindHTML(root)
	if err != nil {
		t.Fatal(err)
	}
	slices.Reverse(htmls)
	out := filepath.Join(t.TempDir(), "nested", "dir", "r.md")
	if err := DiscoveryReport(htmls, out); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	s := string(body)
	if !strings.HasPrefix(s, "# editions html discovery report\n\nhtml editions: 3\n\n") {
		t.Errorf("report header wrong\nreport:\n%s", s)
	}
	rows := []string{
		"| brahmali | pli-tv-vi | 2026-09-26 | Collection-brahmali-2026-09-26.html |",
		"| sujato | an | 2026-08-26 | Numbered-Discourses-sujato-2026-08-26.html |",
		"| sujato | mn | 2026-08-26 | Middle-Discourses-sujato-2026-08-26.html |",
	}
	last := 0
	for _, row := range rows {
		i := strings.Index(s, row)
		if i < 0 {
			t.Fatalf("report missing row %q\nreport:\n%s", row, s)
		}
		if i < last {
			t.Errorf("row %q out of sorted order in report:\n%s", row, s)
		}
		last = i
	}
	if !strings.Contains(s, "| author | collection | build date | file |") {
		t.Errorf("report missing table header\nreport:\n%s", s)
	}
	if strings.Contains(s, "tex") {
		t.Errorf("html-only report must not mention tex\nreport:\n%s", s)
	}
	if strings.Contains(s, "zips") {
		t.Errorf("html-only report must not render a zip table\nreport:\n%s", s)
	}
}

func TestDiscoveryReportEmptyAndDeterministic(t *testing.T) {
	dir := t.TempDir()
	empty := filepath.Join(dir, "a.md")
	if err := DiscoveryReport(nil, empty); err != nil {
		t.Fatal(err)
	}
	first, err := os.ReadFile(empty)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(first), "html editions: 0") {
		t.Errorf("empty report header wrong\nreport:\n%s", first)
	}
	htmls := []HTMLInfo{
		{Author: "sujato", Collection: "mn", FileName: "b-2026-01-01.html", BuildDate: "2026-01-01"},
		{Author: "sujato", Collection: "mn", FileName: "a-2026-01-01.html", BuildDate: "2026-01-01"},
	}
	b := filepath.Join(dir, "b.md")
	if err := DiscoveryReport(htmls, b); err != nil {
		t.Fatal(err)
	}
	second, err := os.ReadFile(b)
	if err != nil {
		t.Fatal(err)
	}
	c := filepath.Join(dir, "c.md")
	if err := DiscoveryReport(htmls, c); err != nil {
		t.Fatal(err)
	}
	third, err := os.ReadFile(c)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(second, third) {
		t.Error("DiscoveryReport output differs between runs")
	}
	i := strings.Index(string(second), "a-2026-01-01.html")
	j := strings.Index(string(second), "b-2026-01-01.html")
	if i < 0 || j < 0 || i > j {
		t.Errorf("report rows not sorted by file name\nreport:\n%s", second)
	}
}
