package config

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

const Version = "2.0.0"

const SchemaVersion = 2

const EnvPrefix = "QM"

const MaxGraphNodes = 5000

type Repo struct {
	URL    string `json:"url"`
	Branch string `json:"branch"`
	Dir    string `json:"dir"`
}

type Quotes struct {
	DBPath           string `json:"db_path"`
	DumpsDir         string `json:"dumps_dir"`
	SeedFile         string `json:"seed_file"`
	ExportFile       string `json:"export_file"`
	PicturesDir      string `json:"pictures_dir"`
	TablesCollection string `json:"tables_collection"`
}

type Server struct {
	Addr                   string `json:"addr"`
	DefaultLimit           int    `json:"default_limit"`
	MaxLimit               int    `json:"max_limit"`
	GraphMaxNodes          int    `json:"graph_max_nodes"`
	GraphMaxEdges          int    `json:"graph_max_edges"`
	JobLogLines            int    `json:"job_log_lines"`
	SQLConsole             bool   `json:"sql_console"`
	AnalyticsThreads       int    `json:"analytics_threads"`
	AnalyticsMemoryGB      int    `json:"analytics_memory_gb"`
	ReadTimeoutSeconds     int    `json:"read_timeout_seconds"`
	WriteTimeoutSeconds    int    `json:"write_timeout_seconds"`
	IdleTimeoutSeconds     int    `json:"idle_timeout_seconds"`
	ShutdownTimeoutSeconds int    `json:"shutdown_timeout_seconds"`
}

type Canon struct {
	Repos struct {
		Bilara   Repo `json:"bilara"`
		Editions Repo `json:"editions"`
	} `json:"repos"`

	Bilara struct {
		RootLang          string   `json:"root_lang"`
		RootEdition       string   `json:"root_edition"`
		TransLang         string   `json:"trans_lang"`
		Authors           []string `json:"authors"`
		SparseDirs        []string `json:"sparse_dirs"`
		KeepRegexes       []string `json:"keep_regexes"`
		MaxRangeExpansion int      `json:"max_range_expansion"`
	} `json:"bilara"`

	Editions struct {
		Authors     map[string][]string `json:"authors"`
		SparseDirs  []string            `json:"sparse_dirs"`
		KeepRegexes []string            `json:"keep_regexes"`
	} `json:"editions"`

	Dpd struct {
		URL                string `json:"url"`
		SHA256             string `json:"sha256"`
		MaxTarballBytes    int64  `json:"max_tarball_bytes"`
		MaxDBBytes         int64  `json:"max_db_bytes"`
		TimeoutSeconds     int    `json:"timeout_seconds"`
		MinTokenLength     int    `json:"min_token_length"`
		UnknownWarnPerMuid int    `json:"unknown_warn_per_muid"`
	} `json:"dpd"`

	Paths struct {
		DataDir      string `json:"data_dir"`
		E2EDir       string `json:"e2e_dir"`
		DistDir      string `json:"dist_dir"`
		DBDir        string `json:"db_dir"`
		ManifestsDir string `json:"manifests_dir"`
		ReportsDir   string `json:"reports_dir"`
	} `json:"paths"`

	Load struct {
		BatchSize int `json:"batch_size"`
	} `json:"load"`

	Triangulate struct {
		Severity         map[string]string `json:"severity"`
		EmptyWarnPerMuid int               `json:"empty_warn_per_muid"`
		SimilarityMin    float64           `json:"similarity_min"`
		DetailSampleMax  int               `json:"detail_sample_max"`
	} `json:"triangulate"`

	E2E struct {
		Muids []string `json:"muids"`
	} `json:"e2e"`

	Export struct {
		Parquet bool `json:"parquet"`
	} `json:"export"`
}

type Config struct {
	Quotes Quotes `json:"quotes"`
	Server Server `json:"server"`
	Canon  Canon  `json:"canon"`
}

var knownChecks = map[string]bool{
	"B001": true, "B002": true, "B003": true, "B004": true, "B005": true, "B006": true, "B007": true,
	"D001": true, "D002": true,
	"E101": true, "E102": true, "E103": true, "E104": true,
}

var severities = map[string]bool{"info": true, "warn": true, "error": true}

