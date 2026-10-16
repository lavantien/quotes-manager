package pipeline

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/lavantien/quotes-manager/internal/canon/model"
	"github.com/lavantien/quotes-manager/internal/canon/source/bilara"
	"github.com/lavantien/quotes-manager/internal/canon/source/editions"
	"github.com/lavantien/quotes-manager/internal/config"
)

type ParseStats struct {
	Files        int  `json:"files"`
	Records      int  `json:"records"`
	Names        int  `json:"names"`
	HtmlSegments int  `json:"html_segments"`
	DpdHeadwords int  `json:"dpd_headwords"`
	DpdRoots     int  `json:"dpd_roots"`
	DpdForms     int  `json:"dpd_forms"`
	DpdAbsent    bool `json:"dpd_absent"`
}

const htmlSegmentsFile = "html-editions.ndjson"

type segLine struct {
	model.SegmentRecord
	Author string `json:"author,omitempty"`
}

func nameUID(key string) string {
	rest := key
	if i := strings.IndexByte(rest, ':'); i >= 0 {
		rest = rest[i+1:]
	}
	dot := strings.IndexByte(rest, '.')
	if dot > 0 && isDigits(rest[:dot]) {
		return rest[dot+1:]
	}
	return rest
}

func isDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return len(s) > 0
}

func muidFilter(cfg *config.Config) func(string) bool {
	if len(cfg.Canon.E2E.Muids) == 0 {
		return func(string) bool { return true }
	}
	keep := map[string]bool{}
	for _, m := range cfg.Canon.E2E.Muids {
		keep[m] = true
	}
	return func(m string) bool { return keep[m] }
}

type ndjson struct {
	f      *os.File
	w      *bufio.Writer
	closed bool
}

func newNDJSON(dir, name string) (*ndjson, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	f, err := os.Create(filepath.Join(dir, name))
	if err != nil {
		return nil, err
	}
	return &ndjson{f: f, w: bufio.NewWriterSize(f, 1<<20)}, nil
}

func (n *ndjson) write(v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if _, err := n.w.Write(b); err != nil {
		return err
	}
	return n.w.WriteByte('\n')
}

func (n *ndjson) close() error {
	if n.closed {
		return nil
	}
	n.closed = true
	if err := n.w.Flush(); err != nil {
		_ = n.f.Close()
		return err
	}
	return n.f.Close()
}

type parsedFileMeta struct {
	DupKeys          []string `json:"dup_keys"`
	NFCChanged       bool     `json:"nfc_changed"`
	EmptyValues      int      `json:"empty_values"`
	Muid             string   `json:"muid"`
	Stream           string   `json:"stream"`
	RecordMuids      []string `json:"record_muids"`
	PrefixMismatches []string `json:"prefix_mismatches"`
}

type parsedMeta struct {
	Files map[string]parsedFileMeta `json:"files"`
}

func Parse(ctx context.Context, cfg *config.Config) (ParseStats, error) {
	select {
	case <-ctx.Done():
		return ParseStats{}, ctx.Err()
	default:
	}
	var stats ParseStats

	if err := os.RemoveAll(cfg.Canon.ParsedDir()); err != nil {
		return ParseStats{}, fmt.Errorf("parse: reset parsed dir: %w", err)
	}

	n, err := parseBilara(cfg, muidFilter(cfg))
	if err != nil {
		return ParseStats{}, err
	}
	stats.Files, stats.Records, stats.Names = n.Files, n.Records, n.Names

	htmls, err := parseEditions(cfg)
	if err != nil {
		return ParseStats{}, err
	}

	htmlSegs, err := parseHTMLEditions(cfg, htmls)
	if err != nil {
		return ParseStats{}, err
	}
	stats.HtmlSegments = htmlSegs

	dpdStats, err := parseDpd(cfg)
	if err != nil {
		return ParseStats{}, err
	}
	stats.DpdHeadwords, stats.DpdRoots, stats.DpdForms, stats.DpdAbsent =
		dpdStats.DpdHeadwords, dpdStats.DpdRoots, dpdStats.DpdForms, dpdStats.DpdAbsent

	if err := refreshManifestHtmls(cfg, htmls); err != nil {
		return ParseStats{}, err
	}
	return stats, nil
}

