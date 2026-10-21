package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVersionConstants(t *testing.T) {
	if Version != "2.1.0" {
		t.Fatalf("Version = %q, want 2.1.0", Version)
	}
	if SchemaVersion != 2 {
		t.Fatalf("SchemaVersion = %d, want 2", SchemaVersion)
	}
	if EnvPrefix != "QM" {
		t.Fatalf("EnvPrefix = %q, want QM", EnvPrefix)
	}
}

func TestDefaultsValidate(t *testing.T) {
	if err := Defaults().Validate(); err != nil {
		t.Fatalf("defaults must validate: %v", err)
	}
}

func TestQuotesDefaults(t *testing.T) {
	c := Defaults()
	want := map[string]string{
		"db_path":           "database/quotes.db",
		"dumps_dir":         "dumps",
		"seed_file":         "database/seed.sql",
		"export_file":       "exports/shortest-first.md",
		"pictures_dir":      "project-pictures",
		"tables_collection": "AriyaSavaka",
	}
	got := map[string]string{
		"db_path":           c.Quotes.DBPath,
		"dumps_dir":         c.Quotes.DumpsDir,
		"seed_file":         c.Quotes.SeedFile,
		"export_file":       c.Quotes.ExportFile,
		"pictures_dir":      c.Quotes.PicturesDir,
		"tables_collection": c.Quotes.TablesCollection,
	}
	for k, w := range want {
		if got[k] != w {
			t.Fatalf("quotes %s = %q, want %q", k, got[k], w)
		}
	}
}

func TestServerDefaults(t *testing.T) {
	c := Defaults()
	if c.Server.Addr != "127.0.0.1:8080" {
		t.Fatalf("server addr = %q, want loopback 8080 default", c.Server.Addr)
	}
	if c.Server.DefaultLimit != 50 || c.Server.MaxLimit != 500 {
		t.Fatalf("server limits = %d/%d, want 50/500", c.Server.DefaultLimit, c.Server.MaxLimit)
	}
	if c.Server.GraphMaxNodes != 400 || c.Server.GraphMaxEdges != 1600 {
		t.Fatalf("server graph caps = %d/%d, want 400/1600", c.Server.GraphMaxNodes, c.Server.GraphMaxEdges)
	}
	if c.Server.JobLogLines != 1000 {
		t.Fatalf("server job log lines = %d, want 1000", c.Server.JobLogLines)
	}
	if c.Server.SQLConsole {
		t.Fatal("sql console must default off")
	}
	if c.Server.AnalyticsThreads != 2 || c.Server.AnalyticsMemoryGB != 2 {
		t.Fatalf("server analytics = %d threads / %d GB, want 2/2", c.Server.AnalyticsThreads, c.Server.AnalyticsMemoryGB)
	}
	if c.Server.ReadTimeoutSeconds != 30 {
		t.Fatalf("server read timeout = %d, want 30", c.Server.ReadTimeoutSeconds)
	}
	if c.Server.WriteTimeoutSeconds != 120 {
		t.Fatalf("server write timeout = %d, want 120", c.Server.WriteTimeoutSeconds)
	}
	if c.Server.IdleTimeoutSeconds != 120 {
		t.Fatalf("server idle timeout = %d, want 120", c.Server.IdleTimeoutSeconds)
	}
	if c.Server.ShutdownTimeoutSeconds != 15 {
		t.Fatalf("server shutdown timeout = %d, want 15", c.Server.ShutdownTimeoutSeconds)
	}
}

