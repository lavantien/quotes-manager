package pipeline

import (
	"database/sql"
	"encoding/json"
	"os"
	"strings"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"github.com/lavantien/quotes-manager/internal/canon/model"
	"github.com/lavantien/quotes-manager/internal/canon/source/dpd"
	"github.com/lavantien/quotes-manager/internal/config"
)

func syncDpdFixture(t *testing.T, cfg *config.Config) {
	t.Helper()
	if _, _, err := dpd.WriteFixture(cfg.Canon.DpdDir()); err != nil {
		t.Fatal(err)
	}
}

func TestParseDpdAbsent(t *testing.T) {
	cfg, _ := testConfig(t)
	stats, err := parseDpd(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !stats.DpdAbsent || stats.DpdHeadwords != 0 || stats.DpdForms != 0 {
		t.Errorf("absent stats = %+v", stats)
	}
	for _, p := range []string{cfg.Canon.DpdHeadwordsFile(), cfg.Canon.DpdRootsFile(), cfg.Canon.DpdFormsFile()} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("%s must not exist when dpd is absent (stat err = %v)", p, err)
		}
	}
}

func writePaddedLemmaDB(t *testing.T, cfg *config.Config) {
	t.Helper()
	if err := os.MkdirAll(cfg.Canon.DpdDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite3", cfg.Canon.DpdDB())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	for _, stmt := range []string{
		`CREATE TABLE dpd_headwords (
			id INTEGER PRIMARY KEY, lemma_1 TEXT NOT NULL UNIQUE, lemma_2 TEXT,
			pos TEXT, grammar TEXT, meaning_1 TEXT, meaning_lit TEXT, sanskrit TEXT,
			root_key TEXT, root_sign TEXT, family_root TEXT, construction TEXT,
			compound_type TEXT, stem TEXT, inflections TEXT)`,
		`CREATE TABLE dpd_roots (
			root TEXT PRIMARY KEY, root_in_comps TEXT, root_has_verb TEXT,
			root_group TEXT, root_sign TEXT, root_meaning TEXT, sanskrit_root TEXT)`,
		`CREATE TABLE lookup (lookup_key TEXT PRIMARY KEY, headwords TEXT, deconstructor TEXT)`,
		`INSERT INTO dpd_headwords VALUES (1, ' dhamma ', ' dhammā ', 'm', NULL,
			'the teaching', NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL,
			' dhammo , dhammaṃ ')`,
		`INSERT INTO dpd_roots VALUES ('dhṛ', NULL, '･', NULL, NULL, 'to hold', NULL)`,
		`INSERT INTO lookup VALUES (' eva ', NULL, NULL)`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("padded db: %v", err)
		}
	}
}

func TestParseDpdTrimsLemmaPadding(t *testing.T) {
	cfg, _ := testConfig(t)
	writePaddedLemmaDB(t, cfg)

	stats, err := parseDpd(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if stats.DpdForms != 4 {
		t.Errorf("forms = %d; want 4", stats.DpdForms)
	}
	forms := map[string]model.DpdForm{}
	for _, l := range readLines(t, cfg.Canon.DpdFormsFile()) {
		var f model.DpdForm
		if err := json.Unmarshal([]byte(l), &f); err != nil {
			t.Fatal(err)
		}
		forms[f.Form] = f
	}
	for _, key := range []string{"dhamma", "dhammo", "dhammam", "eva"} {
		if _, ok := forms[key]; !ok {
			t.Errorf("trimmed key %q missing from forms %v", key, forms)
		}
	}
	for _, bad := range []string{" dhamma ", "dhamma ", " eva ", " dhammo "} {
		if _, ok := forms[bad]; ok {
			t.Errorf("padded key %q must not exist", bad)
		}
	}
	if f := forms["dhamma"]; len(f.Headwords) != 1 || f.Headwords[0] != " dhamma " {
		t.Errorf("headword reference = %v, want the raw lemma", f.Headwords)
	}
}

func TestParseDpdFixtureArtifacts(t *testing.T) {
	cfg, _ := testConfig(t)
	syncDpdFixture(t, cfg)

	stats, err := parseDpd(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if stats.DpdAbsent {
		t.Fatal("fixture present must not count as absent")
	}
	if stats.DpdHeadwords != 5 || stats.DpdRoots != 3 {
		t.Errorf("headwords/roots = %d/%d; want 5/3", stats.DpdHeadwords, stats.DpdRoots)
	}
	if stats.DpdForms != 15 {
		t.Errorf("forms = %d; want 15", stats.DpdForms)
	}

	hwLines := readLines(t, cfg.Canon.DpdHeadwordsFile())
	if len(hwLines) != 5 {
		t.Fatalf("headword lines = %d; want 5", len(hwLines))
	}
	var first model.DpdHeadword
	if err := json.Unmarshal([]byte(hwLines[0]), &first); err != nil {
		t.Fatal(err)
	}
	if first.Lemma1 != "dhamma" || first.Meaning1 != "the teaching; nature; the way things are" {
		t.Errorf("first headword = %+v", first)
	}
	if strings.Contains(hwLines[0], "inflections") {
		t.Errorf("headword record must not carry inflections: %s", hwLines[0])
	}

	formLines := readLines(t, cfg.Canon.DpdFormsFile())
	if len(formLines) != 15 {
		t.Fatalf("form lines = %d; want 15", len(formLines))
	}
	forms := map[string]model.DpdForm{}
	prev := ""
	for _, l := range formLines {
		var f model.DpdForm
		if err := json.Unmarshal([]byte(l), &f); err != nil {
			t.Fatal(err)
		}
		if prev != "" && f.Form <= prev {
			t.Fatalf("forms not sorted: %s after %s", f.Form, prev)
		}
		prev = f.Form
		forms[f.Form] = f
	}
	if f := forms["dhamma"]; len(f.Headwords) != 1 || f.Headwords[0] != "dhamma" {
		t.Errorf("dhamma form = %+v", f)
	}
	if f := forms["sutam"]; len(f.Headwords) != 1 || f.Headwords[0] != "suṇāti" {
		t.Errorf("sutam form = %+v; headwords must come from inflections, keyed by raw lemma_1", f)
	}
	if f := forms["dhammaca"]; len(f.Headwords) != 0 || len(f.Splits) != 2 || f.Splits[0] != "dhamma" || f.Splits[1] != "ca" {
		t.Errorf("dhammaca form = %+v; want lookup-only key with the deconstructor split", f)
	}
	if _, ok := forms["me"]; !ok {
		t.Error("bare lemma_1 fold (me) missing from forms")
	}

	rootLines := readLines(t, cfg.Canon.DpdRootsFile())
	if len(rootLines) != 3 {
		t.Fatalf("root lines = %d; want 3", len(rootLines))
	}
}

func TestParseWiresDpdStats(t *testing.T) {
	cfg, _ := testConfig(t)
	writeBilaraTree(t, cfg.Canon.BilaraDir())
	syncDpdFixture(t, cfg)

	stats, err := Parse(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if stats.DpdAbsent || stats.DpdHeadwords != 5 || stats.DpdRoots != 3 || stats.DpdForms != 15 {
		t.Errorf("parse stats dpd half = %+v", stats)
	}
}
