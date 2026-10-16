package dpd

import (
	"archive/tar"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ulikunitz/xz"
)

type tarMember struct {
	name string
	body string
	dir  bool
	link string
}

var memberModTime = time.Unix(0, 0).UTC()

func buildTarXZBytes(members []tarMember) ([]byte, error) {
	var raw bytes.Buffer
	tw := tar.NewWriter(&raw)
	for _, m := range members {
		hdr := &tar.Header{Name: m.name, Mode: 0o644, Size: int64(len(m.body)), ModTime: memberModTime}
		switch {
		case m.dir:
			hdr.Typeflag = tar.TypeDir
			hdr.Size = 0
		case m.link != "":
			hdr.Typeflag = tar.TypeSymlink
			hdr.Size = 0
			hdr.Linkname = m.link
		default:
			hdr.Typeflag = tar.TypeReg
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return nil, err
		}
		if hdr.Typeflag == tar.TypeReg && hdr.Size > 0 {
			if _, err := tw.Write([]byte(m.body)); err != nil {
				return nil, err
			}
		}
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	var out bytes.Buffer
	xw, err := xz.NewWriter(&out)
	if err != nil {
		return nil, err
	}
	if _, err := xw.Write(raw.Bytes()); err != nil {
		return nil, err
	}
	if err := xw.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func buildTarXZ(t *testing.T, members []tarMember) []byte {
	t.Helper()
	b, err := buildTarXZBytes(members)
	if err != nil {
		t.Fatalf("build tar.xz: %v", err)
	}
	return b
}

func writeTarball(t *testing.T, b []byte) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "dpd.db.tar.xz")
	if err := os.WriteFile(p, b, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func dbPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "dpd", "dpd.db")
}

func dirFiles(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

const bigMax = int64(1) << 30

func TestExtractRoundTrip(t *testing.T) {
	payload := "sqlite-format-3 pretend bytes"
	tarball := writeTarball(t, buildTarXZ(t, []tarMember{{name: "dpd.db", body: payload}}))
	db := dbPath(t)

	if err := Extract(tarball, db, bigMax); err != nil {
		t.Fatalf("Extract: %v", err)
	}
	got, err := os.ReadFile(db)
	if err != nil {
		t.Fatalf("extracted db missing: %v", err)
	}
	if string(got) != payload {
		t.Errorf("extracted db = %q, want %q", got, payload)
	}
	if _, err := os.Stat(tarball); !os.IsNotExist(err) {
		t.Errorf("tarball must be removed on success, stat err = %v", err)
	}
}

func TestExtractWrongMembers(t *testing.T) {
	cases := []struct {
		name    string
		members []tarMember
		want    string
	}{
		{
			"two regular members",
			[]tarMember{{name: "dpd.db", body: "a"}, {name: "extra.txt", body: "b"}},
			"extra.txt",
		},
		{
			"single member wrong name",
			[]tarMember{{name: "other.db", body: "a"}},
			"other.db",
		},
		{
			"duplicate dpd.db",
			[]tarMember{{name: "dpd.db", body: "a"}, {name: "dpd.db", body: "b"}},
			"dpd.db",
		},
		{
			"no regular members",
			[]tarMember{{name: "somedir/", dir: true}},
			"no regular",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tarball := writeTarball(t, buildTarXZ(t, tc.members))
			db := dbPath(t)
			err := Extract(tarball, db, bigMax)
			if err == nil {
				t.Fatalf("Extract: expected error naming %q", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %v, want it to name %q", err, tc.want)
			}
			if _, err := os.Stat(db); err == nil {
				t.Error("failed extract must not leave a db behind")
			}
			if files := dirFiles(t, filepath.Dir(db)); len(files) != 0 {
				t.Errorf("failed extract left files behind: %v", files)
			}
			if _, err := os.Stat(tarball); err != nil {
				t.Errorf("failed extract must keep the tarball for inspection: %v", err)
			}
		})
	}
}

func TestExtractSymlinkMember(t *testing.T) {
	tarball := writeTarball(t, buildTarXZ(t, []tarMember{
		{name: "dpd.db", body: "real"},
		{name: "alias", link: "dpd.db"},
	}))
	err := Extract(tarball, dbPath(t), bigMax)
	if err == nil {
		t.Fatal("Extract with a symlink member: expected error")
	}
	if !strings.Contains(err.Error(), "alias") || !strings.Contains(err.Error(), "not a regular") {
		t.Errorf("error = %v, want it to name the symlink and the cause", err)
	}
}

func TestExtractDirEntryBesideDB(t *testing.T) {
	tarball := writeTarball(t, buildTarXZ(t, []tarMember{
		{name: "somedir/", dir: true},
		{name: "dpd.db", body: "ok"},
	}))
	if err := Extract(tarball, dbPath(t), bigMax); err != nil {
		t.Fatalf("Extract with a dir entry beside dpd.db: %v", err)
	}
}

func TestExtractOversize(t *testing.T) {
	tarball := writeTarball(t, buildTarXZ(t, []tarMember{{name: "dpd.db", body: "0123456789"}}))
	db := dbPath(t)
	err := Extract(tarball, db, 9)
	if err == nil {
		t.Fatal("Extract over maxDBBytes: expected hard error")
	}
	if !strings.Contains(err.Error(), "exceeds") {
		t.Errorf("error should name the oversize cause, got %v", err)
	}
	if files := dirFiles(t, filepath.Dir(db)); len(files) != 0 {
		t.Errorf("oversize extract left files behind: %v", files)
	}
}

func TestExtractGarbage(t *testing.T) {
	for name, b := range map[string][]byte{
		"random":    []byte("definitely not xz"),
		"empty":     nil,
		"truncated": buildTarXZ(t, []tarMember{{name: "dpd.db", body: "0123456789"}})[:12],
	} {
		t.Run(name, func(t *testing.T) {
			if err := Extract(writeTarball(t, b), dbPath(t), bigMax); err == nil {
				t.Fatalf("Extract on %s input: expected error", name)
			}
		})
	}
}

func TestExtractBadMax(t *testing.T) {
	tarball := writeTarball(t, buildTarXZ(t, []tarMember{{name: "dpd.db", body: "x"}}))
	for _, max := range []int64{0, -1} {
		err := Extract(tarball, dbPath(t), max)
		if err == nil {
			t.Errorf("Extract with maxDBBytes %d: expected error", max)
			continue
		}
		if !strings.Contains(err.Error(), "maxDBBytes") {
			t.Errorf("Extract with maxDBBytes %d: error = %v, want the guard rejection", max, err)
		}
	}
}