func TestCanonDefaults(t *testing.T) {
	c := Defaults()
	if c.Canon.Repos.Bilara.URL != "https://github.com/suttacentral/bilara-data" || c.Canon.Repos.Bilara.Branch != "published" || c.Canon.Repos.Bilara.Dir != "bilara-data" {
		t.Fatalf("bilara repo = %+v", c.Canon.Repos.Bilara)
	}
	if c.Canon.Repos.Editions.URL != "https://github.com/suttacentral/editions" || c.Canon.Repos.Editions.Branch != "main" || c.Canon.Repos.Editions.Dir != "editions" {
		t.Fatalf("editions repo = %+v", c.Canon.Repos.Editions)
	}
	if c.Canon.Bilara.RootLang != "pli" || c.Canon.Bilara.RootEdition != "ms" || c.Canon.Bilara.TransLang != "en" {
		t.Fatalf("bilara langs = %s/%s/%s", c.Canon.Bilara.RootLang, c.Canon.Bilara.RootEdition, c.Canon.Bilara.TransLang)
	}
	if len(c.Canon.Bilara.Authors) != 2 || c.Canon.Bilara.Authors[0] != "sujato" || c.Canon.Bilara.Authors[1] != "brahmali" {
		t.Fatalf("bilara authors = %v", c.Canon.Bilara.Authors)
	}
	if c.Canon.Bilara.MaxRangeExpansion != 500 {
		t.Fatalf("bilara max range expansion = %d, want 500", c.Canon.Bilara.MaxRangeExpansion)
	}
	if len(c.Canon.Bilara.SparseDirs) == 0 || len(c.Canon.Bilara.KeepRegexes) == 0 {
		t.Fatal("bilara sparse dirs and keep regexes must default non-empty")
	}
	sujato, ok := c.Canon.Editions.Authors["sujato"]
	if !ok || len(sujato) != 10 {
		t.Fatalf("editions sujato collections = %v", sujato)
	}
	if len(c.Canon.Editions.SparseDirs) == 0 {
		t.Fatal("editions sparse dirs must default non-empty")
	}
	for _, re := range c.Canon.Editions.KeepRegexes {
		if strings.Contains(re, "tex") {
			t.Fatalf("tex keep regex must be gone from editions defaults: %q", re)
		}
	}
	if c.Canon.Dpd.URL != "https://github.com/digitalpalidictionary/dpd-db/releases/latest/download/dpd.db.tar.xz" {
		t.Fatalf("dpd url = %q", c.Canon.Dpd.URL)
	}
	if c.Canon.Dpd.SHA256 != "" {
		t.Fatalf("dpd sha256 default must be empty, got %q", c.Canon.Dpd.SHA256)
	}
	if c.Canon.Dpd.MaxTarballBytes != 1<<30 || c.Canon.Dpd.MaxDBBytes != 8<<30 {
		t.Fatalf("dpd byte caps = %d/%d", c.Canon.Dpd.MaxTarballBytes, c.Canon.Dpd.MaxDBBytes)
	}
	if c.Canon.Dpd.TimeoutSeconds != 1800 || c.Canon.Dpd.MinTokenLength != 2 || c.Canon.Dpd.UnknownWarnPerMuid != 20 {
		t.Fatalf("dpd thresholds = %d/%d/%d", c.Canon.Dpd.TimeoutSeconds, c.Canon.Dpd.MinTokenLength, c.Canon.Dpd.UnknownWarnPerMuid)
	}
	if c.Canon.Load.BatchSize != 50_000 {
		t.Fatalf("load batch size = %d, want 50000", c.Canon.Load.BatchSize)
	}
	if c.Canon.Triangulate.SimilarityMin != 0.85 || c.Canon.Triangulate.EmptyWarnPerMuid != 20 || c.Canon.Triangulate.DetailSampleMax != 200 {
		t.Fatalf("triangulate thresholds = %v/%d/%d", c.Canon.Triangulate.SimilarityMin, c.Canon.Triangulate.EmptyWarnPerMuid, c.Canon.Triangulate.DetailSampleMax)
	}
	if len(c.Canon.E2E.Muids) != 0 {
		t.Fatalf("e2e muids must default to the whole corpus, got %v", c.Canon.E2E.Muids)
	}
	if !c.Canon.Export.Parquet {
		t.Fatal("parquet export must default on")
	}
}

