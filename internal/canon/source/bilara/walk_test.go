package bilara

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lavantien/quotes-manager/internal/canon/model"
)

func TestMuidFromFilename(t *testing.T) {
	ok := []struct {
		name string
		want string
	}{
		{"mn1_root-pli-ms.json", "mn1"},
		{"pli-tv-bu-pm_root-pli-ms.json", "pli-tv-bu-pm"},
		{"pli-tv-bu-vb-1_root-pli-ms.json", "pli-tv-bu-vb-1"},
		{"mn1_translation-en-sujato.json", "mn1"},
		{"mn1_translation-en-brahmali.json", "mn1"},
		{"pli-tv-bu-vb-1_translation-en-sujato.json", "pli-tv-bu-vb-1"},
		{"pli-tv-pvr1.10_translation-en-brahmali.json", "pli-tv-pvr1.10"},
		{"mn1_html.json", "mn1"},
		{"pli-tv-bu-pm_html.json", "pli-tv-bu-pm"},
		{"mn1_reference.json", "mn1"},
	}
	for _, tt := range ok {
		got, err := MuidFromFilename(tt.name)
		if err != nil || got != tt.want {
			t.Errorf("MuidFromFilename(%q) = %q, %v; want %q, nil", tt.name, got, err, tt.want)
		}
	}

	bad := []string{
		"mn1.json",
		"mn1_root-pli-bb.json",
		"mn1_translation-fr-sujato.json",
		"_html.json",
		"_root-pli-ms.json",
		"mn1_root-pli-ms.txt",
		"mn_name-en-sujato.json",
	}
	for _, name := range bad {
		if got, err := MuidFromFilename(name); err == nil {
			t.Errorf("MuidFromFilename(%q) = %q, nil; want error", name, got)
		}
	}
}

func TestValidFullKey(t *testing.T) {
	ok := []string{
		"mn1:2.1",
		"an2.49:1.6",
		"pli-tv-bu-vb-1:3.2",
		"pli-tv-bu-pm:1.1",
		"mn1:2.1.5",
		"mn1:01.1",
	}
	for _, k := range ok {
		if !ValidFullKey(k) {
			t.Errorf("ValidFullKey(%q) = false; want true", k)
		}
	}
	bad := []string{
		"",
		"mn1:",
		"mn1:1",
		":1.1",
		"MN1:1.1",
		"mn1:1.x",
		"mn1:2.1:",
		"-mn1:1.1",
		"mn1 :1.1",
		"mn1:1.",
		"mn1:2..1",
		"mn1:2.1 ",
	}
	for _, k := range bad {
		if ValidFullKey(k) {
			t.Errorf("ValidFullKey(%q) = true; want false", k)
		}
	}
}

