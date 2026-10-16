package pipeline

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/lavantien/quotes-manager/internal/canon/graph"
	"github.com/lavantien/quotes-manager/internal/config"
)

func writeWordCorpus(t *testing.T, cfg *config.Config) {
	t.Helper()
	dir := cfg.Canon.ParsedDir()
	writeFile(t, filepath.Join(dir, "root.ndjson"),
		`{"stream":"root","muid":"mn1","seg_key":"1.1","text":"evaṃ me sutaṃ"}
{"stream":"root","muid":"mn1","seg_key":"2.1","text":"dhammo ca dhamma"}
`)
	writeFile(t, filepath.Join(dir, "meta.json"), `{"files":{
"root/pli/ms/sutta/mn/mn1_root-pli-ms.json":{"dup_keys":[],"nfc_changed":false,"empty_values":0,"muid":"mn1","stream":"root","record_muids":["mn1"],"prefix_mismatches":[]}
}}`)
}

func writeDpdCorpus(t *testing.T, cfg *config.Config) {
	t.Helper()
	m := Manifest{}
	m.Updated = "2026-01-02T00:00:00Z"
	m.Bilara.Commit = "bcommit"
	m.Editions.Commit = "ecommit"
	m.Dpd = DpdProvenance{Tag: "v0.4.20260926", SHA256: "cafe", Size: 42, Fetched: "2026-10-01T00:00:00Z"}
	if err := WriteManifest(cfg, m); err != nil {
		t.Fatal(err)
	}
	syncDpdFixture(t, cfg)
	if _, err := parseDpd(cfg); err != nil {
		t.Fatal(err)
	}
}

func TestLoadDpdNodesEdgesAndRunTag(t *testing.T) {
	cfg, _ := testConfig(t)
	writeWordCorpus(t, cfg)
	writeDpdCorpus(t, cfg)

	res, err := Load(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	store, err := graph.Open(res.DBPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()
	db := store.DB()
	count := func(q string, args ...any) int {
		t.Helper()
		var n int
		if err := db.QueryRow(q, args...).Scan(&n); err != nil {
			t.Fatalf("query %q: %v", q, err)
		}
		return n
	}

	var srcData string
	if err := db.QueryRow(
		`SELECT data FROM node WHERE type='source' AND key='dpd'`).Scan(&srcData); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"commit":"v0.4.20260926"`, `"sha256":"cafe"`, `"repo":"` + cfg.Canon.Dpd.URL + `"`} {
		if !strings.Contains(srcData, want) {
			t.Errorf("dpd source payload missing %s: %s", want, srcData)
		}
	}

	if n := count(`SELECT COUNT(*) FROM node WHERE type='headword'`); n != 5 {
		t.Errorf("headword nodes = %d; want 5", n)
	}
	var hwData string
	if err := db.QueryRow(
		`SELECT data FROM node WHERE type='headword' AND key='dpd:dhamma'`).Scan(&hwData); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(hwData, `"meaning_1":"the teaching; nature; the way things are"`) {
		t.Errorf("dhamma headword payload = %s", hwData)
	}
	if strings.Contains(hwData, "sanskrit") {
		t.Errorf("headword payload must drop the non-English sanskrit field: %s", hwData)
	}

	if n := count(`SELECT COUNT(*) FROM node WHERE type='dpd_root'`); n != 3 {
		t.Errorf("dpd_root nodes = %d; want 3", n)
	}
	if n := count(`SELECT COUNT(*) FROM node WHERE type='dpd_form'`); n != 15 {
		t.Errorf("dpd_form nodes = %d; want 15", n)
	}

	if n := count(`SELECT COUNT(*) FROM edge WHERE type='DERIVED_FROM'`); n != 3 {
		t.Errorf("DERIVED_FROM edges = %d; want 3", n)
	}
	if n := count(`SELECT COUNT(*) FROM edge e
		JOIN node h ON h.id = e.src AND h.type='headword' AND h.key='dpd:dhamma'
		JOIN node r ON r.id = e.dst AND r.type='dpd_root' AND r.key='dpd-root:dhṛ'
		WHERE e.type='DERIVED_FROM'`); n != 1 {
		t.Errorf("dhamma -> dhṛ edge = %d; want 1", n)
	}

	if n := count(`SELECT COUNT(*) FROM edge WHERE type='HAS_WORD'`); n != 4 {
		t.Errorf("HAS_WORD edges = %d; want 4", n)
	}
	var forms string
	if err := db.QueryRow(`
		SELECT e.data FROM edge e
		JOIN node s ON s.id = e.src AND s.type='segment' AND s.key='pli:mn1:2.1'
		JOIN node h ON h.id = e.dst AND h.type='headword' AND h.key='dpd:dhamma'
		WHERE e.type='HAS_WORD'`).Scan(&forms); err != nil {
		t.Fatal(err)
	}
	if forms != `{"forms":["dhamma","dhammo"]}` {
		t.Errorf("HAS_WORD payload = %s; want both deduped forms sorted", forms)
	}
	if n := count(`SELECT COUNT(*) FROM edge e
		JOIN node s ON s.id = e.src AND s.key='pli:mn1:1.1'
		JOIN node h ON h.id = e.dst AND h.key='dpd:suṇāti'
		WHERE e.type='HAS_WORD'`); n != 1 {
		t.Errorf("sutaṃ -> suṇāti edge = %d; want 1", n)
	}

	var tag *string
	if err := db.QueryRow(`SELECT dpd_tag FROM run ORDER BY id DESC LIMIT 1`).Scan(&tag); err != nil {
		t.Fatal(err)
	}
	if tag == nil || *tag != "v0.4.20260926" {
		t.Errorf("run dpd_tag = %v; want the manifest tag", tag)
	}
}

func TestLinkWordsSkipsWithoutForms(t *testing.T) {
	cfg, _ := testConfig(t)
	writeFile(t, filepath.Join(cfg.Canon.ParsedDir(), "root.ndjson"), "{not json}\n")
	s, err := graph.Open(filepath.Join(cfg.Canon.Paths.DataDir, "linkwords.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	if err := s.Migrate(); err != nil {
		t.Fatal(err)
	}
	if err := linkWords(cfg, graph.NewLoader(s, 10),
		map[string]int64{"pli:mn1:1.1": 1}, map[string]int64{"dpd:dhamma": 2}); err != nil {
		t.Fatalf("linkWords without a forms file must skip, got %v", err)
	}
}

func TestLoadDpdAbsentStillLoads(t *testing.T) {
	cfg, _ := testConfig(t)
	writeParsedCorpus(t, cfg)
	writeTestManifest(t, cfg)

	res, err := Load(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	store, err := graph.Open(res.DBPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()
	db := store.DB()
	for _, q := range []string{
		`SELECT COUNT(*) FROM node WHERE type IN ('headword','dpd_root','dpd_form')`,
		`SELECT COUNT(*) FROM node WHERE type='source' AND key='dpd'`,
		`SELECT COUNT(*) FROM edge WHERE type IN ('HAS_WORD','DERIVED_FROM')`,
	} {
		var n int
		if err := db.QueryRow(q).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n != 0 {
			t.Errorf("%s = %d; want 0", q, n)
		}
	}
	var tag *string
	if err := db.QueryRow(`SELECT dpd_tag FROM run ORDER BY id DESC LIMIT 1`).Scan(&tag); err != nil {
		t.Fatal(err)
	}
	if tag != nil {
		t.Errorf("run dpd_tag = %v; want NULL", *tag)
	}
}