func TestCanonPathDefaults(t *testing.T) {
	c := Defaults()
	if c.Canon.Paths.DataDir != "data" {
		t.Fatalf("canon data dir = %q, want data", c.Canon.Paths.DataDir)
	}
	if c.Canon.Paths.E2EDir != "data/e2e-live" {
		t.Fatalf("canon e2e dir = %q, want data/e2e-live", c.Canon.Paths.E2EDir)
	}
	if c.Canon.Paths.DistDir != "dist" {
		t.Fatalf("canon dist dir = %q, want dist", c.Canon.Paths.DistDir)
	}
	if c.Canon.Paths.DBDir != "database/canon" {
		t.Fatalf("canon db dir = %q, want database/canon", c.Canon.Paths.DBDir)
	}
	if c.Canon.Paths.ManifestsDir != "docs/canon/manifests" {
		t.Fatalf("canon manifests dir = %q, want docs/canon/manifests", c.Canon.Paths.ManifestsDir)
	}
	if c.Canon.Paths.ReportsDir != "docs/canon/reports" {
		t.Fatalf("canon reports dir = %q, want docs/canon/reports", c.Canon.Paths.ReportsDir)
	}
}

func TestCanonDerivedPaths(t *testing.T) {
	c := Defaults()
	if got, want := c.Canon.BilaraDir(), filepath.Join("data", "repos", "bilara-data"); got != want {
		t.Fatalf("bilara dir = %q, want %q", got, want)
	}
	if got, want := c.Canon.EditionsDir(), filepath.Join("data", "repos", "editions"); got != want {
		t.Fatalf("editions dir = %q, want %q", got, want)
	}
	if got, want := c.Canon.DpdDir(), filepath.Join("data", "dpd"); got != want {
		t.Fatalf("dpd dir = %q, want %q", got, want)
	}
	if got, want := c.Canon.ParsedDir(), filepath.Join("data", "parsed"); got != want {
		t.Fatalf("parsed dir = %q, want %q", got, want)
	}
	if got, want := c.Canon.DuckDBFile(), filepath.Join("data", "canon.duckdb"); got != want {
		t.Fatalf("duckdb file = %q, want %q", got, want)
	}
	if got, want := c.Canon.ParquetDir(), filepath.Join("data", "export", "parquet"); got != want {
		t.Fatalf("parquet dir = %q, want %q", got, want)
	}
	if got, want := c.Canon.DBFile("20261017T000000Z"), filepath.Join("database", "canon", "canon-20261017T000000Z.db"); got != want {
		t.Fatalf("canon db file = %q, want %q", got, want)
	}
	if got, want := c.Canon.DBPointerFile(), filepath.Join("database", "canon", "pointer.json"); got != want {
		t.Fatalf("canon pointer file = %q, want %q", got, want)
	}
	if got, want := c.Canon.ManifestFile(), filepath.Join("docs", "canon", "manifests", "sync.json"); got != want {
		t.Fatalf("manifest file = %q, want %q", got, want)
	}
	if got, want := c.Canon.ReportsDirFor("2026-10-17"), filepath.Join("docs", "canon", "reports", "2026-10-17"); got != want {
		t.Fatalf("reports dir = %q, want %q", got, want)
	}
	if got, want := c.Canon.DpdDB(), filepath.Join("data", "dpd", "dpd.db"); got != want {
		t.Fatalf("dpd db = %q, want %q", got, want)
	}
	if got, want := c.Canon.HtmlSegmentsFile(), filepath.Join("data", "parsed", "html-editions.ndjson"); got != want {
		t.Fatalf("html segments file = %q, want %q", got, want)
	}
}

