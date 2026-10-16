package pipeline

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lavantien/quotes-manager/internal/canon/graph"
	"github.com/lavantien/quotes-manager/internal/config"
)

func writeParsedCorpus(t *testing.T, cfg *config.Config) {
	t.Helper()
	dir := cfg.Canon.ParsedDir()
	writeFile(t, filepath.Join(dir, "root.ndjson"), `{"stream":"root","muid":"mn1","seg_key":"0.1","text":"Middle Discourses 1"}
{"stream":"root","muid":"mn1","seg_key":"1.1","text":"So I have heard."}
{"stream":"root","muid":"mn1","seg_key":"2.1","text":"Deep indeed."}
{"stream":"root","muid":"mn2","seg_key":"1.1","text":"All the defilements."}
{"stream":"root","muid":"pli-tv-bu-pm","seg_key":"0.1","text":"The monks' rules."}
`)
	writeFile(t, filepath.Join(dir, "translation.ndjson"), `{"stream":"translation","muid":"mn1","seg_key":"0.1","text":"Middle Discourses 1","author":"sujato"}
{"stream":"translation","muid":"mn1","seg_key":"1.1","text":"So I have heard.","author":"sujato"}
{"stream":"translation","muid":"mn1","seg_key":"2.1","text":"Deep indeed.","author":"sujato"}
{"stream":"translation","muid":"mn2","seg_key":"9.9","text":"Orphan by missing root key.","author":"brahmali"}
`)
	writeFile(t, filepath.Join(dir, "html.ndjson"), `{"stream":"html","muid":"mn1","seg_key":"0.1","text":"<p>"}
`)
	writeFile(t, filepath.Join(dir, "reference.ndjson"), `{"stream":"reference","muid":"mn1","seg_key":"0.1","text":"MN i 1"}
`)
	writeFile(t, filepath.Join(dir, "names.ndjson"), `{"muid":"mn1","lang":"en","name":"The Root of All Things"}
{"muid":"mn","lang":"en","name":"The Middle Discourses"}
`)
	writeFile(t, filepath.Join(dir, "meta.json"), `{"files":{
"root/pli/ms/sutta/mn/mn1_root-pli-ms.json":{"dup_keys":[],"nfc_changed":false,"empty_values":0,"muid":"mn1","stream":"root"},
"root/pli/ms/sutta/mn/mn2_root-pli-ms.json":{"dup_keys":[],"nfc_changed":false,"empty_values":0,"muid":"mn2","stream":"root"},
"root/pli/ms/vinaya/pli-tv-bu-pm_root-pli-ms.json":{"dup_keys":[],"nfc_changed":false,"empty_values":0,"muid":"pli-tv-bu-pm","stream":"root"},
"translation/en/sujato/sutta/mn/mn1_translation-en-sujato.json":{"dup_keys":[],"nfc_changed":false,"empty_values":0,"muid":"mn1","stream":"translation"},
"translation/en/brahmali/sutta/mn/mn2_translation-en-brahmali.json":{"dup_keys":[],"nfc_changed":false,"empty_values":0,"muid":"mn2","stream":"translation"},
"html/pli/ms/sutta/mn/mn1_html.json":{"dup_keys":[],"nfc_changed":false,"empty_values":0,"muid":"mn1","stream":"html"},
"reference/pli/ms/sutta/mn/mn1_reference.json":{"dup_keys":[],"nfc_changed":false,"empty_values":0,"muid":"mn1","stream":"reference"}
}}`)
}

func writeTestManifest(t *testing.T, cfg *config.Config) {
	t.Helper()
	m := Manifest{}
	m.Updated = "2026-01-02T00:00:00Z"
	m.Bilara.Commit = "bcommit"
	m.Bilara.Date = "2026-01-02"
	m.Editions.Commit = "ecommit"
	m.Htmls = []HtmlEdition{{
		Author: "sujato", Collection: "mn",
		FileName: "A-sujato-2026-01-02.html", BuildDate: "2026-01-02",
	}}
	if err := WriteManifest(cfg, m); err != nil {
		t.Fatal(err)
	}
}