func Defaults() *Config {
	c := &Config{}
	c.Quotes = Quotes{
		DBPath:           "database/quotes.db",
		DumpsDir:         "dumps",
		SeedFile:         "database/seed.sql",
		ExportFile:       "exports/shortest-first.md",
		PicturesDir:      "project-pictures",
		TablesCollection: "AriyaSavaka",
	}

	c.Server.Addr = "127.0.0.1:8080"
	c.Server.DefaultLimit = 50
	c.Server.MaxLimit = 500
	c.Server.GraphMaxNodes = 400
	c.Server.GraphMaxEdges = 1600
	c.Server.JobLogLines = 1000
	c.Server.SQLConsole = false
	c.Server.AnalyticsThreads = 2
	c.Server.AnalyticsMemoryGB = 2
	c.Server.ReadTimeoutSeconds = 30
	c.Server.WriteTimeoutSeconds = 120
	c.Server.IdleTimeoutSeconds = 120
	c.Server.ShutdownTimeoutSeconds = 15

	c.Canon.Repos.Bilara = Repo{
		URL:    "https://github.com/suttacentral/bilara-data",
		Branch: "published",
		Dir:    "bilara-data",
	}
	c.Canon.Repos.Editions = Repo{
		URL:    "https://github.com/suttacentral/editions",
		Branch: "main",
		Dir:    "editions",
	}

	c.Canon.Bilara.RootLang = "pli"
	c.Canon.Bilara.RootEdition = "ms"
	c.Canon.Bilara.TransLang = "en"
	c.Canon.Bilara.Authors = []string{"sujato", "brahmali"}
	c.Canon.Bilara.MaxRangeExpansion = 500
	c.Canon.Bilara.SparseDirs = []string{
		"root/pli/ms/sutta",
		"root/pli/ms/vinaya",
		"translation/en/sujato",
		"translation/en/brahmali",
		"html/pli/ms/sutta",
		"html/pli/ms/vinaya",
		"reference/pli/ms/sutta",
		"reference/pli/ms/vinaya",
		"_publication/en",
	}
	c.Canon.Bilara.KeepRegexes = []string{
		`^root/pli/ms/(sutta|vinaya)/`,
		`^translation/en/(sujato|brahmali)/`,
		`^html/pli/ms/(sutta|vinaya)/`,
		`^reference/pli/ms/(sutta|vinaya)/`,
		`^_publication/en/`,
		`^_[a-z0-9-]+\.json$`,
		`^(README\.md|LICENSE|LICENSE\.md|CONTRIBUTING\.md)$`,
	}

	c.Canon.Editions.Authors = map[string][]string{
		"sujato":   {"an", "dhp", "dn", "iti", "mn", "sn", "snp", "thag", "thig", "ud"},
		"brahmali": {"pli-tv-vi"},
	}
	c.Canon.Editions.SparseDirs = []string{"en/sujato", "en/brahmali"}
	c.Canon.Editions.KeepRegexes = []string{
		`^en/(sujato|brahmali)/[^/]+/html/[^/]+\.html$`,
		`^last_run_date$`,
		`^(README\.md|LICENSE)$`,
	}

	c.Canon.Dpd.URL = "https://github.com/digitalpalidictionary/dpd-db/releases/latest/download/dpd.db.tar.xz"
	c.Canon.Dpd.MaxTarballBytes = 1 << 30
	c.Canon.Dpd.MaxDBBytes = 8 << 30
	c.Canon.Dpd.TimeoutSeconds = 1800
	c.Canon.Dpd.MinTokenLength = 2
	c.Canon.Dpd.UnknownWarnPerMuid = 20

	c.Canon.Paths.DataDir = "data"
	c.Canon.Paths.E2EDir = "data/e2e-live"
	c.Canon.Paths.DistDir = "dist"
	c.Canon.Paths.DBDir = "database/canon"
	c.Canon.Paths.ManifestsDir = "docs/canon/manifests"
	c.Canon.Paths.ReportsDir = "docs/canon/reports"

	c.Canon.Load.BatchSize = 50_000

	c.Canon.Triangulate.Severity = map[string]string{
		"B001": "warn",
		"B002": "warn",
		"B003": "error",
		"B004": "error",
		"B005": "info",
		"B006": "info",
		"B007": "warn",
		"D001": "info",
		"D002": "info",
		"E101": "warn",
		"E102": "warn",
		"E103": "warn",
		"E104": "info",
	}
	c.Canon.Triangulate.EmptyWarnPerMuid = 20
	c.Canon.Triangulate.SimilarityMin = 0.85
	c.Canon.Triangulate.DetailSampleMax = 200

	c.Canon.E2E.Muids = nil
	c.Canon.Export.Parquet = true
	return c
}

