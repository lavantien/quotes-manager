package analytics

import (
	"context"
	"errors"
	"fmt"
)

var ErrUnknownAggregate = errors.New("analytics: unknown aggregate")

type Param struct {
	Name string `json:"name"`
	Desc string `json:"description"`
}

type CatalogEntry struct {
	Name   string  `json:"name"`
	Desc   string  `json:"description"`
	Params []Param `json:"params,omitempty"`
}

var (
	paramDivision = Param{Name: "division", Desc: "exact division key filter, e.g. mn"}
	paramLimit    = Param{Name: "limit", Desc: "row cap, clamped to [1,500], default 50"}
)

type aggregate struct {
	name     string
	desc     string
	params   []Param
	division bool
	sql      func(division string, limit int) (string, []any)
}

var aggregates = []aggregate{
	{
		name:     "top-headwords",
		desc:     "headwords by HAS_WORD occurrence count and text spread",
		params:   []Param{paramDivision, paramLimit},
		division: true,
		sql: func(division string, limit int) (string, []any) {
			q := `SELECT headword, COUNT(*) AS occurrences, COUNT(DISTINCT muid) AS texts
				FROM word_occurrences`
			var args []any
			if division != "" {
				q += ` WHERE division = ?`
				args = append(args, division)
			}
			q += ` GROUP BY headword ORDER BY occurrences DESC, headword LIMIT ?`
			return q, append(args, limit)
		},
	},
	{
		name:     "root-ranking",
		desc:     "roots by distinct headword count and occurrence volume",
		params:   []Param{paramDivision, paramLimit},
		division: true,
		sql: func(division string, limit int) (string, []any) {
			q := `SELECT root, COUNT(DISTINCT headword) AS headwords, COUNT(*) AS occurrences
				FROM word_occurrences
				WHERE root IS NOT NULL`
			var args []any
			if division != "" {
				q += ` AND division = ?`
				args = append(args, division)
			}
			q += ` GROUP BY root ORDER BY headwords DESC, root LIMIT ?`
			return q, append(args, limit)
		},
	},
	{
		name:     "headword-density",
		desc:     "texts by distinct headwords per pali segment",
		params:   []Param{paramDivision, paramLimit},
		division: true,
		sql: func(division string, limit int) (string, []any) {
			q := `SELECT muid, division, name, segments, distinct_headwords,
				CASE WHEN segments > 0
					THEN ROUND(distinct_headwords * 1.0 / segments, 3) END AS density
				FROM text_stats`
			var args []any
			if division != "" {
				q += ` WHERE division = ?`
				args = append(args, division)
			}
			q += ` ORDER BY density DESC NULLS LAST, muid LIMIT ?`
			return q, append(args, limit)
		},
	},
	{
		name:   "vocab-overlap",
		desc:   "division pairs by shared distinct headwords and jaccard similarity",
		params: []Param{paramLimit},
		sql: func(_ string, limit int) (string, []any) {
			return `WITH vocab AS (
					SELECT DISTINCT division, headword FROM word_occurrences
					WHERE division IS NOT NULL
				), sizes AS (
					SELECT division, COUNT(*) AS vocab_size FROM vocab GROUP BY division
				), pair AS (
					SELECT a.division AS division_a, b.division AS division_b
					FROM sizes a JOIN sizes b ON a.division < b.division
				), shared AS (
					SELECT v1.division AS division_a, v2.division AS division_b,
						COUNT(*) AS shared_headwords
					FROM vocab v1 JOIN vocab v2
						ON v1.division < v2.division AND v1.headword = v2.headword
					GROUP BY 1, 2
				)
				SELECT p.division_a, p.division_b,
					COALESCE(s.shared_headwords, 0) AS shared_headwords,
					sa.vocab_size, sb.vocab_size,
					ROUND(COALESCE(s.shared_headwords, 0) * 1.0 /
						(sa.vocab_size + sb.vocab_size - COALESCE(s.shared_headwords, 0)), 3)
						AS jaccard
				FROM pair p
				JOIN sizes sa ON sa.division = p.division_a
				JOIN sizes sb ON sb.division = p.division_b
				LEFT JOIN shared s
					ON s.division_a = p.division_a AND s.division_b = p.division_b
				ORDER BY shared_headwords DESC, division_a, division_b
				LIMIT ?`, []any{limit}
		},
	},
	{
		name:   "discrepancy-trend",
		desc:   "discrepancy counts by run and check",
		params: []Param{paramLimit},
		sql: func(_ string, limit int) (string, []any) {
			return `SELECT r.id AS run_id, r.started, d."check" AS "check",
					COUNT(*) AS discrepancies
				FROM discrepancy d JOIN run r ON r.id = d.run_id
				GROUP BY r.id, r.started, d."check"
				ORDER BY r.id DESC, d."check"
				LIMIT ?`, []any{limit}
		},
	},
	{
		name:     "coverage-by-division",
		desc:     "edition coverage per division: texts, covered texts, mean segments",
		params:   []Param{paramDivision, paramLimit},
		division: true,
		sql: func(division string, limit int) (string, []any) {
			q := `SELECT division, COUNT(*) AS texts,
					COUNT(*) FILTER (WHERE covered) AS covered,
					ROUND(AVG(segments), 1) AS mean_segments
				FROM text_stats
				WHERE division IS NOT NULL`
			var args []any
			if division != "" {
				q += ` AND division = ?`
				args = append(args, division)
			}
			q += ` GROUP BY division ORDER BY division LIMIT ?`
			return q, append(args, limit)
		},
	},
}

func lookupAggregate(name string) (aggregate, bool) {
	for _, a := range aggregates {
		if a.name == name {
			return a, true
		}
	}
	return aggregate{}, false
}

func (d *DB) Aggregate(ctx context.Context, name, division string, limit int) (Table, error) {
	a, ok := lookupAggregate(name)
	if !ok {
		return Table{}, fmt.Errorf("%w: %q", ErrUnknownAggregate, name)
	}
	if division != "" && !a.division {
		return Table{}, fmt.Errorf("analytics: %q takes no division filter", name)
	}
	q, args := a.sql(division, clampLimit(limit))
	return d.queryTable(ctx, q, args...)
}

func Catalog() []CatalogEntry {
	out := make([]CatalogEntry, 0, len(aggregates))
	for _, a := range aggregates {
		out = append(out, CatalogEntry{Name: a.name, Desc: a.desc, Params: a.params})
	}
	return out
}