func TestLoadFileOverlay(t *testing.T) {
	p := filepath.Join(t.TempDir(), "c.json")
	body := `{
		"quotes": {"db_path": "file.db", "dumps_dir": "filedumps", "tables_collection": "FromFile"},
		"server": {"addr": "127.0.0.1:9999", "sql_console": true},
		"canon": {"dpd": {"min_token_length": 3}, "triangulate": {"similarity_min": 0.5}, "e2e": {"muids": ["mn3"]}}
	}`
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if c.Quotes.DBPath != "file.db" || c.Quotes.DumpsDir != "filedumps" {
		t.Fatalf("quotes overlay not applied: %+v", c.Quotes)
	}
	if c.Quotes.TablesCollection != "FromFile" {
		t.Fatalf("quotes tables collection overlay not applied: %q", c.Quotes.TablesCollection)
	}
	if c.Quotes.SeedFile != "database/seed.sql" {
		t.Fatalf("untouched quotes defaults must survive: %+v", c.Quotes)
	}
	if c.Server.Addr != "127.0.0.1:9999" || !c.Server.SQLConsole {
		t.Fatalf("server overlay not applied: %+v", c.Server)
	}
	if c.Server.MaxLimit != 500 {
		t.Fatalf("untouched server defaults must survive: %d", c.Server.MaxLimit)
	}
	if c.Canon.Dpd.MinTokenLength != 3 {
		t.Fatalf("canon dpd overlay not applied: %d", c.Canon.Dpd.MinTokenLength)
	}
	if c.Canon.Dpd.TimeoutSeconds != 1800 {
		t.Fatalf("untouched canon dpd defaults must survive: %d", c.Canon.Dpd.TimeoutSeconds)
	}
	if c.Canon.Triangulate.SimilarityMin != 0.5 {
		t.Fatalf("canon triangulate overlay not applied: %v", c.Canon.Triangulate.SimilarityMin)
	}
	if len(c.Canon.E2E.Muids) != 1 || c.Canon.E2E.Muids[0] != "mn3" {
		t.Fatalf("canon e2e overlay not applied: %v", c.Canon.E2E.Muids)
	}
}

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "c.json")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoadRejectsUnknownKeys(t *testing.T) {
	bodies := []string{
		`{"nope": 1}`,
		`{"repos": {"bilara": {"url": "https://example.com/x"}}}`,
		`{"quotes": {"nope": 1}}`,
		`{"server": {"addrx": "127.0.0.1:1"}}`,
		`{"canon": {"nope": 1}}`,
		`{"canon": {"bilara": {"nope": 1}}}`,
		`{"canon": {"paths": {"nope": "x"}}}`,
		`{"canon": {"dpd": {"nope": 1}}}`,
		`{"canon": {"editions": {"nope": 1}}}`,
		`{"canon": {"paths": {"extracted_dir": "x"}}}`,
	}
	for _, body := range bodies {
		if _, err := Load(writeConfig(t, body)); err == nil {
			t.Fatalf("unknown key must be rejected: %s", body)
		}
	}
}

