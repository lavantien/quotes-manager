package pipeline

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/lavantien/quotes-manager/internal/canon/source/editions"
	"github.com/lavantien/quotes-manager/internal/config"
)

type HtmlEdition struct {
	Author     string `json:"author"`
	Collection string `json:"collection"`
	FileName   string `json:"file_name"`
	BuildDate  string `json:"build_date"`
}

type DpdProvenance struct {
	Tag     string `json:"tag"`
	SHA256  string `json:"sha256"`
	Size    int64  `json:"size"`
	Fetched string `json:"fetched"`
}

type Manifest struct {
	Updated string `json:"updated"`
	Bilara  struct {
		Commit string `json:"commit"`
		Date   string `json:"date"`
	} `json:"bilara"`
	Editions struct {
		Commit string `json:"commit"`
	} `json:"editions"`
	Dpd   DpdProvenance `json:"dpd"`
	Htmls []HtmlEdition `json:"htmls"`
}

func htmlEditionFromInfo(h editions.HTMLInfo) HtmlEdition {
	return HtmlEdition{
		Author:     h.Author,
		Collection: h.Collection,
		FileName:   h.FileName,
		BuildDate:  h.BuildDate,
	}
}

func sortHtmls(htmls []HtmlEdition) {
	sort.Slice(htmls, func(i, j int) bool {
		a, b := htmls[i], htmls[j]
		switch {
		case a.Author != b.Author:
			return a.Author < b.Author
		case a.Collection != b.Collection:
			return a.Collection < b.Collection
		default:
			return a.FileName < b.FileName
		}
	})
}

func WriteManifest(cfg *config.Config, m Manifest) error {
	sortHtmls(m.Htmls)
	if m.Htmls == nil {
		m.Htmls = []HtmlEdition{}
	}
	raw, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(cfg.Canon.Paths.ManifestsDir, 0o755); err != nil {
		return fmt.Errorf("manifest: %w", err)
	}
	return os.WriteFile(cfg.Canon.ManifestFile(), append(raw, '\n'), 0o644)
}

func ReadManifest(cfg *config.Config) (Manifest, error) {
	raw, err := os.ReadFile(cfg.Canon.ManifestFile())
	if err != nil {
		return Manifest{}, fmt.Errorf("read manifest: %w", err)
	}
	var m Manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return Manifest{}, fmt.Errorf("decode manifest %s: %w", cfg.Canon.ManifestFile(), err)
	}
	return m, nil
}

func refreshManifestHtmls(cfg *config.Config, htmls []editions.HTMLInfo) error {
	if _, err := os.Stat(cfg.Canon.ManifestFile()); err != nil {
		return nil
	}
	m, err := ReadManifest(cfg)
	if err != nil {
		return err
	}
	m.Htmls = make([]HtmlEdition, len(htmls))
	for i, h := range htmls {
		m.Htmls[i] = htmlEditionFromInfo(h)
	}
	return WriteManifest(cfg, m)
}