func Load(path string) (*Config, error) {
	c := Defaults()
	if path != "" {
		b, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read config: %w", err)
		}
		dec := json.NewDecoder(strings.NewReader(string(b)))
		dec.DisallowUnknownFields()
		if err := dec.Decode(c); err != nil {
			return nil, fmt.Errorf("decode config %s: %w", path, err)
		}
	}
	if err := c.applyEnv(); err != nil {
		return nil, err
	}
	if err := c.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}
	return c, nil
}

func (c *Config) applyEnv() error {
	set := func(name string, dst *string) {
		if v := os.Getenv(EnvPrefix + "_" + name); v != "" {
			*dst = v
		}
	}
	set("QUOTES_DB_PATH", &c.Quotes.DBPath)
	set("QUOTES_DUMPS_DIR", &c.Quotes.DumpsDir)
	set("QUOTES_SEED_FILE", &c.Quotes.SeedFile)
	set("QUOTES_EXPORT_FILE", &c.Quotes.ExportFile)
	set("QUOTES_PICTURES_DIR", &c.Quotes.PicturesDir)
	set("QUOTES_TABLES_COLLECTION", &c.Quotes.TablesCollection)

	set("SERVER_ADDR", &c.Server.Addr)

	set("CANON_DATA_DIR", &c.Canon.Paths.DataDir)
	set("CANON_E2E_DIR", &c.Canon.Paths.E2EDir)
	set("CANON_DIST_DIR", &c.Canon.Paths.DistDir)
	set("CANON_DB_DIR", &c.Canon.Paths.DBDir)
	set("CANON_MANIFESTS_DIR", &c.Canon.Paths.ManifestsDir)
	set("CANON_REPORTS_DIR", &c.Canon.Paths.ReportsDir)

	set("CANON_DPD_URL", &c.Canon.Dpd.URL)
	set("CANON_DPD_SHA256", &c.Canon.Dpd.SHA256)

	intEnv := func(name string, dst *int) error {
		v := os.Getenv(EnvPrefix + "_" + name)
		if v == "" {
			return nil
		}
		n, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("bad %s_%s: %w", EnvPrefix, name, err)
		}
		*dst = n
		return nil
	}
	for name, dst := range map[string]*int{
		"SERVER_READ_TIMEOUT_SECONDS":      &c.Server.ReadTimeoutSeconds,
		"SERVER_WRITE_TIMEOUT_SECONDS":     &c.Server.WriteTimeoutSeconds,
		"SERVER_IDLE_TIMEOUT_SECONDS":      &c.Server.IdleTimeoutSeconds,
		"SERVER_SHUTDOWN_TIMEOUT_SECONDS":  &c.Server.ShutdownTimeoutSeconds,
		"CANON_SERVER_DEFAULT_LIMIT":       &c.Server.DefaultLimit,
		"CANON_SERVER_MAX_LIMIT":           &c.Server.MaxLimit,
		"CANON_SERVER_GRAPH_MAX_NODES":     &c.Server.GraphMaxNodes,
		"CANON_SERVER_GRAPH_MAX_EDGES":     &c.Server.GraphMaxEdges,
		"CANON_SERVER_JOB_LOG_LINES":       &c.Server.JobLogLines,
		"CANON_SERVER_ANALYTICS_THREADS":   &c.Server.AnalyticsThreads,
		"CANON_SERVER_ANALYTICS_MEMORY_GB": &c.Server.AnalyticsMemoryGB,
	} {
		if err := intEnv(name, dst); err != nil {
			return err
		}
	}
	boolEnv := func(name string, dst *bool) error {
		v := os.Getenv(EnvPrefix + "_" + name)
		if v == "" {
			return nil
		}
		b, err := strconv.ParseBool(v)
		if err != nil {
			return fmt.Errorf("bad %s_%s: %w", EnvPrefix, name, err)
		}
		*dst = b
		return nil
	}
	if err := boolEnv("CANON_SERVER_SQL_CONSOLE", &c.Server.SQLConsole); err != nil {
		return err
	}
	if err := boolEnv("CANON_EXPORT_PARQUET", &c.Canon.Export.Parquet); err != nil {
		return err
	}

	if err := intEnv("CANON_BATCH_SIZE", &c.Canon.Load.BatchSize); err != nil {
		return err
	}
	if v := os.Getenv(EnvPrefix + "_CANON_SIMILARITY_MIN"); v != "" {
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return fmt.Errorf("bad %s_CANON_SIMILARITY_MIN: %w", EnvPrefix, err)
		}
		c.Canon.Triangulate.SimilarityMin = f
	}
	if err := intEnv("CANON_EMPTY_WARN_PER_MUID", &c.Canon.Triangulate.EmptyWarnPerMuid); err != nil {
		return err
	}
	if v := os.Getenv(EnvPrefix + "_CANON_E2E_MUIDS"); v != "" {
		c.Canon.E2E.Muids = strings.Split(v, ",")
	}
	return nil
}