func TestEnvOverridesFile(t *testing.T) {
	p := writeConfig(t, `{
		"quotes": {"db_path": "from-file.db"},
		"server": {"addr": "127.0.0.1:9111", "max_limit": 200},
		"canon": {"dpd": {"url": "file:///from-file/dpd.db.tar.xz"}}
	}`)
	t.Setenv("QM_QUOTES_DB_PATH", "from-env.db")
	t.Setenv("QM_QUOTES_PICTURES_DIR", "from-env-pictures")
	t.Setenv("QM_QUOTES_TABLES_COLLECTION", "FromEnv")
	t.Setenv("QM_SERVER_ADDR", "127.0.0.1:9222")
	t.Setenv("QM_SERVER_READ_TIMEOUT_SECONDS", "45")
	t.Setenv("QM_CANON_SERVER_MAX_LIMIT", "250")
	t.Setenv("QM_CANON_SERVER_SQL_CONSOLE", "true")
	t.Setenv("QM_CANON_SERVER_ANALYTICS_THREADS", "4")
	t.Setenv("QM_CANON_DPD_URL", "file:///from-env/dpd.db.tar.xz")
	t.Setenv("QM_CANON_SIMILARITY_MIN", "0.7")
	t.Setenv("QM_CANON_BATCH_SIZE", "1234")
	t.Setenv("QM_CANON_E2E_MUIDS", "mn1,mn2")
	t.Setenv("QM_CANON_DATA_DIR", "envdata")
	t.Setenv("QM_CANON_DB_DIR", "database/envcanon")
	t.Setenv("QM_CANON_MANIFESTS_DIR", "docs/envmanifests")
	c, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if c.Quotes.DBPath != "from-env.db" {
		t.Fatalf("env quotes db path must beat file: %q", c.Quotes.DBPath)
	}
	if c.Quotes.PicturesDir != "from-env-pictures" {
		t.Fatalf("env quotes pictures dir not applied: %q", c.Quotes.PicturesDir)
	}
	if c.Quotes.TablesCollection != "FromEnv" {
		t.Fatalf("env quotes tables collection not applied: %q", c.Quotes.TablesCollection)
	}
	if c.Quotes.SeedFile != "database/seed.sql" {
		t.Fatalf("untouched quotes defaults must survive: %q", c.Quotes.SeedFile)
	}
	if c.Server.Addr != "127.0.0.1:9222" {
		t.Fatalf("env server addr must beat file: %q", c.Server.Addr)
	}
	if c.Server.ReadTimeoutSeconds != 45 {
		t.Fatalf("env server read timeout not applied: %d", c.Server.ReadTimeoutSeconds)
	}
	if c.Server.MaxLimit != 250 {
		t.Fatalf("env canon server max limit must beat file: %d", c.Server.MaxLimit)
	}
	if !c.Server.SQLConsole {
		t.Fatal("env canon server sql console not applied")
	}
	if c.Server.AnalyticsThreads != 4 {
		t.Fatalf("env canon server analytics threads not applied: %d", c.Server.AnalyticsThreads)
	}
	if c.Server.WriteTimeoutSeconds != 120 {
		t.Fatalf("untouched server timeout defaults must survive: %d", c.Server.WriteTimeoutSeconds)
	}
	if c.Canon.Dpd.URL != "file:///from-env/dpd.db.tar.xz" {
		t.Fatalf("env canon dpd url must beat file: %q", c.Canon.Dpd.URL)
	}
	if c.Canon.Triangulate.SimilarityMin != 0.7 {
		t.Fatalf("env canon similarity min not applied: %v", c.Canon.Triangulate.SimilarityMin)
	}
	if c.Canon.Load.BatchSize != 1234 {
		t.Fatalf("env canon batch size not applied: %d", c.Canon.Load.BatchSize)
	}
	if len(c.Canon.E2E.Muids) != 2 || c.Canon.E2E.Muids[0] != "mn1" || c.Canon.E2E.Muids[1] != "mn2" {
		t.Fatalf("env canon e2e muids not applied: %v", c.Canon.E2E.Muids)
	}
	if c.Canon.Paths.DataDir != "envdata" {
		t.Fatalf("env canon data dir not applied: %q", c.Canon.Paths.DataDir)
	}
	if c.Canon.Paths.DBDir != "database/envcanon" {
		t.Fatalf("env canon db dir not applied: %q", c.Canon.Paths.DBDir)
	}
	if c.Canon.Paths.ManifestsDir != "docs/envmanifests" {
		t.Fatalf("env canon manifests dir not applied: %q", c.Canon.Paths.ManifestsDir)
	}
}

func TestEnvBadValuesRejected(t *testing.T) {
	t.Setenv("QM_SERVER_READ_TIMEOUT_SECONDS", "notanint")
	if _, err := Load(""); err == nil {
		t.Fatal("bad QM_SERVER_READ_TIMEOUT_SECONDS must fail load")
	}
	t.Setenv("QM_CANON_BATCH_SIZE", "notanint")
	if _, err := Load(""); err == nil {
		t.Fatal("bad QM_CANON_BATCH_SIZE must fail load")
	}
	t.Setenv("QM_CANON_SIMILARITY_MIN", "notafloat")
	if _, err := Load(""); err == nil {
		t.Fatal("bad QM_CANON_SIMILARITY_MIN must fail load")
	}
	t.Setenv("QM_CANON_SERVER_SQL_CONSOLE", "notabool")
	if _, err := Load(""); err == nil {
		t.Fatal("bad QM_CANON_SERVER_SQL_CONSOLE must fail load")
	}
}

func TestValidateQuotesFailures(t *testing.T) {
	for name, mut := range map[string]func(*Config){
		"empty db path":           func(c *Config) { c.Quotes.DBPath = "" },
		"empty dumps dir":         func(c *Config) { c.Quotes.DumpsDir = "" },
		"empty seed file":         func(c *Config) { c.Quotes.SeedFile = "" },
		"empty export file":       func(c *Config) { c.Quotes.ExportFile = "" },
		"empty pictures dir":      func(c *Config) { c.Quotes.PicturesDir = "" },
		"empty tables collection": func(c *Config) { c.Quotes.TablesCollection = "" },
	} {
		t.Run(name, func(t *testing.T) {
			c := Defaults()
			mut(c)
			if err := c.Validate(); err == nil {
				t.Fatal("must fail validation")
			}
		})
	}
}

