package editions

import (
	"archive/zip"
	"bytes"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

func FuzzZipWalk(f *testing.F) {
	var seed bytes.Buffer
	zw := zip.NewWriter(&seed)
	w, err := zw.Create("a/b.tex")
	if err != nil {
		f.Fatal(err)
	}
	if _, err := w.Write([]byte("x\n")); err != nil {
		f.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		f.Fatal(err)
	}
	f.Add(seed.Bytes())
	f.Add([]byte("not a zip"))
	f.Fuzz(func(t *testing.T, data []byte) {
		zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			return
		}
		out := t.TempDir()
		if _, err := extractReader(zr, out, "fuzz"); err != nil {
			return
		}
		err = filepath.WalkDir(out, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(out, p)
			if err != nil {
				return err
			}
			if filepath.IsAbs(rel) || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				t.Fatalf("extracted path escapes output root: %s", p)
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk output root: %v", err)
		}
	})
}
