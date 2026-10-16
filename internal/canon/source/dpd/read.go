package dpd

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

type Headword struct {
	ID           int64
	Lemma1       string
	Lemma2       string
	Pos          string
	Grammar      string
	Meaning1     string
	MeaningLit   string
	Sanskrit     string
	RootKey      string
	RootSign     string
	FamilyRoot   string
	Construction string
	CompoundType string
	Stem         string
	Inflections  string
}

type Root struct {
	Root         string
	RootInComps  string
	RootHasVerb  string
	RootGroup    string
	RootSign     string
	RootMeaning  string
	SanskritRoot string
}

type LookupRow struct {
	LookupKey     string
	Headwords     string
	Deconstructor string
}

const (
	headwordColumns = `id, lemma_1, lemma_2, pos, grammar, meaning_1, meaning_lit, sanskrit, root_key, root_sign, family_root, construction, compound_type, stem, inflections`
	rootColumns     = `root, root_in_comps, root_has_verb, root_group, root_sign, root_meaning, sanskrit_root`
	lookupColumns   = `lookup_key, headwords, deconstructor`
)

func Open(path string) (*sql.DB, error) {
	if path == "" {
		return nil, fmt.Errorf("dpd open: empty path")
	}
	if strings.ContainsAny(path, "?%") {
		return nil, fmt.Errorf("dpd open: path %q contains %q, unusable as a driver DSN", path, `?%`)
	}
	db, err := sql.Open("sqlite3", "file:"+filepath.ToSlash(path)+"?mode=ro&_query_only=1")
	if err != nil {
		return nil, fmt.Errorf("dpd open %s: %w", path, err)
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("dpd open %s: %w", path, err)
	}
	return db, nil
}

func StreamHeadwords(ctx context.Context, db *sql.DB, yield func(Headword) error) error {
	rows, err := db.QueryContext(ctx,
		"SELECT "+headwordColumns+" FROM dpd_headwords ORDER BY lemma_1")
	if err != nil {
		return fmt.Errorf("dpd headwords query: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var (
			h                                                         Headword
			lemma1, lemma2, pos, grammar                              sql.NullString
			meaning1, meaningLit, sanskrit                            sql.NullString
			rootKey, rootSign, familyRoot, construction, compoundType sql.NullString
			stem, inflections                                         sql.NullString
		)
		if err := rows.Scan(
			&h.ID, &lemma1, &lemma2, &pos, &grammar, &meaning1, &meaningLit,
			&sanskrit, &rootKey, &rootSign, &familyRoot, &construction,
			&compoundType, &stem, &inflections,
		); err != nil {
			return fmt.Errorf("dpd headwords scan: %w", err)
		}
		h.Lemma1 = lemma1.String
		h.Lemma2 = lemma2.String
		h.Pos = pos.String
		h.Grammar = grammar.String
		h.Meaning1 = meaning1.String
		h.MeaningLit = meaningLit.String
		h.Sanskrit = sanskrit.String
		h.RootKey = rootKey.String
		h.RootSign = rootSign.String
		h.FamilyRoot = familyRoot.String
		h.Construction = construction.String
		h.CompoundType = compoundType.String
		h.Stem = stem.String
		h.Inflections = inflections.String
		if err := yield(h); err != nil {
			return err
		}
	}
	return rows.Err()
}

func StreamRoots(ctx context.Context, db *sql.DB, yield func(Root) error) error {
	rows, err := db.QueryContext(ctx,
		"SELECT "+rootColumns+" FROM dpd_roots ORDER BY root")
	if err != nil {
		return fmt.Errorf("dpd roots query: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var (
			r                                                          Root
			root, inComps, hasVerb, group, sign, meaning, sanskritRoot sql.NullString
		)
		if err := rows.Scan(&root, &inComps, &hasVerb, &group, &sign, &meaning, &sanskritRoot); err != nil {
			return fmt.Errorf("dpd roots scan: %w", err)
		}
		r.Root = root.String
		r.RootInComps = inComps.String
		r.RootHasVerb = hasVerb.String
		r.RootGroup = group.String
		r.RootSign = sign.String
		r.RootMeaning = meaning.String
		r.SanskritRoot = sanskritRoot.String
		if err := yield(r); err != nil {
			return err
		}
	}
	return rows.Err()
}

func StreamLookup(ctx context.Context, db *sql.DB, yield func(LookupRow) error) error {
	rows, err := db.QueryContext(ctx,
		"SELECT "+lookupColumns+" FROM lookup ORDER BY lookup_key")
	if err != nil {
		return fmt.Errorf("dpd lookup query: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var (
			l                    LookupRow
			key, headwords, deco sql.NullString
		)
		if err := rows.Scan(&key, &headwords, &deco); err != nil {
			return fmt.Errorf("dpd lookup scan: %w", err)
		}
		l.LookupKey = key.String
		l.Headwords = headwords.String
		l.Deconstructor = deco.String
		if err := yield(l); err != nil {
			return err
		}
	}
	return rows.Err()
}

func SplitInflections(csv string) []string {
	if csv == "" {
		return nil
	}
	parts := strings.Split(csv, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func UnpackList(packed string) []string {
	if packed == "" {
		return nil
	}
	var out []string
	if err := json.Unmarshal([]byte(packed), &out); err != nil {
		return nil
	}
	return out
}