func TestValidateServerFailures(t *testing.T) {
	cases := []struct {
		name string
		mut  func(*Config)
	}{
		{"bad addr", func(c *Config) { c.Server.Addr = "nohostport" }},
		{"default above max", func(c *Config) { c.Server.DefaultLimit = 501 }},
		{"zero max limit", func(c *Config) { c.Server.MaxLimit = 0 }},
		{"max limit above ceiling", func(c *Config) { c.Server.MaxLimit = 10001 }},
		{"zero graph nodes", func(c *Config) { c.Server.GraphMaxNodes = 0 }},
		{"zero graph edges", func(c *Config) { c.Server.GraphMaxEdges = 0 }},
		{"graph nodes above ceiling", func(c *Config) { c.Server.GraphMaxNodes = MaxGraphNodes + 1 }},
		{"zero job log lines", func(c *Config) { c.Server.JobLogLines = 0 }},
		{"zero read timeout", func(c *Config) { c.Server.ReadTimeoutSeconds = 0 }},
		{"zero write timeout", func(c *Config) { c.Server.WriteTimeoutSeconds = 0 }},
		{"zero idle timeout", func(c *Config) { c.Server.IdleTimeoutSeconds = 0 }},
		{"shutdown over ceiling", func(c *Config) { c.Server.ShutdownTimeoutSeconds = 301 }},
		{"analytics threads zero", func(c *Config) { c.Server.AnalyticsThreads = 0 }},
		{"analytics threads high", func(c *Config) { c.Server.AnalyticsThreads = 65 }},
		{"analytics memory zero", func(c *Config) { c.Server.AnalyticsMemoryGB = 0 }},
		{"analytics memory high", func(c *Config) { c.Server.AnalyticsMemoryGB = 65 }},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			c := Defaults()
			tt.mut(c)
			if err := c.Validate(); err == nil {
				t.Fatal("must fail validation")
			}
		})
	}
}

func TestValidateGraphMaxNodesCeiling(t *testing.T) {
	c := Defaults()
	c.Server.GraphMaxNodes = MaxGraphNodes
	if err := c.Validate(); err != nil {
		t.Fatalf("graph max nodes at the ceiling must validate: %v", err)
	}
	c.Server.GraphMaxNodes = MaxGraphNodes + 1
	if err := c.Validate(); err == nil {
		t.Fatal("graph max nodes above the ceiling must fail validation")
	}
}

func TestMaxGraphNodesUnderVariableBudget(t *testing.T) {
	const sqliteVarLimit = 32766
	if 2*(MaxGraphNodes+1) >= sqliteVarLimit {
		t.Fatalf("MaxGraphNodes %d binds %d parameters over the sqlite limit %d",
			MaxGraphNodes, 2*(MaxGraphNodes+1), sqliteVarLimit)
	}
}

