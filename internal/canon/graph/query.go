package graph

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/lavantien/quotes-manager/internal/canon/model"
)

const (
	paliKeyPrefix = "pli:"
	enKeyPrefix   = "en/"
)

const maxTreeDepth = 1000

type Pair struct {
	Muid   string
	SegKey string
	Pali   string
	En     string
	Author string
}

type TreeNode struct {
	Key   string
	Type  model.NodeType
	Ord   *int64
	Depth int
	Data  string
}

const translatorPick = `(
	SELECT tr.src
	FROM edge tr
	JOIN node n2 ON n2.id = tr.src
	WHERE tr.dst = pseg.id AND tr.type = 'TRANSLATES'
		AND n2.type = 'segment'
	ORDER BY tr.src
	LIMIT 1
)`

func keyRange(prefix string) (string, string) {
	lo := prefix
	hi := prefix[:len(prefix)-1] + string(prefix[len(prefix)-1]+1)
	return lo, hi
}

func splitPaliKey(key string) (muid, segKey string, err error) {
	rest, ok := strings.CutPrefix(key, paliKeyPrefix)
	if !ok {
		return "", "", fmt.Errorf("key %q lacks %q prefix", key, paliKeyPrefix)
	}
	muid, segKey, ok = strings.Cut(rest, ":")
	if !ok {
		return "", "", fmt.Errorf("key %q lacks muid:segkey split", key)
	}
	return muid, segKey, nil
}

func splitEnKey(key string) (author, muid, segKey string, err error) {
	rest, ok := strings.CutPrefix(key, enKeyPrefix)
	if !ok {
		return "", "", "", fmt.Errorf("key %q lacks %q prefix", key, enKeyPrefix)
	}
	parts := strings.SplitN(rest, ":", 3)
	if len(parts) != 3 {
		return "", "", "", fmt.Errorf("key %q lacks author:muid:segkey split", key)
	}
	return parts[0], parts[1], parts[2], nil
}

func textOf(dataJSON string) string {
	var payload struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal([]byte(dataJSON), &payload); err != nil {
		return ""
	}
	return payload.Text
}

func pairOf(paliKey string, enKey sql.NullString) (Pair, error) {
	p := Pair{}
	var err error
	if p.Muid, p.SegKey, err = splitPaliKey(paliKey); err != nil {
		return Pair{}, err
	}
	if enKey.Valid {
		if author, _, _, err := splitEnKey(enKey.String); err == nil {
			p.Author = author
		}
	}
	return p, nil
}

func addLimit(query string, limit int) string {
	if limit > 0 {
		return query + fmt.Sprintf(" LIMIT %d", limit)
	}
	return query
}

func addLimitOffset(query string, limit, offset int) string {
	switch {
	case limit > 0 && offset > 0:
		return query + fmt.Sprintf(" LIMIT %d OFFSET %d", limit, offset)
	case limit > 0:
		return query + fmt.Sprintf(" LIMIT %d", limit)
	case offset > 0:
		return query + fmt.Sprintf(" LIMIT -1 OFFSET %d", offset)
	default:
		return query
	}
}

