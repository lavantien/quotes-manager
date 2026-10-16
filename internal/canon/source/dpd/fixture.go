package dpd

import (
	"archive/tar"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/ulikunitz/xz"
)

var fixtureModTime = time.Unix(0, 0).UTC()

func WriteFixture(dir string) (dbPath, tarPath string, err error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", "", fmt.Errorf("dpd fixture: %w", err)
	}
	dbPath = filepath.Join(dir, dbMemberName)
	tarPath = filepath.Join(dir, dbMemberName+".tar.xz")
	if err := writeFixtureDB(dbPath); err != nil {
		return "", "", err
	}
	if err := writeFixtureTarball(dbPath, tarPath); err != nil {
		return "", "", err
	}
	return dbPath, tarPath, nil
}

func writeFixtureDB(path string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("dpd fixture: %w", err)
	}
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return fmt.Errorf("dpd fixture: %w", err)
	}
	defer func() { _ = db.Close() }()
	for _, stmt := range []string{
		`CREATE TABLE dpd_headwords (
			id INTEGER PRIMARY KEY,
			lemma_1 TEXT NOT NULL UNIQUE,
			lemma_2 TEXT, pos TEXT, grammar TEXT,
			meaning_1 TEXT, meaning_lit TEXT, sanskrit TEXT,
			root_key TEXT, root_sign TEXT, family_root TEXT,
			construction TEXT, compound_type TEXT, stem TEXT,
			inflections TEXT
		)`,
		`CREATE TABLE dpd_roots (
			root TEXT PRIMARY KEY,
			root_in_comps TEXT,
			root_has_verb TEXT,
			root_group TEXT, root_sign TEXT, root_meaning TEXT,
			sanskrit_root TEXT
		)`,
		`CREATE TABLE lookup (
			lookup_key TEXT PRIMARY KEY,
			headwords TEXT,
			deconstructor TEXT
		)`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("dpd fixture: create schema: %w", err)
		}
	}

	headwords := [][]any{
		{1, "eva", nil, "ind", nil, "thus, in this way", nil, "eva", nil, nil, nil, nil, nil, nil, "evaṃ"},
		{
			2, "dhamma", "dhammā", "m", "masc nom sg", "the teaching; nature; the way things are",
			"bearer", "dharma", "dhṛ", "i", "dhṛ", "dha + ma", nil, "dhamma",
			"dhammo, dhammā, dhammaṃ",
		},
		{
			3, "gacchati", nil, "v", "pr 3 sg", "he goes", nil, "gacchati",
			"gam", "ch", "gam", nil, nil, "gacchati", "gacchāmi, gacchasi, gacchāma, gacchatha",
		},
		{
			4, "suṇāti", nil, "v", "pr 3 sg", "he hears", nil, "śṛṇoti",
			"suṇ", "ā", "suṇ", nil, nil, "suṇā", "suṇāsi, sutam",
		},
		{
			5, "me", nil, "pron", "1st sg gen/dat enclitic", "of me, my", nil, "ma",
			nil, nil, nil, nil, nil, nil, nil,
		},
	}
	if err := insertRows(db,
		"INSERT INTO dpd_headwords ("+headwordColumns+") VALUES ("+placeholders(15)+")", headwords); err != nil {
		return err
	}
	roots := [][]any{
		{"dhṛ", "dharma", "･", "1", "i", "to hold, bear", "dhṛ"},
		{"gam", "gaccha", "･", "1", "", "to go", "gam"},
		{"suṇ", "suṇa", "×", "1", "ā", "to hear", "śru"},
	}
	if err := insertRows(db,
		"INSERT INTO dpd_roots ("+rootColumns+") VALUES ("+placeholders(7)+")", roots); err != nil {
		return err
	}
	lookups := [][]any{
		{"dhamma", `["dhamma"]`, nil},
		{"dhammaca", nil, `["dhamma","ca"]`},
		{"evam", `["eva"]`, nil},
		{"me", `["me"]`, nil},
		{"sutam", `["suṇāti"]`, nil},
	}
	return errors.Join(
		insertRows(db, "INSERT INTO lookup ("+lookupColumns+") VALUES ("+placeholders(3)+")", lookups),
		db.Close(),
	)
}

func insertRows(db *sql.DB, query string, rows [][]any) error {
	for _, row := range rows {
		if _, err := db.Exec(query, row...); err != nil {
			return fmt.Errorf("dpd fixture: insert: %w", err)
		}
	}
	return nil
}

func placeholders(n int) string {
	s := ""
	for i := 0; i < n; i++ {
		if i > 0 {
			s += ","
		}
		s += "?"
	}
	return s
}

func writeFixtureTarball(dbPath, tarPath string) error {
	f, err := os.Open(dbPath)
	if err != nil {
		return fmt.Errorf("dpd fixture: %w", err)
	}
	defer func() { _ = f.Close() }()
	fi, err := f.Stat()
	if err != nil {
		return fmt.Errorf("dpd fixture: %w", err)
	}
	out, err := os.Create(tarPath)
	if err != nil {
		return fmt.Errorf("dpd fixture: %w", err)
	}
	xw, err := xz.NewWriter(out)
	if err != nil {
		_ = out.Close()
		return fmt.Errorf("dpd fixture: %w", err)
	}
	tw := tar.NewWriter(xw)
	hdr := &tar.Header{
		Typeflag: tar.TypeReg, Name: dbMemberName,
		Size: fi.Size(), Mode: 0o644, ModTime: fixtureModTime,
	}
	if err := tw.WriteHeader(hdr); err != nil {
		_ = tw.Close()
		_ = xw.Close()
		_ = out.Close()
		return fmt.Errorf("dpd fixture: tar header: %w", err)
	}
	if _, err := io.Copy(tw, f); err != nil {
		_ = tw.Close()
		_ = xw.Close()
		_ = out.Close()
		return fmt.Errorf("dpd fixture: tar copy: %w", err)
	}
	for _, closer := range []struct {
		name string
		fn   func() error
	}{
		{"tar close", tw.Close},
		{"xz close", xw.Close},
		{"file close", out.Close},
	} {
		if err := closer.fn(); err != nil {
			_ = os.Remove(tarPath)
			return fmt.Errorf("dpd fixture: %s: %w", closer.name, err)
		}
	}
	return nil
}
