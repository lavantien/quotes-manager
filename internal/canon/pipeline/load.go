package pipeline

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lavantien/quotes-manager/internal/canon/graph"
	"github.com/lavantien/quotes-manager/internal/canon/model"
	"github.com/lavantien/quotes-manager/internal/config"
)

type LoadResult struct {
	RunID  string `json:"run_id"`
	DBPath string `json:"db_path"`
}

const runIDLayout = "20060102T150405Z"

type sourcePayload struct {
	Repo      string `json:"repo"`
	Commit    string `json:"commit"`
	FetchedAt string `json:"fetched_at"`
	SHA256    string `json:"sha256,omitempty"`
}

type namePayload struct {
	Name string `json:"name,omitempty"`
}

type textPayload struct {
	Name     string `json:"name,omitempty"`
	FileMuid string `json:"file_muid"`
	FileOrd  int    `json:"file_ord"`
}

type translatesPayload struct {
	Author string `json:"author"`
}

func Load(ctx context.Context, cfg *config.Config) (LoadResult, error) {
	select {
	case <-ctx.Done():
		return LoadResult{}, ctx.Err()
	default:
	}
	meta, err := readParsedMeta(cfg)
	if err != nil {
		return LoadResult{}, err
	}
	man, manErr := ReadManifest(cfg)
	if manErr != nil {
		if editionsTreePresent(cfg) {
			return LoadResult{}, fmt.Errorf(
				"load: editions tree present but sync manifest unreadable (%v); run qm sync", manErr)
		}
		man = Manifest{}
	}

	runID := uniqueRunID(cfg, time.Now().UTC())
	dbPath := cfg.Canon.DBFile(runID)
	if err := os.MkdirAll(cfg.Canon.Paths.DBDir, 0o755); err != nil {
		return LoadResult{}, fmt.Errorf("load: %w", err)
	}
	store, err := graph.Open(dbPath)
	if err != nil {
		return LoadResult{}, fmt.Errorf("load: %w", err)
	}
	defer func() { _ = store.Close() }()
	if err := store.Migrate(); err != nil {
		return LoadResult{}, fmt.Errorf("load: %w", err)
	}
	loader := graph.NewLoader(store, cfg.Canon.Load.BatchSize)

	names, err := readNames(cfg)
	if err != nil {
		return LoadResult{}, err
	}
	textIDs, err := loadHierarchy(cfg, loader, meta, names, man)
	if err != nil {
		return LoadResult{}, fmt.Errorf("load hierarchy: %w", err)
	}
	hwIDs, err := loadDpd(cfg, loader, man)
	if err != nil {
		return LoadResult{}, fmt.Errorf("load dpd: %w", err)
	}

	paliIDs := map[string]int64{}
	if err := streamSegments(cfg, loader, textIDs, paliIDs); err != nil {
		return LoadResult{}, fmt.Errorf("load segments: %w", err)
	}
	if err := loader.Flush(); err != nil {
		return LoadResult{}, fmt.Errorf("load: %w", err)
	}
	if err := indexFTS(ctx, store, loader); err != nil {
		return LoadResult{}, fmt.Errorf("load fts: %w", err)
	}
	if err := loader.Flush(); err != nil {
		return LoadResult{}, fmt.Errorf("load: %w", err)
	}
	if err := linkWords(cfg, loader, paliIDs, hwIDs); err != nil {
		return LoadResult{}, fmt.Errorf("load words: %w", err)
	}
	if err := loader.Flush(); err != nil {
		return LoadResult{}, fmt.Errorf("load: %w", err)
	}

	runRow, err := store.NewRun(man.Bilara.Commit, man.Editions.Commit, man.Dpd.Tag)
	if err != nil {
		return LoadResult{}, fmt.Errorf("load: %w", err)
	}
	if err := store.FinishRun(runRow); err != nil {
		return LoadResult{}, fmt.Errorf("load: %w", err)
	}
	if err := store.Checkpoint(); err != nil {
		return LoadResult{}, fmt.Errorf("load: %w", err)
	}
	if err := store.Close(); err != nil {
		return LoadResult{}, fmt.Errorf("load: %w", err)
	}
	if err := graph.PublishDB(cfg, runID, dbPath); err != nil {
		if !errors.Is(err, graph.ErrRetirementLag) {
			return LoadResult{}, fmt.Errorf("load: %w", err)
		}
	}
	return LoadResult{RunID: runID, DBPath: dbPath}, nil
}

func uniqueRunID(cfg *config.Config, now time.Time) string {
	base := now.Format(runIDLayout)
	for n := 2; ; n++ {
		if _, err := os.Stat(cfg.Canon.DBFile(base)); os.IsNotExist(err) {
			return base
		}
		base = fmt.Sprintf("%s-%d", now.Format(runIDLayout), n)
	}
}

func editionsTreePresent(cfg *config.Config) bool {
	_, err := os.Stat(filepath.Join(cfg.Canon.EditionsDir(), "en"))
	return err == nil
}

