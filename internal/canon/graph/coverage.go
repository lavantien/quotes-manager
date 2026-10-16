package graph

import (
	"context"
	"fmt"
)

const dpdRootKeyPrefix = "dpd-root:"

type RootMember struct {
	Lemma1   string `json:"lemma_1"`
	POS      string `json:"pos"`
	Meaning1 string `json:"meaning_1"`
	Segments int64  `json:"segments"`
}

type CoverRow struct {
	Muid     string `json:"muid"`
	Name     string `json:"name"`
	Segments int64  `json:"segments"`
	Covered  bool   `json:"covered"`
}

type DivisionCoverage struct {
	Division string `json:"division"`
	Texts    int64  `json:"texts"`
	Covered  int64  `json:"covered"`
}

func (s *Store) CoverageSummary(ctx context.Context) ([]DivisionCoverage, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT dv.key,
			COUNT(t.id),
			COALESCE(SUM(EXISTS (SELECT 1 FROM edge c
				WHERE c.src = t.id AND c.type = 'COVERED_BY')), 0)
		FROM node dv
		JOIN edge tch ON tch.dst = dv.id AND tch.type = 'CHILD_OF'
		JOIN node t ON t.id = tch.src AND t.type = 'text'
		WHERE dv.type = 'division'
		GROUP BY dv.key
		ORDER BY dv.key`)
	if err != nil {
		return nil, fmt.Errorf("coverage summary: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := []DivisionCoverage{}
	for rows.Next() {
		var d DivisionCoverage
		if err := rows.Scan(&d.Division, &d.Texts, &d.Covered); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (s *Store) RootFamily(ctx context.Context, root string, limit int) ([]RootMember, error) {
	query := addLimit(`
		SELECT substr(hw.key, 5),
			COALESCE(json_extract(hw.data, '$.pos'), ''),
			COALESCE(json_extract(hw.data, '$.meaning_1'), ''),
			(SELECT COUNT(*) FROM edge hwe
			 WHERE hwe.dst = hw.id AND hwe.type = 'HAS_WORD')
		FROM node r
		JOIN edge de ON de.dst = r.id AND de.type = 'DERIVED_FROM'
		JOIN node hw ON hw.id = de.src AND hw.type = 'headword'
		WHERE r.type = 'dpd_root' AND r.key = ?
		ORDER BY hw.key`, limit)
	rows, err := s.db.QueryContext(ctx, query, dpdRootKeyPrefix+root)
	if err != nil {
		return nil, fmt.Errorf("root family %q: %w", root, err)
	}
	defer func() { _ = rows.Close() }()
	out := []RootMember{}
	for rows.Next() {
		var m RootMember
		if err := rows.Scan(&m.Lemma1, &m.POS, &m.Meaning1, &m.Segments); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *Store) Coverage(ctx context.Context, divisionKey string, limit int) ([]CoverRow, error) {
	query := addLimit(`
		SELECT t.key,
			COALESCE(json_extract(t.data, '$.name'), ''),
			(SELECT COUNT(*) FROM edge pe
			 JOIN node p ON p.id = pe.src AND p.type = 'segment' AND p.key LIKE 'pli:%'
			 WHERE pe.dst = t.id AND pe.type = 'CHILD_OF'),
			EXISTS (SELECT 1 FROM edge c
			 WHERE c.src = t.id AND c.type = 'COVERED_BY')
		FROM node dv
		JOIN edge tch ON tch.dst = dv.id AND tch.type = 'CHILD_OF'
		JOIN node t ON t.id = tch.src AND t.type = 'text'
		WHERE dv.type = 'division' AND dv.key = ?
		ORDER BY t.key`, limit)
	rows, err := s.db.QueryContext(ctx, query, divisionKey)
	if err != nil {
		return nil, fmt.Errorf("coverage %q: %w", divisionKey, err)
	}
	defer func() { _ = rows.Close() }()
	out := []CoverRow{}
	for rows.Next() {
		var r CoverRow
		if err := rows.Scan(&r.Muid, &r.Name, &r.Segments, &r.Covered); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