func (c *Config) Validate() error {
	if slices.Contains([]string{
		c.Quotes.DBPath, c.Quotes.DumpsDir, c.Quotes.SeedFile, c.Quotes.ExportFile, c.Quotes.PicturesDir,
		c.Quotes.TablesCollection,
	}, "") {
		return fmt.Errorf("quotes: all paths and tables_collection required")
	}
	for name, r := range map[string]Repo{"bilara": c.Canon.Repos.Bilara, "editions": c.Canon.Repos.Editions} {
		if r.URL == "" || r.Branch == "" || r.Dir == "" {
			return fmt.Errorf("repo %s: url, branch, dir all required", name)
		}
	}
	if len(c.Canon.Bilara.SparseDirs) == 0 || len(c.Canon.Bilara.KeepRegexes) == 0 {
		return fmt.Errorf("bilara: sparse_dirs and keep_regexes required")
	}
	if c.Canon.Bilara.RootLang == "" || c.Canon.Bilara.RootEdition == "" || c.Canon.Bilara.TransLang == "" || len(c.Canon.Bilara.Authors) == 0 {
		return fmt.Errorf("bilara: root_lang, root_edition, trans_lang, authors required")
	}
	if c.Canon.Bilara.MaxRangeExpansion <= 0 {
		return fmt.Errorf("bilara: max_range_expansion must be > 0")
	}
	if len(c.Canon.Editions.Authors) == 0 || len(c.Canon.Editions.SparseDirs) == 0 || len(c.Canon.Editions.KeepRegexes) == 0 {
		return fmt.Errorf("editions: authors, sparse_dirs, keep_regexes required")
	}
	u, err := url.Parse(c.Canon.Dpd.URL)
	if err != nil || u.Scheme != "http" && u.Scheme != "https" && u.Scheme != "file" {
		return fmt.Errorf("dpd: url must be http(s) or file: %q", c.Canon.Dpd.URL)
	}
	if c.Canon.Dpd.MaxTarballBytes <= 0 || c.Canon.Dpd.MaxDBBytes <= 0 || c.Canon.Dpd.TimeoutSeconds <= 0 {
		return fmt.Errorf("dpd: max_tarball_bytes, max_db_bytes, timeout_seconds must be > 0")
	}
	if c.Canon.Dpd.MinTokenLength < 1 || c.Canon.Dpd.UnknownWarnPerMuid < 0 {
		return fmt.Errorf("dpd: min_token_length must be >= 1 and unknown_warn_per_muid >= 0")
	}
	if c.Canon.Dpd.SHA256 != "" {
		if len(c.Canon.Dpd.SHA256) != 64 {
			return fmt.Errorf("dpd: sha256 must be 64 hex characters")
		}
		if _, err := hex.DecodeString(c.Canon.Dpd.SHA256); err != nil {
			return fmt.Errorf("dpd: sha256 must be 64 hex characters: %w", err)
		}
	}
	if c.Canon.Load.BatchSize <= 0 {
		return fmt.Errorf("load.batch_size must be > 0")
	}
	if c.Canon.Triangulate.SimilarityMin <= 0 || c.Canon.Triangulate.SimilarityMin > 1 {
		return fmt.Errorf("triangulate.similarity_min must be in (0,1]")
	}
	if c.Canon.Triangulate.EmptyWarnPerMuid < 0 || c.Canon.Triangulate.DetailSampleMax <= 0 {
		return fmt.Errorf("triangulate thresholds out of range")
	}
	for id, sev := range c.Canon.Triangulate.Severity {
		base, _, _ := strings.Cut(id, ".")
		if !knownChecks[base] {
			return fmt.Errorf("triangulate.severity: unknown check %q", id)
		}
		if !severities[sev] {
			return fmt.Errorf("triangulate.severity[%s]: bad severity %q", id, sev)
		}
	}
	if slices.Contains([]string{
		c.Canon.Paths.DataDir, c.Canon.Paths.E2EDir, c.Canon.Paths.DistDir,
		c.Canon.Paths.DBDir, c.Canon.Paths.ManifestsDir, c.Canon.Paths.ReportsDir,
	}, "") {
		return fmt.Errorf("paths: all directories required")
	}
	if _, _, err := net.SplitHostPort(c.Server.Addr); err != nil {
		return fmt.Errorf("server: addr must be host:port: %w", err)
	}
	if c.Server.DefaultLimit < 1 || c.Server.DefaultLimit > c.Server.MaxLimit {
		return fmt.Errorf("server: default_limit must be in [1,max_limit]")
	}
	if c.Server.MaxLimit < 1 || c.Server.MaxLimit > 10000 {
		return fmt.Errorf("server: max_limit must be in [1,10000]")
	}
	if c.Server.GraphMaxNodes <= 0 || c.Server.GraphMaxEdges <= 0 {
		return fmt.Errorf("server: graph_max_nodes and graph_max_edges must be > 0")
	}
	if c.Server.GraphMaxNodes > MaxGraphNodes {
		return fmt.Errorf("server: graph_max_nodes must be in [1,%d]", MaxGraphNodes)
	}
	if c.Server.JobLogLines <= 0 {
		return fmt.Errorf("server: job_log_lines must be > 0")
	}
	if c.Server.AnalyticsThreads < 1 || c.Server.AnalyticsThreads > 64 {
		return fmt.Errorf("server: analytics_threads must be in [1,64]")
	}
	if c.Server.AnalyticsMemoryGB < 1 || c.Server.AnalyticsMemoryGB > 64 {
		return fmt.Errorf("server: analytics_memory_gb must be in [1,64]")
	}
	for name, v := range map[string]int{
		"read_timeout_seconds":     c.Server.ReadTimeoutSeconds,
		"write_timeout_seconds":    c.Server.WriteTimeoutSeconds,
		"idle_timeout_seconds":     c.Server.IdleTimeoutSeconds,
		"shutdown_timeout_seconds": c.Server.ShutdownTimeoutSeconds,
	} {
		if v <= 0 {
			return fmt.Errorf("server: %s must be > 0", name)
		}
	}
	if c.Server.ShutdownTimeoutSeconds > 300 {
		return fmt.Errorf("server: shutdown_timeout_seconds must be <= 300")
	}
	return nil
}