func readParsedMeta(cfg *config.Config) (parsedMeta, error) {
	raw, err := os.ReadFile(filepath.Join(cfg.Canon.ParsedDir(), "meta.json"))
	if err != nil {
		return parsedMeta{}, fmt.Errorf("load: read meta: %w", err)
	}
	var m parsedMeta
	if err := json.Unmarshal(raw, &m); err != nil {
		return parsedMeta{}, fmt.Errorf("load: decode meta: %w", err)
	}
	return m, nil
}

func readNames(cfg *config.Config) (map[string]string, error) {
	names := map[string]string{}
	f, err := os.Open(filepath.Join(cfg.Canon.ParsedDir(), "names.ndjson"))
	if err != nil {
		if os.IsNotExist(err) {
			return names, nil
		}
		return nil, fmt.Errorf("load: %w", err)
	}
	defer func() { _ = f.Close() }()
	r := bufio.NewReader(f)
	for {
		line, err := r.ReadString('\n')
		if line != "" {
			var rec model.NameRecord
			if jerr := json.Unmarshal([]byte(strings.TrimRight(line, "\r\n")), &rec); jerr != nil {
				return nil, fmt.Errorf("load: bad name line: %w", jerr)
			}
			if rec.Muid != "" {
				names[rec.Muid] = rec.Name
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				return names, nil
			}
			return nil, fmt.Errorf("load: %w", err)
		}
	}
}

func deriveDivision(rel string, muid string) (collection, division string, err error) {
	segs := strings.Split(rel, "/")
	pi := -1
	for i, s := range segs {
		if s == "sutta" || s == "vinaya" {
			pi = i
			break
		}
	}
	if pi < 0 || pi+1 >= len(segs) {
		return "", "", fmt.Errorf("cannot derive division from %q", rel)
	}
	if segs[pi] == "sutta" {
		return "sutta", segs[pi+1], nil
	}
	unit := muid
	if pi+2 < len(segs) {
		unit = segs[pi+1]
	}
	return "vinaya", "vinaya/" + strings.TrimPrefix(unit, "pli-tv-"), nil
}

type textRef struct {
	Division string
	FileMuid string
	FileOrd  int
}

func textsOf(meta parsedMeta) (map[string]textRef, error) {
	out := map[string]textRef{}
	paths := make([]string, 0, len(meta.Files))
	for p := range meta.Files {
		paths = append(paths, p)
	}
	sortStrings(paths)
	for _, path := range paths {
		f := meta.Files[path]
		_, div, err := deriveDivision(path, f.Muid)
		if err != nil {
			return nil, err
		}
		muids := f.RecordMuids
		if len(muids) == 0 {
			muids = []string{f.Muid}
		}
		for i, m := range muids {
			if _, ok := out[m]; !ok || f.Stream == string(model.StreamRoot) {
				out[m] = textRef{Division: div, FileMuid: f.Muid, FileOrd: i + 1}
			}
		}
	}
	return out, nil
}

func loadHierarchy(cfg *config.Config, loader *graph.Loader, meta parsedMeta, names map[string]string, man Manifest) (map[string]int64, error) {
	_, err := loader.EnsureNode(model.NodeSource, "bilara",
		mustJSON(sourcePayload{Repo: cfg.Canon.Repos.Bilara.URL, Commit: man.Bilara.Commit, FetchedAt: man.Updated}))
	if err != nil {
		return nil, err
	}
	srcEditions, err := loader.EnsureNode(model.NodeSource, "editions",
		mustJSON(sourcePayload{Repo: cfg.Canon.Repos.Editions.URL, Commit: man.Editions.Commit, FetchedAt: man.Updated}))
	if err != nil {
		return nil, err
	}

	for _, e := range newestEditions(man.Htmls) {
		id, err := loader.EnsureNode(model.NodeEdition, e.Author+"/"+e.Collection, mustJSON(e))
		if err != nil {
			return nil, err
		}
		if err := loader.AddEdge(id, srcEditions, model.EdgePartOf, nil, ""); err != nil {
			return nil, err
		}
	}

	texts, err := textsOf(meta)
	if err != nil {
		return nil, err
	}
	muids := make([]string, 0, len(texts))
	for m := range texts {
		muids = append(muids, m)
	}
	sortStrings(muids)

	colIDs := map[string]int64{}
	divIDs := map[string]int64{}
	textIDs := map[string]int64{}
	for _, muid := range muids {
		ref := texts[muid]
		col := "sutta"
		if strings.HasPrefix(ref.Division, "vinaya/") {
			col = "vinaya"
		}
		if _, ok := colIDs[col]; !ok {
			id, err := loader.EnsureNode(model.NodeCollection, col, mustJSON(namePayload{Name: names[col]}))
			if err != nil {
				return nil, err
			}
			colIDs[col] = id
		}
		if _, ok := divIDs[ref.Division]; !ok {
			id, err := loader.EnsureNode(model.NodeDivision, ref.Division, mustJSON(namePayload{Name: names[ref.Division]}))
			if err != nil {
				return nil, err
			}
			if err := loader.AddEdge(id, colIDs[col], model.EdgeChildOf, nil, ""); err != nil {
				return nil, err
			}
			divIDs[ref.Division] = id
		}
		id, err := loader.EnsureNode(model.NodeText, muid, mustJSON(textPayload{
			Name: names[muid], FileMuid: ref.FileMuid, FileOrd: ref.FileOrd,
		}))
		if err != nil {
			return nil, err
		}
		textIDs[muid] = id
		if err := loader.AddEdge(id, divIDs[ref.Division], model.EdgeChildOf, nil, ""); err != nil {
			return nil, err
		}
	}
	return textIDs, nil
}

