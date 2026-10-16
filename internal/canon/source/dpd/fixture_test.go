package dpd

import (
	"archive/tar"
	"database/sql"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ulikunitz/xz"
)

func queryCount(t *testing.T, db *sql.DB, table string) int {
	t.Helper()
	var n int
	if err := db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&n); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return n
}

func openPlain(t *testing.T, path string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Ping(); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestWriteFixtureFiles(t *testing.T) {
	dir := t.TempDir()
	dbPath, tarPath, err := WriteFixture(dir)
	if err != nil {
		t.Fatalf("WriteFixture: %v", err)
	}
	if want := filepath.Join(dir, "dpd.db"); dbPath != want {
		t.Errorf("dbPath = %q, want %q", dbPath, want)
	}
	if want := filepath.Join(dir, "dpd.db.tar.xz"); tarPath != want {
		t.Errorf("tarPath = %q, want %q", tarPath, want)
	}
	for _, p := range []string{dbPath, tarPath} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("missing fixture file %s: %v", p, err)
		}
	}
}

func TestWriteFixtureRows(t *testing.T) {
	dir := t.TempDir()
	dbPath, _, err := WriteFixture(dir)
	if err != nil {
		t.Fatalf("WriteFixture: %v", err)
	}
	db := openPlain(t, dbPath)

	var roots []string
	rows, err := db.Query("SELECT root FROM dpd_roots ORDER BY root")
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var r string
		if err := rows.Scan(&r); err != nil {
			t.Fatal(err)
		}
		roots = append(roots, r)
	}
	_ = rows.Close()
	if !slices.Contains(roots, "gam") || len(roots) < 2 {
		t.Errorf("roots = %v, want at least gam plus one more", roots)
	}

	var keys []string
	rows, err = db.Query("SELECT lookup_key FROM lookup ORDER BY lookup_key")
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			t.Fatal(err)
		}
		keys = append(keys, k)
	}
	_ = rows.Close()
	for _, want := range []string{"evam", "dhamma", "sutam", "me"} {
		if !slices.Contains(keys, want) {
			t.Errorf("lookup keys %v missing %q", keys, want)
		}
	}
	var deconstructor string
	if err := db.QueryRow("SELECT deconstructor FROM lookup WHERE lookup_key = 'dhammaca'").Scan(&deconstructor); err != nil {
		t.Fatalf("dhammaca deconstructor row: %v", err)
	}
	if want := `["dhamma","ca"]`; deconstructor != want {
		t.Errorf("dhammaca deconstructor = %q, want %q", deconstructor, want)
	}

	for lemma, wantInf := range map[string]int{
		"eva":    1,
		"dhamma": 3,
		"suṇāti": 2,
	} {
		var inf string
		if err := db.QueryRow("SELECT inflections FROM dpd_headwords WHERE lemma_1 = ?", lemma).Scan(&inf); err != nil {
			t.Fatalf("headword %s: %v", lemma, err)
		}
		if got := len(strings.Split(strings.TrimSpace(inf), ",")); got != wantInf {
			t.Errorf("headword %s inflections = %d (%q), want %d", lemma, got, inf, wantInf)
		}
	}
	var meInf any
	if err := db.QueryRow("SELECT inflections FROM dpd_headwords WHERE lemma_1 = 'me'").Scan(&meInf); err != nil {
		t.Fatalf("headword me: %v", err)
	}
	if meInf != nil {
		t.Errorf("headword me inflections = %v, want NULL", meInf)
	}
}

func TestWriteFixtureTarballShape(t *testing.T) {
	dir := t.TempDir()
	dbPath, tarPath, err := WriteFixture(dir)
	if err != nil {
		t.Fatalf("WriteFixture: %v", err)
	}
	f, err := os.Open(tarPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	xr, err := xz.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	tr := tar.NewReader(xr)
	hdr, err := tr.Next()
	if err != nil {
		t.Fatalf("first member: %v", err)
	}
	if hdr.Name != dbMemberName {
		t.Errorf("member name = %q, want %q", hdr.Name, dbMemberName)
	}
	body, err := io.ReadAll(tr)
	if err != nil {
		t.Fatal(err)
	}
	dbBytes := readFile(t, dbPath)
	if string(body) != string(dbBytes) {
		t.Error("tar member content differs from the db file")
	}
	if _, err := tr.Next(); err != io.EOF {
		t.Errorf("expected exactly one member, next returned %v", err)
	}
	fi, err := os.Stat(tarPath)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Size() > 64*1024 {
		t.Errorf("fixture tarball is %d bytes, must stay tiny (< 64 KiB)", fi.Size())
	}
}

func TestWriteFixtureRoundTripThroughExtract(t *testing.T) {
	src := t.TempDir()
	dbPath, tarPath, err := WriteFixture(src)
	if err != nil {
		t.Fatalf("WriteFixture: %v", err)
	}
	before := openPlain(t, dbPath)
	wantHeadwords := queryCount(t, before, "dpd_headwords")
	wantRoots := queryCount(t, before, "dpd_roots")
	wantLookup := queryCount(t, before, "lookup")

	outDir := t.TempDir()
	outDB := filepath.Join(outDir, "dpd", "dpd.db")
	if err := Extract(tarPath, outDB, bigMax); err != nil {
		t.Fatalf("Extract of fixture tarball: %v", err)
	}
	after := openPlain(t, outDB)
	for table, want := range map[string]int{
		"dpd_headwords": wantHeadwords,
		"dpd_roots":     wantRoots,
		"lookup":        wantLookup,
	} {
		if got := queryCount(t, after, table); got != want {
			t.Errorf("extracted %s count = %d, want %d", table, got, want)
		}
	}
}

func TestWriteFixtureDeterministic(t *testing.T) {
	a, b := t.TempDir(), t.TempDir()
	_, tarA, err := WriteFixture(a)
	if err != nil {
		t.Fatalf("WriteFixture: %v", err)
	}
	_, tarB, err := WriteFixture(b)
	if err != nil {
		t.Fatalf("WriteFixture: %v", err)
	}
	if string(readFile(t, tarA)) != string(readFile(t, tarB)) {
		t.Error("fixture tarball bytes differ between runs")
	}
}

func TestWriteFixtureOverwrites(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := WriteFixture(dir); err != nil {
		t.Fatal(err)
	}
	if _, _, err := WriteFixture(dir); err != nil {
		t.Fatalf("second WriteFixture in the same dir: %v", err)
	}
}

func TestWriteFixtureBadDir(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := WriteFixture(filepath.Join(blocker, "sub")); err == nil {
		t.Fatal("WriteFixture under a file path: expected error")
	}
}