func writeTree(t *testing.T, files []string) string {
	t.Helper()
	root := t.TempDir()
	for _, f := range files {
		p := filepath.Join(root, filepath.FromSlash(f))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestWalkSynthetic(t *testing.T) {
	root := writeTree(t, []string{
		"root/pli/ms/sutta/mn/mn1_root-pli-ms.json",
		"root/pli/ms/vinaya/pli-tv-bu-pm_root-pli-ms.json",
		"root/pli/ms/vinaya/pli-tv-bu-vb/pli-tv-bu-vb-1_root-pli-ms.json",
		"translation/en/sujato/sutta/mn/mn1_translation-en-sujato.json",
		"translation/en/brahmali/sutta/mn/mn2_translation-en-brahmali.json",
		"html/pli/ms/sutta/mn/mn1_html.json",
		"reference/pli/ms/sutta/mn/mn1_reference.json",
		"_publication/en/sujato/mn1_publication.json",
		"README.md",
		"root/pli/ms/sutta/mn/notes.txt",
	})
	var got []File
	if err := Walk(root, func(f File) error {
		got = append(got, f)
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	want := []struct {
		stream model.Stream
		muid   string
		rel    string
	}{
		{model.StreamHTML, "mn1", "html/pli/ms/sutta/mn/mn1_html.json"},
		{model.StreamReference, "mn1", "reference/pli/ms/sutta/mn/mn1_reference.json"},
		{model.StreamRoot, "mn1", "root/pli/ms/sutta/mn/mn1_root-pli-ms.json"},
		{model.StreamRoot, "pli-tv-bu-pm", "root/pli/ms/vinaya/pli-tv-bu-pm_root-pli-ms.json"},
		{model.StreamRoot, "pli-tv-bu-vb-1", "root/pli/ms/vinaya/pli-tv-bu-vb/pli-tv-bu-vb-1_root-pli-ms.json"},
		{model.StreamTranslation, "mn2", "translation/en/brahmali/sutta/mn/mn2_translation-en-brahmali.json"},
		{model.StreamTranslation, "mn1", "translation/en/sujato/sutta/mn/mn1_translation-en-sujato.json"},
	}
	if len(got) != len(want) {
		t.Fatalf("Walk found %d files (%v); want %d", len(got), got, len(want))
	}
	for i, w := range want {
		f := got[i]
		if f.Stream != w.stream || f.Muid != w.muid {
			t.Errorf("Walk[%d] = %s %s; want %s %s", i, f.Stream, f.Muid, w.stream, w.muid)
		}
		if want := filepath.Join(root, filepath.FromSlash(w.rel)); f.Path != want {
			t.Errorf("Walk[%d].Path = %q; want %q", i, f.Path, want)
		}
	}
}

func TestWalkSuffixMismatch(t *testing.T) {
	root := writeTree(t, []string{
		"root/pli/ms/sutta/mn/mn1_root-pli-bb.json",
	})
	err := Walk(root, func(File) error { return nil })
	if err == nil {
		t.Fatal("Walk must fail on a root-stream file with the wrong edition suffix")
	}
	if want := "mn1_root-pli-bb.json"; !strings.Contains(err.Error(), want) {
		t.Errorf("error %q must name the offending file %q", err, want)
	}
}

func TestWalkEmptyMuid(t *testing.T) {
	root := writeTree(t, []string{
		"html/pli/ms/sutta/mn/_html.json",
	})
	err := Walk(root, func(File) error { return nil })
	if err == nil {
		t.Fatal("a stream file with an empty muid must be a hard error")
	}
	if !strings.Contains(err.Error(), "_html.json") {
		t.Errorf("error %q must name the offending file", err)
	}
}

func TestWalkYieldStopsWalk(t *testing.T) {
	root := writeTree(t, []string{
		"html/pli/ms/sutta/mn/mn1_html.json",
		"root/pli/ms/sutta/mn/mn1_root-pli-ms.json",
	})
	sentinel := os.ErrNotExist
	n := 0
	err := Walk(root, func(File) error { n++; return sentinel })
	if err != sentinel {
		t.Fatalf("Walk must propagate the yield error, got %v", err)
	}
	if n != 1 {
		t.Errorf("Walk visited %d files after yield error; want 1", n)
	}
}

func TestWalkTestdataQuartet(t *testing.T) {
	root := filepath.Join("..", "..", "testdata", "bilara")
	var got []File
	if err := Walk(root, func(f File) error {
		got = append(got, f)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	want := []struct {
		stream model.Stream
		muid   string
	}{
		{model.StreamHTML, "mn1"},
		{model.StreamReference, "mn1"},
		{model.StreamRoot, "mn1"},
		{model.StreamTranslation, "mn1"},
	}
	if len(got) != len(want) {
		t.Fatalf("Walk found %d files (%v); want the mn1 quartet %d", len(got), got, len(want))
	}
	for i, w := range want {
		if got[i].Stream != w.stream || got[i].Muid != w.muid {
			t.Errorf("Walk[%d] = %s %s; want %s %s", i, got[i].Stream, got[i].Muid, w.stream, w.muid)
		}
	}
}

func TestWalkNames(t *testing.T) {
	root := t.TempDir()
	files := []string{
		"translation/en/sujato/name/mn/mn_name-en-sujato.json",
		"translation/en/sujato/name/pli-tv/pli-tv_name-en-sujato.json",
		"translation/en/sujato/name/mn/readme.txt",
		"translation/en/sujato/sutta/mn/mn1_translation-en-sujato.json",
		"root/pli/ms/sutta/mn/mn1_root-pli-ms.json",
		"html/pli/ms/sutta/mn/mn1_html.json",
	}
	for _, f := range files {
		p := filepath.Join(root, filepath.FromSlash(f))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(root, "translation", "en", "sujato", "name", "other.json"), 0o755); err != nil {
		t.Fatal(err)
	}

	var got []string
	if err := WalkNames(root, func(p string) error {
		got = append(got, p)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"translation/en/sujato/name/mn/mn_name-en-sujato.json",
		"translation/en/sujato/name/pli-tv/pli-tv_name-en-sujato.json",
	}
	if len(got) != len(want) {
		t.Fatalf("WalkNames found %v; want %v", got, want)
	}
	for i, w := range want {
		if p := filepath.ToSlash(got[i]); !strings.HasSuffix(p, w) {
			t.Errorf("WalkNames[%d] = %q; want suffix %q", i, p, w)
		}
	}
}
