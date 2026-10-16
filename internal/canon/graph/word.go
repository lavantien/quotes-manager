package graph

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/lavantien/quotes-manager/internal/canon/model"
	"github.com/lavantien/quotes-manager/internal/canon/norm"
)

const (
	headwordKeyPrefix = "dpd:"
	dpdFormKeyPrefix  = "dpd-form:"
)

type WordEntry struct {
	Lemma1     string `json:"lemma_1"`
	POS        string `json:"pos"`
	Meaning1   string `json:"meaning_1"`
	Root       string `json:"root"`
	FamilyRoot string `json:"family_root"`
	Segments   int64  `json:"segments"`
}

func (s *Store) ResolveForm(ctx context.Context, token string) ([]WordEntry, error) {
	var data string
	err := s.db.QueryRowContext(ctx,
		`SELECT data FROM node WHERE type = ? AND key = ?`,
		string(model.NodeDpdForm), dpdFormKeyPrefix+norm.FoldPali(token),
	).Scan(&data)
	if errors.Is(err, sql.ErrNoRows) {
		return []WordEntry{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("resolve form %q: %w", token, err)
	}
	var f struct {
		Headwords []string `json:"headwords"`
	}
	if err := json.Unmarshal([]byte(data), &f); err != nil {
		return nil, fmt.Errorf("resolve form %q: bad form node data: %w", token, err)
	}
	out := []WordEntry{}
	for _, h := range f.Headwords {
		var hwData string
		var segments int64
		err := s.db.QueryRowContext(ctx, `
			SELECT n.data,
				(SELECT COUNT(*) FROM edge e WHERE e.dst = n.id AND e.type = 'HAS_WORD')
			FROM node n WHERE n.type = ? AND n.key = ?`,
			string(model.NodeHeadword), headwordKeyPrefix+h,
		).Scan(&hwData, &segments)
		if err != nil {
			return nil, fmt.Errorf("resolve form %q: headword %q: %w", token, h, err)
		}
		var p struct {
			Lemma1     string `json:"lemma_1"`
			POS        string `json:"pos"`
			Meaning1   string `json:"meaning_1"`
			RootKey    string `json:"root_key"`
			FamilyRoot string `json:"family_root"`
		}
		if err := json.Unmarshal([]byte(hwData), &p); err != nil {
			return nil, fmt.Errorf("resolve form %q: bad headword %q data: %w", token, h, err)
		}
		out = append(out, WordEntry{
			Lemma1: p.Lemma1, POS: p.POS, Meaning1: p.Meaning1,
			Root: p.RootKey, FamilyRoot: p.FamilyRoot, Segments: segments,
		})
	}
	return out, nil
}

func (s *Store) HeadwordUsage(ctx context.Context, lemma string, withSegments bool, limit, offset int) (int64, []string, error) {
	var n int64
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM edge e
		JOIN node hw ON hw.id = e.dst AND hw.type = 'headword' AND hw.key = ?
		WHERE e.type = 'HAS_WORD'`, headwordKeyPrefix+lemma,
	).Scan(&n); err != nil {
		return 0, nil, fmt.Errorf("headword usage %q: %w", lemma, err)
	}
	if !withSegments || n == 0 {
		return n, nil, nil
	}
	query := addLimitOffset(`
		SELECT src.key FROM edge e
		JOIN node hw ON hw.id = e.dst AND hw.type = 'headword' AND hw.key = ?
		JOIN node src ON src.id = e.src
		WHERE e.type = 'HAS_WORD'
		ORDER BY src.key`, limit, offset)
	rows, err := s.db.QueryContext(ctx, query, headwordKeyPrefix+lemma)
	if err != nil {
		return 0, nil, fmt.Errorf("headword usage %q: %w", lemma, err)
	}
	defer func() { _ = rows.Close() }()
	refs := []string{}
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return 0, nil, err
		}
		refs = append(refs, strings.TrimPrefix(key, paliKeyPrefix))
	}
	return n, refs, rows.Err()
}
