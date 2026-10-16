package triangulate

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/lavantien/quotes-manager/internal/canon/model"
	"github.com/lavantien/quotes-manager/internal/config"
)

const (
	prefixPali = "pli:"
	prefixEn   = "en/"
	prefixHTML = "html:"
	prefixRef  = "reference:"
)

type parsedMeta struct {
	Files map[string]parsedFile `json:"files"`
}

type parsedFile struct {
	DupKeys          []string `json:"dup_keys"`
	NFCChanged       bool     `json:"nfc_changed"`
	EmptyValues      int      `json:"empty_values"`
	Muid             string   `json:"muid"`
	Stream           string   `json:"stream"`
	RecordMuids      []string `json:"record_muids"`
	PrefixMismatches []string `json:"prefix_mismatches"`
}

func loadParsedMeta(env Env) (parsedMeta, error) {
	if env.ParsedDir == "" {
		return parsedMeta{}, nil
	}
	b, err := os.ReadFile(filepath.Join(env.ParsedDir, "meta.json"))
	if err != nil {
		return parsedMeta{}, fmt.Errorf("read parsed meta: %w", err)
	}
	var m parsedMeta
	if err := json.Unmarshal(b, &m); err != nil {
		return parsedMeta{}, fmt.Errorf("decode parsed meta: %w", err)
	}
	return m, nil
}