func scanPairs(rows *sql.Rows) ([]Pair, error) {
	out := []Pair{}
	for rows.Next() {
		var paliKey, paliData string
		var enKey, enData sql.NullString
		if err := rows.Scan(&paliKey, &paliData, &enKey, &enData); err != nil {
			return nil, err
		}
		p, err := pairOf(paliKey, enKey)
		if err != nil {
			return nil, err
		}
		p.Pali = textOf(paliData)
		if enData.Valid {
			p.En = textOf(enData.String)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) Segments(ctx context.Context, muid string, withEn bool, limit, offset int) ([]Pair, error) {
	lo, hi := keyRange(paliKeyPrefix + muid + ":")
	join := "LEFT JOIN"
	if withEn {
		join = "JOIN"
	}
	query := addLimitOffset(`
		SELECT pseg.key, pseg.data, eseg.key, eseg.data
		FROM node t
		JOIN edge pe ON pe.dst = t.id AND pe.type = 'CHILD_OF'
		JOIN node pseg ON pseg.id = pe.src AND pseg.type = 'segment'
			AND pseg.key >= ? AND pseg.key < ?
		`+join+` node eseg ON eseg.id = `+translatorPick+`
		WHERE t.type = 'text' AND t.key = ?
		ORDER BY pe.ord, pseg.key`, limit, offset)
	rows, err := s.db.QueryContext(ctx, query, lo, hi, muid)
	if err != nil {
		return nil, fmt.Errorf("segments %s: %w", muid, err)
	}
	defer func() { _ = rows.Close() }()
	return scanPairs(rows)
}

func (s *Store) SearchFTS(ctx context.Context, text, lang string, limit, offset int) ([]Pair, error) {
	var column string
	switch lang {
	case "pali", "en":
		column = lang
	default:
		return nil, fmt.Errorf("search lang %q must be pali or en", lang)
	}
	if strings.TrimSpace(text) == "" {
		return nil, fmt.Errorf("search text must not be empty")
	}
	match := column + `:"` + strings.ReplaceAll(text, `"`, `""`) + `"`
	query := addLimitOffset(`
		SELECT pseg.key, pseg.data, eseg.key, eseg.data
		FROM fts_segment f
		JOIN node pseg ON pseg.id = f.rowid AND pseg.type = 'segment'
		LEFT JOIN node eseg ON eseg.id = `+translatorPick+`
		WHERE fts_segment MATCH ?
		ORDER BY pseg.key`, limit, offset)
	rows, err := s.db.QueryContext(ctx, query, match)
	if err != nil {
		return nil, fmt.Errorf("search %s:%q: %w", lang, text, err)
	}
	defer func() { _ = rows.Close() }()
	return scanPairs(rows)
}

func (s *Store) Tree(ctx context.Context, divisionKey, types string, limit, offset int) ([]TreeNode, error) {
	var typeFilter string
	switch types {
	case "segment", "text":
		typeFilter = "AND n.type = '" + types + "'"
	case "all":
		typeFilter = ""
	default:
		return nil, fmt.Errorf("tree types %q must be segment, text, or all", types)
	}
	query := addLimitOffset(`
		WITH RECURSIVE reach(id, depth) AS (
			SELECT id, 0 FROM node
			WHERE type = 'division' AND key = ?
			UNION
			SELECT e.src, r.depth + 1
			FROM reach r
			JOIN edge e ON e.dst = r.id AND e.type = 'CHILD_OF'
			WHERE r.depth < `+fmt.Sprintf("%d", maxTreeDepth)+`
		), ordmin AS (
			SELECT e.src, MIN(e.ord) AS ord
			FROM edge e
			WHERE e.type = 'CHILD_OF' AND e.dst IN (SELECT id FROM reach)
			GROUP BY e.src
		)
		SELECT n.key, n.type, o.ord, r.depth, n.data
		FROM (SELECT id, MIN(depth) AS depth FROM reach GROUP BY id) r
		JOIN node n ON n.id = r.id
		LEFT JOIN ordmin o ON o.src = n.id
		WHERE 1 = 1 `+typeFilter+`
		ORDER BY r.depth, o.ord, n.key`, limit, offset)
	rows, err := s.db.QueryContext(ctx, query, divisionKey)
	if err != nil {
		return nil, fmt.Errorf("tree %s: %w", divisionKey, err)
	}
	defer func() { _ = rows.Close() }()
	out := []TreeNode{}
	for rows.Next() {
		var n TreeNode
		if err := rows.Scan(&n.Key, &n.Type, &n.Ord, &n.Depth, &n.Data); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func (s *Store) Stats(ctx context.Context) (map[string]int64, error) {
	out := map[string]int64{}
	for _, table := range [...]string{"node", "edge"} {
		if err := s.countByType(ctx, table, out); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (s *Store) countByType(ctx context.Context, table string, out map[string]int64) error {
	rows, err := s.db.QueryContext(ctx, `SELECT type, COUNT(*) FROM `+table+` GROUP BY type`)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var typ string
		var n int64
		if err := rows.Scan(&typ, &n); err != nil {
			return err
		}
		out[table+":"+typ] = n
	}
	return rows.Err()
}