func newestEditions(htmls []HtmlEdition) []HtmlEdition {
	best := map[string]HtmlEdition{}
	keys := make([]string, 0, len(htmls))
	for _, h := range htmls {
		k := h.Author + "/" + h.Collection
		cur, ok := best[k]
		if !ok {
			keys = append(keys, k)
			best[k] = h
			continue
		}
		if h.BuildDate > cur.BuildDate || (h.BuildDate == cur.BuildDate && h.FileName > cur.FileName) {
			best[k] = h
		}
	}
	sortStrings(keys)
	out := make([]HtmlEdition, 0, len(keys))
	for _, k := range keys {
		out = append(out, best[k])
	}
	return out
}

func streamSegments(cfg *config.Config, loader *graph.Loader, textIDs map[string]int64, paliIDs map[string]int64) error {
	ords := map[string]int64{}

	pali := func(rec segLine) error {
		ord := ords[rec.Muid]
		ords[rec.Muid] = ord + 1
		key := "pli:" + rec.Muid + ":" + rec.SegKey
		id, err := loader.EnsureNode(model.NodeSegment, key, mustJSON(rec.SegmentRecord))
		if err != nil {
			return err
		}
		paliIDs[key] = id
		o := ord
		return loader.AddEdge(id, textIDs[rec.Muid], model.EdgeChildOf, &o, "")
	}
	translation := func(rec segLine) error {
		if rec.Author == "" {
			return fmt.Errorf("translation record %s:%s without author", rec.Muid, rec.SegKey)
		}
		k := rec.Author + "/" + rec.Muid
		ord := ords[k]
		ords[k] = ord + 1
		key := "en/" + rec.Author + ":" + rec.Muid + ":" + rec.SegKey
		id, err := loader.EnsureNode(model.NodeSegment, key, mustJSON(rec.SegmentRecord))
		if err != nil {
			return err
		}
		o := ord
		if err := loader.AddEdge(id, textIDs[rec.Muid], model.EdgeChildOf, &o, ""); err != nil {
			return err
		}
		if paliID, ok := paliIDs["pli:"+rec.Muid+":"+rec.SegKey]; ok {
			return loader.AddEdge(id, paliID, model.EdgeTranslates, nil, mustJSON(translatesPayload{Author: rec.Author}))
		}
		return nil
	}
	htmlRef := func(rec segLine, prefix string) error {
		_, err := loader.EnsureNode(model.NodeSegment, prefix+rec.Muid+":"+rec.SegKey, mustJSON(rec.SegmentRecord))
		return err
	}

	steps := []struct {
		file string
		fn   func(segLine) error
	}{
		{string(model.StreamRoot) + ".ndjson", pali},
		{string(model.StreamTranslation) + ".ndjson", translation},
		{string(model.StreamHTML) + ".ndjson", func(r segLine) error { return htmlRef(r, "html:") }},
		{string(model.StreamReference) + ".ndjson", func(r segLine) error { return htmlRef(r, "reference:") }},
	}
	for _, s := range steps {
		if err := streamNDJSON(filepath.Join(cfg.Canon.ParsedDir(), s.file), s.fn); err != nil {
			return fmt.Errorf("%s: %w", s.file, err)
		}
	}
	return nil
}

func streamNDJSON(path string, fn func(segLine) error) error {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer func() { _ = f.Close() }()
	r := bufio.NewReader(f)
	for {
		line, err := r.ReadString('\n')
		if line != "" {
			var rec segLine
			if jerr := json.Unmarshal([]byte(strings.TrimRight(line, "\r\n")), &rec); jerr != nil {
				return fmt.Errorf("bad line: %w", jerr)
			}
			if ferr := fn(rec); ferr != nil {
				return ferr
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
	}
}

func indexFTS(ctx context.Context, store *graph.Store, loader *graph.Loader) error {
	rows, err := store.DB().QueryContext(ctx, `
		SELECT p.id, json_extract(p.data, '$.text'), json_extract(e.data, '$.text')
		FROM node p
		JOIN node e ON e.id = (
			SELECT MIN(tr.src) FROM edge tr
			WHERE tr.dst = p.id AND tr.type = 'TRANSLATES'
		)
		WHERE p.type = 'segment' AND p.key LIKE 'pli:%'
		ORDER BY p.id`)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var id int64
		var pali, en string
		if err := rows.Scan(&id, &pali, &en); err != nil {
			return err
		}
		if err := loader.SetFTS(id, pali, en); err != nil {
			return err
		}
	}
	return rows.Err()
}

func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}