func sortedFiles(m parsedMeta) []string {
	out := make([]string, 0, len(m.Files))
	for p := range m.Files {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

func splitPaliKey(key string) (muid, segkey string, ok bool) {
	return splitPrefixed(key, prefixPali)
}

func splitCognateKey(key, prefix string) (muid, segkey string, ok bool) {
	return splitPrefixed(key, prefix)
}

func splitPrefixed(key, prefix string) (head, tail string, ok bool) {
	after, hasPrefix := strings.CutPrefix(key, prefix)
	if !hasPrefix {
		return "", "", false
	}
	head, tail, ok = strings.Cut(after, ":")
	switch {
	case !ok, head == "", tail == "":
		return "", "", false
	}
	return head, tail, true
}

func splitEnKey(key string) (author, muid string, ok bool) {
	after, hasPrefix := strings.CutPrefix(key, prefixEn)
	if !hasPrefix {
		return "", "", false
	}
	author, rest, cut1 := strings.Cut(after, ":")
	muid, _, cut2 := strings.Cut(rest, ":")
	switch {
	case !cut1, !cut2, author == "", muid == "":
		return "", "", false
	}
	return author, muid, true
}

func runB001(ctx context.Context, cfg *config.Config, st Store, env Env) ([]model.Discrepancy, error) {
	db := st.DB()
	authorsByMuid, err := b001AuthorsByMuid(ctx, db)
	if err != nil {
		return nil, fmt.Errorf("b001: %w", err)
	}
	untranslated, err := b001Untranslated(ctx, db, cfg, authorsByMuid)
	if err != nil {
		return nil, fmt.Errorf("b001: %w", err)
	}
	orphans, err := b001Orphans(ctx, db, cfg)
	if err != nil {
		return nil, fmt.Errorf("b001: %w", err)
	}
	return append(untranslated, orphans...), nil
}

func b001AuthorsByMuid(ctx context.Context, db *sql.DB) (map[string]map[string]bool, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT key FROM node WHERE type = 'segment' AND key LIKE 'en/%' ORDER BY key`)
	if err != nil {
		return nil, fmt.Errorf("authors query: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := map[string]map[string]bool{}
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, fmt.Errorf("authors scan: %w", err)
		}
		author, muid, ok := splitEnKey(key)
		if !ok {
			continue
		}
		if out[muid] == nil {
			out[muid] = map[string]bool{}
		}
		out[muid][author] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("authors iterate: %w", err)
	}
	return out, nil
}

func b001Untranslated(ctx context.Context, db *sql.DB, cfg *config.Config, authorsByMuid map[string]map[string]bool) ([]model.Discrepancy, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT r.id, r.key, en.key AS en_key
		FROM node r
		LEFT JOIN edge tr ON tr.dst = r.id AND tr.type = 'TRANSLATES'
		LEFT JOIN node en ON en.id = tr.src
		WHERE r.type = 'segment' AND r.key LIKE 'pli:%'
		ORDER BY r.key`)
	if err != nil {
		return nil, fmt.Errorf("roots query: %w", err)
	}
	defer func() { _ = rows.Close() }()

	sev := model.Severity(cfg.Canon.SeverityFor("B001.untranslated"))
	var ds []model.Discrepancy
	flush := func(id int64, key string, present map[string]bool) {
		muid, segkey, ok := splitPaliKey(key)
		if !ok {
			return
		}
		for _, author := range sortedKeys(authorsByMuid[muid]) {
			if present[author] {
				continue
			}
			ds = append(ds, model.Discrepancy{
				Check:    model.CheckKeysetRootTranslation,
				Severity: sev,
				Muid:     muid,
				NodeID:   id,
				Detail:   fmt.Sprintf("untranslated by %s: %s:%s", author, muid, segkey),
			})
		}
	}

	var curKey string
	var curID int64
	curAuthors := map[string]bool{}
	started := false
	for rows.Next() {
		var id int64
		var key string
		var enKey sql.NullString
		if err := rows.Scan(&id, &key, &enKey); err != nil {
			return nil, fmt.Errorf("roots scan: %w", err)
		}
		if started && key != curKey {
			flush(curID, curKey, curAuthors)
			curAuthors = map[string]bool{}
		}
		started = true
		curKey, curID = key, id
		if enKey.Valid {
			if author, _, ok := splitEnKey(enKey.String); ok {
				curAuthors[author] = true
			}
		}
	}
	if started {
		flush(curID, curKey, curAuthors)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("roots iterate: %w", err)
	}
	return ds, nil
}

func b001Orphans(ctx context.Context, db *sql.DB, cfg *config.Config) ([]model.Discrepancy, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT n.id, n.key FROM node n
		WHERE n.type = 'segment' AND n.key LIKE 'en/%'
		AND NOT EXISTS (SELECT 1 FROM edge e WHERE e.src = n.id AND e.type = 'TRANSLATES')
		ORDER BY n.key`)
	if err != nil {
		return nil, fmt.Errorf("orphans query: %w", err)
	}
	defer func() { _ = rows.Close() }()
	sev := directionSeverity(cfg, "B001.orphan", model.Error)
	var ds []model.Discrepancy
	for rows.Next() {
		var id int64
		var key string
		if err := rows.Scan(&id, &key); err != nil {
			return nil, fmt.Errorf("orphans scan: %w", err)
		}
		muid := ""
		if _, m, ok := splitEnKey(key); ok {
			muid = m
		}
		ds = append(ds, model.Discrepancy{
			Check:    model.CheckKeysetRootTranslation,
			Severity: sev,
			Muid:     muid,
			NodeID:   id,
			Detail:   "orphan translation " + key,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("orphans iterate: %w", err)
	}
	return ds, nil
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

type diffRow struct {
	id  int64
	key string
}

func cognateExtra(ctx context.Context, db *sql.DB, prefix string) ([]diffRow, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT n.id, n.key FROM node n
		WHERE n.type = 'segment' AND n.key LIKE ?
		AND NOT EXISTS (
			SELECT 1 FROM node r
			WHERE r.type = 'segment' AND r.key = 'pli:' || substr(n.key, ?)
		)
		ORDER BY n.key`, prefix+"%", len(prefix)+1)
	if err != nil {
		return nil, fmt.Errorf("extra query %q: %w", prefix, err)
	}
	defer func() { _ = rows.Close() }()
	var out []diffRow
	for rows.Next() {
		var r diffRow
		if err := rows.Scan(&r.id, &r.key); err != nil {
			return nil, fmt.Errorf("extra scan: %w", err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("extra iterate: %w", err)
	}
	return out, nil
}

func cognateMissing(ctx context.Context, db *sql.DB, prefix string) ([]diffRow, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT r.id, r.key FROM node r
		WHERE r.type = 'segment' AND r.key LIKE 'pli:%'
		AND NOT EXISTS (
			SELECT 1 FROM node n
			WHERE n.type = 'segment' AND n.key = ? || substr(r.key, ?)
		)
		ORDER BY r.key`, prefix, len(prefixPali)+1)
	if err != nil {
		return nil, fmt.Errorf("missing query %q: %w", prefix, err)
	}
	defer func() { _ = rows.Close() }()
	var out []diffRow
	for rows.Next() {
		var r diffRow
		if err := rows.Scan(&r.id, &r.key); err != nil {
			return nil, fmt.Errorf("missing scan: %w", err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("missing iterate: %w", err)
	}
	return out, nil
}

func cognateDiscrepancies(check model.CheckID, extraSev, missingSev model.Severity, prefix, label string, extra, missing []diffRow) []model.Discrepancy {
	var ds []model.Discrepancy
	for _, r := range extra {
		muid, _, _ := splitCognateKey(r.key, prefix)
		ds = append(ds, model.Discrepancy{
			Check: check, Severity: extraSev, Muid: muid, NodeID: r.id,
			Detail: label + " key without root: " + r.key,
		})
	}
	for _, r := range missing {
		muid, _, _ := splitPaliKey(r.key)
		ds = append(ds, model.Discrepancy{
			Check: check, Severity: missingSev, Muid: muid, NodeID: r.id,
			Detail: "root key without " + label + ": " + r.key,
		})
	}
	return ds
}

func runB002(ctx context.Context, cfg *config.Config, st Store, env Env) ([]model.Discrepancy, error) {
	db := st.DB()
	extra, err := cognateExtra(ctx, db, prefixHTML)
	if err != nil {
		return nil, fmt.Errorf("b002: %w", err)
	}
	missing, err := cognateMissing(ctx, db, prefixHTML)
	if err != nil {
		return nil, fmt.Errorf("b002: %w", err)
	}
	return cognateDiscrepancies(
		model.CheckKeysetRootHTML,
		model.Severity(cfg.Canon.SeverityFor("B002.extra")),
		directionSeverity(cfg, "B002.missing", model.Info),
		prefixHTML, "html", extra, missing,
	), nil
}

func runB003(ctx context.Context, cfg *config.Config, st Store, env Env) ([]model.Discrepancy, error) {
	db := st.DB()
	extra, err := cognateExtra(ctx, db, prefixRef)
	if err != nil {
		return nil, fmt.Errorf("b003: %w", err)
	}
	missing, err := cognateMissing(ctx, db, prefixRef)
	if err != nil {
		return nil, fmt.Errorf("b003: %w", err)
	}
	return cognateDiscrepancies(
		model.CheckKeysetReference,
		model.Severity(cfg.Canon.SeverityFor("B003.extra")),
		directionSeverity(cfg, "B003.missing", model.Info),
		prefixRef, "reference", extra, missing,
	), nil
}

func runB004(ctx context.Context, cfg *config.Config, st Store, env Env) ([]model.Discrepancy, error) {
	meta, err := loadParsedMeta(env)
	if err != nil {
		return nil, fmt.Errorf("b004: %w", err)
	}
	var ds []model.Discrepancy
	for _, path := range sortedFiles(meta) {
		f := meta.Files[path]
		if len(f.DupKeys) == 0 {
			continue
		}
		ds = append(ds, model.Discrepancy{
			Check:    model.CheckDuplicateKeys,
			Severity: model.Severity(cfg.Canon.SeverityFor(string(model.CheckDuplicateKeys))),
			Muid:     f.Muid,
			Detail: fmt.Sprintf("dup keys in %s: %s", path,
				capJoin(f.DupKeys, cfg.Canon.Triangulate.DetailSampleMax)),
		})
	}
	return ds, nil
}

func runB005(ctx context.Context, cfg *config.Config, st Store, env Env) ([]model.Discrepancy, error) {
	meta, err := loadParsedMeta(env)
	if err != nil {
		return nil, fmt.Errorf("b005: %w", err)
	}
	emptyByMuid := map[string]int{}
	for _, f := range meta.Files {
		if f.Stream != string(model.StreamRoot) && f.Stream != string(model.StreamTranslation) {
			continue
		}
		emptyByMuid[f.Muid] += f.EmptyValues
	}
	below := model.Severity(cfg.Canon.SeverityFor(string(model.CheckEmptyValue)))
	atOrAbove := model.Warn
	if severityRank(below) > severityRank(model.Warn) {
		atOrAbove = below
	}
	var ds []model.Discrepancy
	for _, muid := range sortedMuids(emptyByMuid) {
		n := emptyByMuid[muid]
		if n == 0 {
			continue
		}
		sev := below
		if n >= cfg.Canon.Triangulate.EmptyWarnPerMuid {
			sev = atOrAbove
		}
		ds = append(ds, model.Discrepancy{
			Check:    model.CheckEmptyValue,
			Severity: sev,
			Muid:     muid,
			Detail:   fmt.Sprintf("empty values: %d (threshold %d)", n, cfg.Canon.Triangulate.EmptyWarnPerMuid),
		})
	}
	return ds, nil
}

func sortedMuids(m map[string]int) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func runB006(ctx context.Context, cfg *config.Config, st Store, env Env) ([]model.Discrepancy, error) {
	meta, err := loadParsedMeta(env)
	if err != nil {
		return nil, fmt.Errorf("b006: %w", err)
	}
	var ds []model.Discrepancy
	for _, path := range sortedFiles(meta) {
		f := meta.Files[path]
		if !f.NFCChanged {
			continue
		}
		ds = append(ds, model.Discrepancy{
			Check:    model.CheckNFCChanged,
			Severity: model.Severity(cfg.Canon.SeverityFor(string(model.CheckNFCChanged))),
			Muid:     f.Muid,
			Detail:   "nfc changed: " + path,
		})
	}
	return ds, nil
}

func runB007(ctx context.Context, cfg *config.Config, st Store, env Env) ([]model.Discrepancy, error) {
	meta, err := loadParsedMeta(env)
	if err != nil {
		return nil, fmt.Errorf("b007: %w", err)
	}
	var ds []model.Discrepancy
	for _, path := range sortedFiles(meta) {
		f := meta.Files[path]
		if len(f.PrefixMismatches) == 0 {
			continue
		}
		ds = append(ds, model.Discrepancy{
			Check:    model.CheckPrefixMismatch,
			Severity: model.Severity(cfg.Canon.SeverityFor(string(model.CheckPrefixMismatch))),
			Muid:     f.Muid,
			Detail: fmt.Sprintf("quarantined or malformed keys in %s: %s", path,
				capJoin(f.PrefixMismatches, cfg.Canon.Triangulate.DetailSampleMax)),
		})
	}
	return ds, nil
}