func (cc *Canon) SeverityFor(name string) string {
	if v, ok := cc.Triangulate.Severity[name]; ok {
		return v
	}
	if base, _, found := strings.Cut(name, "."); found {
		if v, ok := cc.Triangulate.Severity[base]; ok {
			return v
		}
	}
	return "info"
}

func (cc *Canon) MirrorsDir() string  { return filepath.Join(cc.Paths.DataDir, "repos") }
func (cc *Canon) BilaraDir() string   { return filepath.Join(cc.MirrorsDir(), cc.Repos.Bilara.Dir) }
func (cc *Canon) EditionsDir() string { return filepath.Join(cc.MirrorsDir(), cc.Repos.Editions.Dir) }
func (cc *Canon) ParsedDir() string   { return filepath.Join(cc.Paths.DataDir, "parsed") }
func (cc *Canon) HtmlSegmentsFile() string {
	return filepath.Join(cc.ParsedDir(), "html-editions.ndjson")
}

func (cc *Canon) DpdDir() string            { return filepath.Join(cc.Paths.DataDir, "dpd") }
func (cc *Canon) DpdTarball() string        { return filepath.Join(cc.DpdDir(), "dpd.db.tar.xz") }
func (cc *Canon) DpdDB() string             { return filepath.Join(cc.DpdDir(), "dpd.db") }
func (cc *Canon) DpdProvenanceFile() string { return filepath.Join(cc.DpdDir(), "provenance.json") }
func (cc *Canon) DpdHeadwordsFile() string {
	return filepath.Join(cc.ParsedDir(), "dpd-headwords.ndjson")
}
func (cc *Canon) DpdRootsFile() string { return filepath.Join(cc.ParsedDir(), "dpd-roots.ndjson") }
func (cc *Canon) DpdFormsFile() string { return filepath.Join(cc.ParsedDir(), "dpd-forms.ndjson") }

func (cc *Canon) DBFile(runID string) string {
	return filepath.Join(cc.Paths.DBDir, fmt.Sprintf("canon-%s.db", runID))
}
func (cc *Canon) DBPointerFile() string { return filepath.Join(cc.Paths.DBDir, "pointer.json") }
func (cc *Canon) DuckDBFile() string    { return filepath.Join(cc.Paths.DataDir, "canon.duckdb") }
func (cc *Canon) ParquetDir() string {
	return filepath.Join(cc.Paths.DataDir, "export", "parquet")
}
func (cc *Canon) ManifestFile() string { return filepath.Join(cc.Paths.ManifestsDir, "sync.json") }
func (cc *Canon) ReportsDirFor(date string) string {
	return filepath.Join(cc.Paths.ReportsDir, date)
}