func TestValidateCanonFailures(t *testing.T) {
	hex64 := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	cases := []struct {
		name string
		mut  func(*Config)
	}{
		{"empty bilara url", func(c *Config) { c.Canon.Repos.Bilara.URL = "" }},
		{"empty editions branch", func(c *Config) { c.Canon.Repos.Editions.Branch = "" }},
		{"bilara empty root lang", func(c *Config) { c.Canon.Bilara.RootLang = "" }},
		{"bilara empty root edition", func(c *Config) { c.Canon.Bilara.RootEdition = "" }},
		{"bilara empty trans lang", func(c *Config) { c.Canon.Bilara.TransLang = "" }},
		{"bilara no authors", func(c *Config) { c.Canon.Bilara.Authors = nil }},
		{"bilara no sparse dirs", func(c *Config) { c.Canon.Bilara.SparseDirs = nil }},
		{"bilara zero range expansion", func(c *Config) { c.Canon.Bilara.MaxRangeExpansion = 0 }},
		{"editions no authors", func(c *Config) { c.Canon.Editions.Authors = nil }},
		{"editions no keep regexes", func(c *Config) { c.Canon.Editions.KeepRegexes = nil }},
		{"dpd ftp scheme", func(c *Config) { c.Canon.Dpd.URL = "ftp://example.com/dpd.db.tar.xz" }},
		{"dpd garbage url", func(c *Config) { c.Canon.Dpd.URL = "not a url" }},
		{"dpd zero max tarball", func(c *Config) { c.Canon.Dpd.MaxTarballBytes = 0 }},
		{"dpd zero max db", func(c *Config) { c.Canon.Dpd.MaxDBBytes = 0 }},
		{"dpd zero timeout", func(c *Config) { c.Canon.Dpd.TimeoutSeconds = 0 }},
		{"dpd zero min token", func(c *Config) { c.Canon.Dpd.MinTokenLength = 0 }},
		{"dpd sha256 short", func(c *Config) { c.Canon.Dpd.SHA256 = hex64[:63] }},
		{"dpd sha256 even length under 64", func(c *Config) { c.Canon.Dpd.SHA256 = hex64[:62] }},
		{"dpd sha256 not hex", func(c *Config) { c.Canon.Dpd.SHA256 = "zz" + hex64[2:] }},
		{"load zero batch", func(c *Config) { c.Canon.Load.BatchSize = 0 }},
		{"triangulate similarity high", func(c *Config) { c.Canon.Triangulate.SimilarityMin = 1.5 }},
		{"triangulate empty warn negative", func(c *Config) { c.Canon.Triangulate.EmptyWarnPerMuid = -1 }},
		{"triangulate detail sample zero", func(c *Config) { c.Canon.Triangulate.DetailSampleMax = 0 }},
		{"unknown check id", func(c *Config) { c.Canon.Triangulate.Severity["B999"] = "warn" }},
		{"unknown severity", func(c *Config) { c.Canon.Triangulate.Severity["B001"] = "fatal" }},
		{"unknown dotted check base", func(c *Config) { c.Canon.Triangulate.Severity["B999.x"] = "warn" }},
		{"empty data dir", func(c *Config) { c.Canon.Paths.DataDir = "" }},
		{"empty e2e dir", func(c *Config) { c.Canon.Paths.E2EDir = "" }},
		{"empty dist dir", func(c *Config) { c.Canon.Paths.DistDir = "" }},
		{"empty db dir", func(c *Config) { c.Canon.Paths.DBDir = "" }},
		{"empty manifests dir", func(c *Config) { c.Canon.Paths.ManifestsDir = "" }},
		{"empty reports dir", func(c *Config) { c.Canon.Paths.ReportsDir = "" }},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			c := Defaults()
			tt.mut(c)
			if err := c.Validate(); err == nil {
				t.Fatal("must fail validation")
			}
		})
	}
}

func TestSeverityFor(t *testing.T) {
	c := Defaults()
	if got := c.Canon.SeverityFor("B003"); got != "error" {
		t.Fatalf("B003 severity = %q, want error", got)
	}
	if got := c.Canon.SeverityFor("B005"); got != "info" {
		t.Fatalf("B005 severity = %q, want info", got)
	}
	if got := c.Canon.SeverityFor("E101"); got != "warn" {
		t.Fatalf("E101 severity = %q, want warn", got)
	}
	if got := c.Canon.SeverityFor("D001"); got != "info" {
		t.Fatalf("D001 severity = %q, want info", got)
	}
	c.Canon.Triangulate.Severity["B001.orphan"] = "error"
	if got := c.Canon.SeverityFor("B001.orphan"); got != "error" {
		t.Fatalf("exact suffix key must win: %q", got)
	}
	if got := c.Canon.SeverityFor("B001.other"); got != "warn" {
		t.Fatalf("unknown suffix must fall back to base check: %q", got)
	}
	if got := c.Canon.SeverityFor("E999"); got != "info" {
		t.Fatalf("unknown check must fall back to info: %q", got)
	}
	if err := c.Validate(); err != nil {
		t.Fatalf("dotted severity keys must validate: %v", err)
	}
}