func TestLoadBuildsGraphAndPublishes(t *testing.T) {
	cfg, _ := testConfig(t)
	writeParsedCorpus(t, cfg)
	writeTestManifest(t, cfg)

	res, err := Load(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if res.DBPath == "" || res.RunID == "" {
		t.Fatalf("load result = %+v", res)
	}
	curID, curPath, err := graph.CurrentDB(cfg)
	if err != nil {
		t.Fatalf("pointer after load: %v", err)
	}
	if curID != res.RunID || curPath != res.DBPath {
		t.Errorf("pointer = %s/%s; want %s/%s", curID, curPath, res.RunID, res.DBPath)
	}

	store, err := graph.Open(curPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	one := func(q string, args ...any) string {
		t.Helper()
		var s string
		if err := store.DB().QueryRow(q, args...).Scan(&s); err != nil {
			t.Fatalf("query %q: %v", q, err)
		}
		return s
	}
	count := func(q string, args ...any) int {
		t.Helper()
		var n int
		if err := store.DB().QueryRow(q, args...).Scan(&n); err != nil {
			t.Fatalf("query %q: %v", q, err)
		}
		return n
	}

	if got := one(`SELECT data FROM node WHERE type='text' AND key='mn1'`); !jsonStrEq(t, got, "name", "The Root of All Things") {
		t.Errorf("mn1 text payload = %s", got)
	}
	if got := one(`SELECT data FROM node WHERE type='segment' AND key='pli:mn1:1.1'`); !jsonStrEq(t, got, "text", "So I have heard.") {
		t.Errorf("pali segment payload = %s", got)
	}

	for _, div := range []string{"mn", "vinaya/bu-pm"} {
		if n := count(`SELECT COUNT(*) FROM node WHERE type='division' AND key=?`, div); n != 1 {
			t.Errorf("division %s count = %d", div, n)
		}
	}
	if n := count(`SELECT COUNT(*) FROM node WHERE type='collection' AND key IN ('sutta','vinaya')`); n != 2 {
		t.Errorf("collection nodes = %d; want 2", n)
	}

	if n := count(`SELECT COUNT(*) FROM edge e
		JOIN node s ON s.id = e.src AND s.key = 'pli:mn1:2.1'
		JOIN node d ON d.id = e.dst AND d.type = 'text' AND d.key = 'mn1'
		WHERE e.type = 'CHILD_OF' AND e.ord = 2`); n != 1 {
		t.Errorf("pali ord 2 for mn1:2.1 not found (%d)", n)
	}

	if got := one(`SELECT e.data FROM edge e
		JOIN node src ON src.id = e.src AND src.key = 'en/sujato:mn1:1.1'
		JOIN node dst ON dst.id = e.dst AND dst.key = 'pli:mn1:1.1'
		WHERE e.type = 'TRANSLATES'`); !jsonStrEq(t, got, "author", "sujato") {
		t.Errorf("translates payload = %s", got)
	}
	if n := count(`SELECT COUNT(*) FROM edge e
		JOIN node src ON src.id = e.src AND src.key = 'en/brahmali:mn2:9.9'
		WHERE e.type = 'TRANSLATES'`); n != 0 {
		t.Errorf("orphan translation must have no TRANSLATES edge, got %d", n)
	}

	if n := count(`SELECT COUNT(*) FROM node WHERE type='segment' AND key IN ('html:mn1:0.1','reference:mn1:0.1')`); n != 2 {
		t.Errorf("html/reference nodes = %d; want 2", n)
	}
	if n := count(`SELECT COUNT(*) FROM edge e
		JOIN node src ON src.id = e.src AND src.key LIKE 'html:%'
		WHERE e.type = 'CHILD_OF'`); n != 0 {
		t.Errorf("html segments must not link CHILD_OF, got %d", n)
	}

	if n := count(`SELECT COUNT(*) FROM edge e
		JOIN node ed ON ed.id = e.src AND ed.type = 'edition' AND ed.key = 'sujato/mn'
		JOIN node src ON src.id = e.dst AND src.type = 'source' AND src.key = 'editions'
		WHERE e.type = 'PART_OF'`); n != 1 {
		t.Errorf("edition PART_OF source edge missing (%d)", n)
	}
	if got := one(`SELECT data FROM node WHERE type='edition' AND key='sujato/mn'`); !jsonStrEq(t, got, "file_name", "A-sujato-2026-01-02.html") {
		t.Errorf("edition payload = %s", got)
	}

	ctx := t.Context()
	pairs, err := store.SearchFTS(ctx, "deep", "en", 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(pairs) != 1 || pairs[0].Muid != "mn1" || pairs[0].En != "Deep indeed." {
		t.Errorf("fts search = %+v", pairs)
	}

	var bcommit, finished string
	if err := store.DB().QueryRow(`SELECT bilara_commit, finished FROM run ORDER BY id DESC LIMIT 1`).Scan(&bcommit, &finished); err != nil {
		t.Fatal(err)
	}
	if bcommit != "bcommit" || finished == "" {
		t.Errorf("run row = %s/%s", bcommit, finished)
	}

	dbs := 0
	dbEntries, _ := os.ReadDir(cfg.Canon.Paths.DBDir)
	for _, e := range dbEntries {
		if strings.HasPrefix(e.Name(), "canon-") && filepath.Ext(e.Name()) == ".db" {
			dbs++
		}
	}
	if dbs != 1 {
		t.Errorf("published databases = %d; want 1", dbs)
	}
}

func TestLoadGroupedRangeFiles(t *testing.T) {
	cfg, _ := testConfig(t)
	dir := cfg.Canon.ParsedDir()
	writeFile(t, filepath.Join(dir, "root.ndjson"), `{"stream":"root","muid":"an1.1","seg_key":"0.1","text":"Ones 1"}
{"stream":"root","muid":"an1.1","seg_key":"1.1","text":"Mind is the forerunner."}
{"stream":"root","muid":"an1.2","seg_key":"1.1","text":"Not by hatred."}
`)
	writeFile(t, filepath.Join(dir, "translation.ndjson"), `{"stream":"translation","muid":"an1.1","seg_key":"1.1","text":"Mind is the forerunner.","author":"sujato"}
{"stream":"translation","muid":"an1.2","seg_key":"1.1","text":"Not by hatred.","author":"sujato"}
`)
	writeFile(t, filepath.Join(dir, "meta.json"), `{"files":{
"root/pli/ms/sutta/an/an1/an1.1-2_root-pli-ms.json":{"dup_keys":[],"nfc_changed":false,"empty_values":0,"muid":"an1.1-2","stream":"root","record_muids":["an1.1","an1.2"],"prefix_mismatches":[]},
"translation/en/sujato/sutta/an/an1/an1.1-2_translation-en-sujato.json":{"dup_keys":[],"nfc_changed":false,"empty_values":0,"muid":"an1.1-2","stream":"translation","record_muids":["an1.1","an1.2"],"prefix_mismatches":[]}
}}`)

	res, err := Load(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	store, err := graph.Open(res.DBPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()
	ctx := t.Context()

	for _, muid := range []string{"an1.1", "an1.2"} {
		var data string
		if err := store.DB().QueryRowContext(ctx,
			`SELECT data FROM node WHERE type='text' AND key=?`, muid).Scan(&data); err != nil {
			t.Fatalf("text node %s: %v", muid, err)
		}
		var p struct {
			FileMuid string `json:"file_muid"`
			FileOrd  int    `json:"file_ord"`
		}
		if err := json.Unmarshal([]byte(data), &p); err != nil {
			t.Fatal(err)
		}
		if p.FileMuid != "an1.1-2" {
			t.Errorf("%s file_muid = %q", muid, p.FileMuid)
		}
		if want := map[string]int{"an1.1": 1, "an1.2": 2}[muid]; p.FileOrd != want {
			t.Errorf("%s file_ord = %d; want %d", muid, p.FileOrd, want)
		}
		var div int
		if err := store.DB().QueryRowContext(ctx, `
			SELECT COUNT(*) FROM edge e
			JOIN node d ON d.id = e.dst AND d.type = 'division' AND d.key = 'an'
			JOIN node t ON t.id = e.src AND t.type = 'text' AND t.key = ?
			WHERE e.type = 'CHILD_OF'`, muid).Scan(&div); err != nil || div != 1 {
			t.Errorf("%s CHILD_OF an division = %d/%v", muid, div, err)
		}
	}
	var fileMuidNodes int
	if err := store.DB().QueryRowContext(ctx,
		`SELECT COUNT(*) FROM node WHERE type='text' AND key='an1.1-2'`).Scan(&fileMuidNodes); err != nil || fileMuidNodes != 0 {
		t.Errorf("file muid must not be a text node: %d/%v", fileMuidNodes, err)
	}

	var pairs int
	if err := store.DB().QueryRowContext(ctx, `
		SELECT COUNT(*) FROM edge e
		JOIN node en ON en.id = e.src AND en.key LIKE 'en/sujato:an1.%'
		JOIN node pl ON pl.id = e.dst AND pl.key LIKE 'pli:an1.%'
		WHERE e.type = 'TRANSLATES'`).Scan(&pairs); err != nil || pairs != 2 {
		t.Errorf("grouped TRANSLATES pairs = %d/%v; want 2", pairs, err)
	}
}

func TestLoadFailsWithoutManifestWhenEditionsPresent(t *testing.T) {
	cfg, _ := testConfig(t)
	writeParsedCorpus(t, cfg)
	writeFile(t, filepath.Join(cfg.Canon.EditionsDir(), "en/sujato/mn/html/A-sujato-2026-01-02.html"), "<html></html>")

	if _, err := Load(t.Context(), cfg); err == nil {
		t.Fatal("load must fail when the editions tree exists but the manifest does not")
	}
}

func jsonStrEq(t *testing.T, payload, field, want string) bool {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(payload), &m); err != nil {
		t.Fatalf("bad payload %s: %v", payload, err)
	}
	return m[field] == want
}

func TestLoadSecondRunRetiresFirstDB(t *testing.T) {
	cfg, _ := testConfig(t)
	writeParsedCorpus(t, cfg)
	writeTestManifest(t, cfg)

	first, err := Load(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Load(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(first.DBPath); !os.IsNotExist(err) {
		t.Errorf("first database must be retired, stat err = %v", err)
	}
	if _, err := os.Stat(second.DBPath); err != nil {
		t.Fatalf("second database missing: %v", err)
	}
	curID, _, err := graph.CurrentDB(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if curID != second.RunID {
		t.Errorf("pointer run = %s; want %s", curID, second.RunID)
	}
}

func TestDeriveDivision(t *testing.T) {
	cases := []struct {
		rel, muid, col, div string
	}{
		{"root/pli/ms/sutta/mn/mn1_root-pli-ms.json", "mn1", "sutta", "mn"},
		{"root/pli/ms/sutta/kn/dhp/dhp1-20_root-pli-ms.json", "dhp1-20", "sutta", "kn"},
		{"root/pli/ms/vinaya/pli-tv-bu-vb/pli-tv-bu-vb-1_root-pli-ms.json", "pli-tv-bu-vb-1", "vinaya", "vinaya/bu-vb"},
		{"root/pli/ms/vinaya/pli-tv-bu-pm_root-pli-ms.json", "pli-tv-bu-pm", "vinaya", "vinaya/bu-pm"},
		{"translation/en/sujato/vinaya/pli-tv-kd/pli-tv-kd1_translation-en-sujato.json", "pli-tv-kd1", "vinaya", "vinaya/kd"},
		{"html/pli/ms/sutta/dn/dn1_html.json", "dn1", "sutta", "dn"},
	}
	for _, c := range cases {
		col, div, err := deriveDivision(c.rel, c.muid)
		if err != nil {
			t.Errorf("%s: %v", c.rel, err)
			continue
		}
		if col != c.col || div != c.div {
			t.Errorf("deriveDivision(%s) = %s/%s; want %s/%s", c.rel, col, div, c.col, c.div)
		}
	}
	if _, _, err := deriveDivision("comment/en/foo.json", "foo"); err == nil {
		t.Error("paths outside sutta/vinaya must error")
	}
}

func TestNewestEditionsPicksNewestBuild(t *testing.T) {
	htmls := []HtmlEdition{
		{Author: "sujato", Collection: "mn", BuildDate: "2026-01-02", FileName: "A"},
		{Author: "sujato", Collection: "mn", BuildDate: "2026-03-04", FileName: "B"},
		{Author: "brahmali", Collection: "pli-tv-vi", BuildDate: "2026-02-02", FileName: "C"},
	}
	got := newestEditions(htmls)
	if len(got) != 2 {
		t.Fatalf("newest editions = %d; want 2", len(got))
	}
	if got[0].Author != "brahmali" || got[1].FileName != "B" {
		t.Errorf("newest editions = %+v", got)
	}
}