func parseBilara(cfg *config.Config, keep func(string) bool) (ParseStats, error) {
	var stats ParseStats
	root := cfg.Canon.BilaraDir()

	nameFiles := map[string]bool{}
	if err := bilara.WalkNames(root, func(p string) error {
		nameFiles[p] = true
		return nil
	}); err != nil {
		return ParseStats{}, fmt.Errorf("parse bilara names walk: %w", err)
	}

	streams := map[model.Stream]*ndjson{}
	var closers []*ndjson
	closeAll := func() error {
		var first error
		for _, c := range closers {
			if err := c.close(); err != nil && first == nil {
				first = err
			}
		}
		return first
	}
	for _, s := range []model.Stream{model.StreamRoot, model.StreamTranslation, model.StreamHTML, model.StreamReference} {
		nj, err := newNDJSON(cfg.Canon.ParsedDir(), string(s)+".ndjson")
		if err != nil {
			return ParseStats{}, fmt.Errorf("parse bilara: %w", err)
		}
		closers = append(closers, nj)
		streams[s] = nj
	}
	names, err := newNDJSON(cfg.Canon.ParsedDir(), "names.ndjson")
	if err != nil {
		return ParseStats{}, fmt.Errorf("parse bilara: %w", err)
	}
	closers = append(closers, names)
	defer func() { _ = closeAll() }()

	meta := parsedMeta{Files: map[string]parsedFileMeta{}}
	err = bilara.Walk(root, func(f bilara.File) error {
		if nameFiles[f.Path] || !keep(f.Muid) {
			return nil
		}
		rel, err := relSlash(root, f.Path)
		if err != nil {
			return err
		}
		meta.Files[rel] = parsedFileMeta{DupKeys: []string{}, Muid: f.Muid, Stream: string(f.Stream)}
		fm := meta.Files[rel]

		r, err := os.Open(f.Path)
		if err != nil {
			return err
		}
		res, perr := bilara.ParseSegments(r, f.Stream, f.Muid)
		cerr := r.Close()
		if perr != nil {
			return perr
		}
		if cerr != nil {
			return cerr
		}
		author := authorFromPath(rel)
		allowed := bilara.AllowedMuids(f.Muid, cfg.Canon.Bilara.MaxRangeExpansion)
		flagged := make(map[string]bool, len(res.PrefixMismatches))
		for _, k := range res.PrefixMismatches {
			flagged[k] = true
		}
		for _, rec := range res.Records {
			colonless := rec.Muid == f.Muid && flagged[rec.SegKey]
			if rec.Muid != f.Muid && !allowed[rec.Muid] || colonless {
				key := rec.Muid + ":" + rec.SegKey
				if colonless {
					key = rec.SegKey
				}
				fm.PrefixMismatches = append(fm.PrefixMismatches, key)
				continue
			}
			if rec.Text == "" {
				fm.EmptyValues++
			}
			stats.Records++
			if !slices.Contains(fm.RecordMuids, rec.Muid) {
				fm.RecordMuids = append(fm.RecordMuids, rec.Muid)
			}
			if err := streams[f.Stream].write(segLine{SegmentRecord: rec, Author: author}); err != nil {
				return err
			}
		}
		fm.DupKeys = res.DupKeys
		if fm.DupKeys == nil {
			fm.DupKeys = []string{}
		}
		if fm.RecordMuids == nil {
			fm.RecordMuids = []string{}
		}
		if fm.PrefixMismatches == nil {
			fm.PrefixMismatches = []string{}
		}
		fm.NFCChanged = res.NFCChanged
		meta.Files[rel] = fm
		stats.Files++
		return nil
	})
	if err != nil {
		return ParseStats{}, fmt.Errorf("parse bilara: %w", err)
	}
	if stats.Files == 0 {
		return ParseStats{}, fmt.Errorf("parse bilara: no segment files found under %s", root)
	}

	seenUID := map[string]bool{}
	err = bilara.WalkNames(root, func(p string) error {
		r, err := os.Open(p)
		if err != nil {
			return err
		}
		nf, perr := bilara.ParseNames(r)
		cerr := r.Close()
		if perr != nil {
			return perr
		}
		if cerr != nil {
			return cerr
		}
		uids := make([]string, 0, len(nf))
		for k := range nf {
			uids = append(uids, k)
		}
		sortStrings(uids)
		for _, k := range uids {
			uid := nameUID(k)
			if seenUID[uid] {
				continue
			}
			seenUID[uid] = true
			stats.Names++
			if err := names.write(model.NameRecord{Muid: uid, Lang: cfg.Canon.Bilara.TransLang, Name: nf[k]}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return ParseStats{}, fmt.Errorf("parse bilara names: %w", err)
	}

	if err := closeAll(); err != nil {
		return ParseStats{}, fmt.Errorf("parse bilara: %w", err)
	}

	raw, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return ParseStats{}, err
	}
	if err := os.WriteFile(filepath.Join(cfg.Canon.ParsedDir(), "meta.json"), append(raw, '\n'), 0o644); err != nil {
		return ParseStats{}, fmt.Errorf("parse bilara meta: %w", err)
	}
	return stats, nil
}

func authorFromPath(relSlashPath string) string {
	segs := strings.Split(relSlashPath, "/")
	if len(segs) >= 3 && segs[0] == string(model.StreamTranslation) {
		return segs[2]
	}
	return ""
}

func parseEditions(cfg *config.Config) ([]editions.HTMLInfo, error) {
	out := filepath.Join(cfg.Canon.Paths.ManifestsDir, "editions-discovery.md")
	if _, err := os.Stat(filepath.Join(cfg.Canon.EditionsDir(), "en")); err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
		return nil, editions.DiscoveryReport(nil, out)
	}
	htmls, err := editions.FindHTML(cfg.Canon.EditionsDir())
	if err != nil {
		return nil, fmt.Errorf("parse editions html: %w", err)
	}
	if err := editions.DiscoveryReport(htmls, out); err != nil {
		return nil, fmt.Errorf("parse editions discovery: %w", err)
	}
	return htmls, nil
}

func parseHTMLEditions(cfg *config.Config, htmls []editions.HTMLInfo) (int, error) {
	if len(htmls) == 0 {
		return 0, nil
	}
	out, err := newNDJSON(cfg.Canon.ParsedDir(), htmlSegmentsFile)
	if err != nil {
		return 0, fmt.Errorf("parse editions html: %w", err)
	}
	n := 0
	for _, h := range htmls {
		f, err := os.Open(h.Path)
		if err != nil {
			_ = out.close()
			return 0, err
		}
		perr := editions.ParseHTMLSegments(f, func(muid, segkey, text string) error {
			n++
			return out.write(model.HTMLSegment{Author: h.Author, Muid: muid, SegKey: segkey, Text: text})
		})
		cerr := f.Close()
		if perr != nil {
			_ = out.close()
			return 0, fmt.Errorf("parse editions html %s: %w", h.Path, perr)
		}
		if cerr != nil {
			_ = out.close()
			return 0, cerr
		}
	}
	if err := out.close(); err != nil {
		return 0, fmt.Errorf("parse editions html: %w", err)
	}
	return n, nil
}

func relSlash(root, p string) (string, error) {
	rel, err := filepath.Rel(root, p)
	if err != nil {
		return "", err
	}
	return filepath.ToSlash(rel), nil
}

func sortStrings(s []string) { sort.Strings(s) }
