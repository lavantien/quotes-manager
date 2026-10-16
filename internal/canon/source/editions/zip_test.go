package editions

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"hash/crc32"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

type zipEntry struct {
	name string
	body string
}

func buildZip(t *testing.T, entries []zipEntry) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, e := range entries {
		h := &zip.FileHeader{Name: e.name, Method: zip.Deflate}
		if strings.HasSuffix(e.name, "/") {
			h.SetMode(os.ModeDir | 0o755)
		} else {
			h.SetMode(0o644)
		}
		w, err := zw.CreateHeader(h)
		if err != nil {
			t.Fatalf("create %s: %v", e.name, err)
		}
		if _, err := w.Write([]byte(e.body)); err != nil {
			t.Fatalf("write %s: %v", e.name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	return buf.Bytes()
}

func hexSum(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func makeRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	put := func(rel string, data []byte) {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mnZip := buildZip(t, []zipEntry{
		{name: "suttas/", body: ""},
		{name: "suttas/mn1.tex", body: "\\documentclass{book}\n"},
		{name: "suttas/sub/mn2.tex", body: "two\n"},
		{name: "suttas/cover.pdf", body: "%PDF"},
	})
	put("en/sujato/mn/paperback/Middle-Discourses-sujato-2026-08-26-tex.zip", mnZip)
	hcZip := buildZip(t, []zipEntry{
		{name: "suttas/mn1.tex", body: "same inner path, hardcover\n"},
	})
	put("en/sujato/mn/hardcover/Middle-Discourses-sujato-2026-08-26-hardcover-tex.zip", hcZip)
	snZip := buildZip(t, []zipEntry{{name: "suttas/sn1.tex", body: "sn\n"}})
	put("en/sujato/sn/hardcover/Linked-Discourses-sujato-2026-09-01-hardcover-tex.zip", snZip)
	viZip := buildZip(t, []zipEntry{{name: "vin/pli-tv-vi.tex", body: "vi\n"}})
	put("en/brahmali/pli-tv-vi/paperback/Vinaya-Brahmali-2026-07-04-tex.zip", viZip)
	put("en/sujato/mn/epub/Middle-Discourses-sujato-2026-08-26.epub", []byte("epub"))
	put("en/sujato/mn/html/index.html", []byte("<html>"))
	put("en/sujato/mn/paperback/Middle-Discourses-sujato-2026-08-26-cover.zip", []byte("cover"))
	put("en/sujato/mn/paperback/Middle-Discourses-sujato-2026-08-26.zip", mnZip)
	put("en/sujato/notes.txt", []byte("x"))
	return root
}

func TestParseBuildDate(t *testing.T) {
	cases := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"Middle-Discourses-sujato-2026-08-26-tex.zip", "2026-08-26", false},
		{"Vinaya-Brahmali-2026-08-26-hardcover-tex.zip", "2026-08-26", false},
		{"Name-2025-01-02-and-2025-03-04-tex.zip", "2025-03-04", false},
		{"no-date-tex.zip", "", true},
		{"x-2026-13-01-tex.zip", "", true},
		{"x-2026-02-30-tex.zip", "", true},
		{"X-12345-01-01-tex.zip", "", true},
		{"X-2026-01-019-tex.zip", "", true},
		{"plain.zip", "", true},
		{"", "", true},
	}
	for _, tc := range cases {
		got, err := ParseBuildDate(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("ParseBuildDate(%q): expected error, got %q", tc.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseBuildDate(%q): %v", tc.in, err)
		}
		if got != tc.want {
			t.Errorf("ParseBuildDate(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestFindZips(t *testing.T) {
	root := makeRepo(t)
	got, err := FindZips(root)
	if err != nil {
		t.Fatalf("FindZips: %v", err)
	}
	if len(got) != 4 {
		t.Fatalf("FindZips: got %d zips, want 4: %+v", len(got), got)
	}
	byRel := map[string]ZipInfo{}
	for _, z := range got {
		rel, err := filepath.Rel(root, z.Path)
		if err != nil {
			t.Fatal(err)
		}
		byRel[filepath.ToSlash(rel)] = z
	}
	mn := byRel["en/sujato/mn/paperback/Middle-Discourses-sujato-2026-08-26-tex.zip"]
	if mn == (ZipInfo{}) {
		t.Fatalf("FindZips: mn paperback zip missing: %+v", got)
	}
	mnOnDisk, err := os.ReadFile(filepath.Join(root, "en", "sujato", "mn", "paperback", "Middle-Discourses-sujato-2026-08-26-tex.zip"))
	if err != nil {
		t.Fatal(err)
	}
	want := ZipInfo{
		Author:     "sujato",
		Collection: "mn",
		Variant:    "paperback",
		ZipName:    "Middle-Discourses-sujato-2026-08-26-tex.zip",
		BuildDate:  "2026-08-26",
		SHA256:     hexSum(mnOnDisk),
		Path:       filepath.FromSlash(filepath.Join(root, "en/sujato/mn/paperback/Middle-Discourses-sujato-2026-08-26-tex.zip")),
	}
	if mn != want {
		t.Errorf("mn zip = %+v, want %+v", mn, want)
	}
	hc := byRel["en/sujato/mn/hardcover/Middle-Discourses-sujato-2026-08-26-hardcover-tex.zip"]
	if hc.Variant != "hardcover" || hc.BuildDate != "2026-08-26" {
		t.Errorf("hardcover zip = %+v", hc)
	}
	for _, z := range got {
		if z.TexCount != 0 {
			t.Errorf("FindZips must not set TexCount, got %d for %s", z.TexCount, z.ZipName)
		}
	}
}

func TestFindZipsDeterministicAndSorted(t *testing.T) {
	root := makeRepo(t)
	a, err := FindZips(root)
	if err != nil {
		t.Fatal(err)
	}
	b, err := FindZips(root)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("FindZips not deterministic:\n%+v\n%+v", a, b)
	}
	for i := 1; i < len(a); i++ {
		if a[i-1].Path >= a[i].Path {
			t.Errorf("FindZips not sorted by path: %q >= %q", a[i-1].Path, a[i].Path)
		}
	}
}

func TestFindZipsMissingRepo(t *testing.T) {
	if _, err := FindZips(filepath.Join(t.TempDir(), "nope")); err == nil {
		t.Fatal("FindZips on missing repo: expected error")
	}
}

func TestExtract(t *testing.T) {
	root := makeRepo(t)
	out := t.TempDir()
	got, err := Extract(root, out)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(got) != 4 {
		t.Fatalf("Extract: got %d zips, want 4", len(got))
	}
	join := func(segs ...string) string { return filepath.Join(append([]string{out}, segs...)...) }
	mn1 := join("sujato", "mn", "paperback", "Middle-Discourses-sujato-2026-08-26-tex", "suttas", "mn1.tex")
	body, err := os.ReadFile(mn1)
	if err != nil {
		t.Fatalf("mn1.tex not extracted: %v", err)
	}
	if string(body) != "\\documentclass{book}\n" {
		t.Errorf("mn1.tex body = %q", body)
	}
	if _, err := os.Stat(join("sujato", "mn", "paperback", "Middle-Discourses-sujato-2026-08-26-tex", "suttas", "sub", "mn2.tex")); err != nil {
		t.Errorf("nested mn2.tex not extracted: %v", err)
	}
	err = filepath.Walk(out, func(p string, d os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && !strings.HasSuffix(p, ".tex") {
			t.Errorf("non-tex file extracted: %s", p)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	hcMn1 := join("sujato", "mn", "hardcover", "Middle-Discourses-sujato-2026-08-26-hardcover-tex", "suttas", "mn1.tex")
	hcBody, err := os.ReadFile(hcMn1)
	if err != nil {
		t.Fatalf("hardcover mn1.tex not extracted: %v", err)
	}
	if string(hcBody) == string(body) {
		t.Error("hardcover mn1.tex should come from its own zip, bodies differ in fixture")
	}
	counts := map[string]int{}
	for _, z := range got {
		counts[z.ZipName] = z.TexCount
	}
	if counts["Middle-Discourses-sujato-2026-08-26-tex.zip"] != 2 {
		t.Errorf("paperback mn TexCount = %d, want 2", counts["Middle-Discourses-sujato-2026-08-26-tex.zip"])
	}
	if counts["Linked-Discourses-sujato-2026-09-01-hardcover-tex.zip"] != 1 {
		t.Errorf("sn TexCount = %d, want 1", counts["Linked-Discourses-sujato-2026-09-01-hardcover-tex.zip"])
	}
}

func TestExtractZipSlip(t *testing.T) {
	cases := []string{
		"../evil.tex",
		"/abs.tex",
		"C:/evil.tex",
		"C:\\evil.tex",
		"dir\\evil.tex",
		"..\\evil.tex",
		"a/../b.tex",
		"a//b.tex",
		"./b.tex",
		"host.tex:evil.tex",
		"ab:x.tex",
		"sub/ab:x.tex",
	}
	for _, evil := range cases {
		root := t.TempDir()
		p := filepath.Join(root, "en", "a", "c", "paperback", "X-a-2026-01-01-tex.zip")
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, buildZip(t, []zipEntry{
			{name: "ok.tex", body: "ok\n"},
			{name: evil, body: "evil\n"},
		}), 0o644); err != nil {
			t.Fatal(err)
		}
		out := t.TempDir()
		if _, err := Extract(root, out); err == nil {
			t.Errorf("entry %q: expected zip-slip rejection, got success", evil)
			continue
		}
		if _, err := os.Stat(filepath.Join(out, "ok.tex")); err == nil {
			t.Errorf("entry %q: ok.tex must not land at output root (inner path must be preserved)", evil)
		}
		if _, err := os.Stat(filepath.Join(out, "evil.tex")); err == nil {
			t.Errorf("entry %q: evil.tex escaped", evil)
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

func TestExtractEmptyOutRoot(t *testing.T) {
	root := makeRepo(t)
	if _, err := Extract(root, ""); err == nil {
		t.Fatal("Extract with empty outRoot: expected error, got extraction into the working directory")
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

func TestWithinDir(t *testing.T) {
	root := t.TempDir()
	cases := []struct {
		p    string
		want bool
	}{
		{root, true},
		{filepath.Join(root, "a"), true},
		{filepath.Join(root, "a", "b.tex"), true},
		{filepath.Join(root, ".."), false},
		{filepath.Join(root, "..", "x.tex"), false},
	}
	for _, tc := range cases {
		if got := withinDir(root, tc.p); got != tc.want {
			t.Errorf("withinDir(root, %q) = %v, want %v", tc.p, got, tc.want)
		}
	}
}

func TestExtractSkipsDirModeTexEntry(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, "en", "a", "c", "paperback", "X-a-2026-01-01-tex.zip")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, e := range []zipEntry{{name: "folder.tex", body: ""}, {name: "real.tex", body: "x\n"}} {
		h := &zip.FileHeader{Name: e.name, Method: zip.Deflate}
		if e.name == "folder.tex" {
			h.SetMode(os.ModeDir | 0o755)
		} else {
			h.SetMode(0o644)
		}
		w, err := zw.CreateHeader(h)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(e.body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	out := t.TempDir()
	infos, err := Extract(root, out)
	if err != nil {
		t.Fatal(err)
	}
	if infos[0].TexCount != 1 {
		t.Errorf("TexCount = %d, want 1 (dir-mode .tex entry must be skipped)", infos[0].TexCount)
	}
	if _, err := os.Stat(filepath.Join(out, "a", "c", "paperback", "X-a-2026-01-01-tex", "folder.tex")); err == nil {
		t.Error("dir-mode entry must not be written as a file")
	}
}

func TestExtractCorruptCRC(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, "en", "a", "c", "paperback", "X-a-2026-01-01-tex.zip")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	valid := buildZip(t, []zipEntry{{name: "only.tex", body: "payload\n"}})
	sum := crc32.ChecksumIEEE([]byte("payload\n"))
	bad := make([]byte, 4)
	binary.LittleEndian.PutUint32(bad, sum^0xFFFF)
	good := make([]byte, 4)
	binary.LittleEndian.PutUint32(good, sum)
	if bytes.Count(valid, good) != 2 {
		t.Fatalf("expected CRC to appear in local header and central directory, found %d", bytes.Count(valid, good))
	}
	corrupt := bytes.ReplaceAll(valid, good, bad)
	if err := os.WriteFile(p, corrupt, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Extract(root, t.TempDir()); err == nil {
		t.Fatal("Extract on a CRC-corrupt zip: expected error, got silent success")
	}
}

func TestDiscoveryReportTreeAndFences(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, "en", "a", "c", "paperback", "X-a-2026-01-01-tex.zip")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, buildZip(t, []zipEntry{{name: "one.tex", body: "line1\n```\nline3\n"}}), 0o644); err != nil {
		t.Fatal(err)
	}
	texRoot := t.TempDir()
	for _, d := range []string{"a.tex", "b.tex", "c.tex"} {
		if err := os.MkdirAll(filepath.Join(texRoot, "a", "c", "paperback", "X-a-2026-01-01-tex", d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	zips, err := Extract(root, texRoot)
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "r.md")
	if err := DiscoveryReport(zips, nil, texRoot, out); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	s := string(body)
	for _, want := range []string{
		"- X-a-2026-01-01-tex/",
		"### a/c/paperback/X-a-2026-01-01-tex/one.tex",
		"\n````tex\n",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("report missing %q\nreport:\n%s", want, s)
		}
	}
	if strings.Contains(s, "(none)") {
		t.Error("report must not claim an empty tree for a populated texRoot")
	}

	out2 := filepath.Join(t.TempDir(), "r2.md")
	if err := DiscoveryReport(zips, nil, filepath.Join(t.TempDir(), "missing"), out2); err != nil {
		t.Fatal(err)
	}
	body2, err := os.ReadFile(out2)
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(string(body2), "(none)"); n != 2 {
		t.Errorf("missing texRoot: got %d (none) markers, want 2\n%s", n, body2)
	}
}

func TestDiscoveryReportDeterministic(t *testing.T) {
	root := makeRepo(t)
	texRoot := filepath.Join(t.TempDir(), "tex")
	zips, err := Extract(root, texRoot)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	a, b := filepath.Join(dir, "a.md"), filepath.Join(dir, "b.md")
	if err := DiscoveryReport(zips, nil, texRoot, a); err != nil {
		t.Fatal(err)
	}
	if err := DiscoveryReport(zips, nil, texRoot, b); err != nil {
		t.Fatal(err)
	}
	first, err := os.ReadFile(a)
	if err != nil {
		t.Fatal(err)
	}
	second, err := os.ReadFile(b)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("DiscoveryReport output differs between runs")
	}
	s := string(first)
	for _, want := range []string{
		"| sujato | mn | paperback | 2026-08-26 | 2 |",
		"Middle-Discourses-sujato-2026-08-26-tex/",
		"mn1.tex",
		"\\documentclass{book}",
		"## sample tex files",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("report missing %q\nreport:\n%s", want, s)
		}
	}
}

func TestDiscoveryReportSampleLineCap(t *testing.T) {
	root := t.TempDir()
	var lines strings.Builder
	for i := 1; i <= 100; i++ {
		fmt.Fprintf(&lines, "L%02d\n", i)
	}
	p := filepath.Join(root, "en", "a", "c", "paperback", "X-a-2026-01-01-tex.zip")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, buildZip(t, []zipEntry{{name: "one.tex", body: lines.String()}}), 0o644); err != nil {
		t.Fatal(err)
	}
	texRoot := t.TempDir()
	zips, err := Extract(root, texRoot)
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "r.md")
	if err := DiscoveryReport(zips, nil, texRoot, out); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	s := string(body)
	if !strings.Contains(s, "L40") {
		t.Error("report missing line 40 of sample")
	}
	if strings.Contains(s, "L41") {
		t.Error("report must cap samples at 40 lines")
	}
	if !strings.Contains(s, "\n```tex\n") {
		t.Error("report must open the sample with a minimal 3-backtick fence")
	}
}
