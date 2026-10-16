package export

import (
	"context"
	"database/sql"
)

const wordOccurrencesSQL = `
SELECT p.id,
	substr(p.key, 5, instr(substr(p.key, 5), ':') - 1),
	substr(p.key, 5 + instr(substr(p.key, 5), ':')),
	dv.key,
	json_extract(hw.data, '$.lemma_1'),
	json_extract(hw.data, '$.root_key'),
	json_extract(hw.data, '$.family_root'),
	e.data
FROM edge e
JOIN node p ON p.id = e.src
	AND p.type = 'segment' AND p.key LIKE 'pli:%'
JOIN node hw ON hw.id = e.dst AND hw.type = 'headword'
LEFT JOIN node t ON t.id = (
	SELECT MIN(tc.dst) FROM edge tc
	JOIN node tn ON tn.id = tc.dst AND tn.type = 'text'
	WHERE tc.src = p.id AND tc.type = 'CHILD_OF'
)
LEFT JOIN node dv ON dv.id = (
	SELECT MIN(dc.dst) FROM edge dc
	JOIN node dn ON dn.id = dc.dst AND dn.type = 'division'
	WHERE dc.src = t.id AND dc.type = 'CHILD_OF'
)
WHERE e.type = 'HAS_WORD'
ORDER BY p.key, hw.key`

const textStatsSQL = `
SELECT t.key,
	dv.key,
	json_extract(t.data, '$.name'),
	(SELECT COUNT(*) FROM node p
		WHERE p.type = 'segment' AND p.key LIKE 'pli:' || t.key || ':%'),
	(SELECT COUNT(*) FROM node p
		WHERE p.type = 'segment' AND p.key LIKE 'pli:' || t.key || ':%'
		AND EXISTS (SELECT 1 FROM edge tr
			WHERE tr.dst = p.id AND tr.type = 'TRANSLATES')),
	(SELECT COUNT(*) FROM edge w JOIN node p ON p.id = w.src
		WHERE w.type = 'HAS_WORD'
		AND p.type = 'segment' AND p.key LIKE 'pli:' || t.key || ':%'),
	(SELECT COUNT(DISTINCT w.dst) FROM edge w JOIN node p ON p.id = w.src
		WHERE w.type = 'HAS_WORD'
		AND p.type = 'segment' AND p.key LIKE 'pli:' || t.key || ':%'),
	EXISTS (SELECT 1 FROM edge c
		WHERE c.src = t.id AND c.type = 'COVERED_BY')
FROM node t
LEFT JOIN node dv ON dv.id = (
	SELECT MIN(dc.dst) FROM edge dc
	JOIN node dn ON dn.id = dc.dst AND dn.type = 'division'
	WHERE dc.src = t.id AND dc.type = 'CHILD_OF'
)
WHERE t.type = 'text'
ORDER BY t.key`

func copyWordOccurrences(ctx context.Context, sdb *sql.DB, appendRow func(vals ...any) error) error {
	rows, err := sdb.QueryContext(ctx, wordOccurrencesSQL)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var paliID int64
		var muid, segKey string
		var division, headword, root, family, forms sql.NullString
		if err := rows.Scan(&paliID, &muid, &segKey, &division, &headword, &root, &family, &forms); err != nil {
			return err
		}
		if err := appendRow(paliID, muid, nullStr(division), segKey, nullStr(headword),
			nullStr(root), nullStr(family), nullStr(forms)); err != nil {
			return err
		}
	}
	return rows.Err()
}

func copyTextStats(ctx context.Context, sdb *sql.DB, appendRow func(vals ...any) error) error {
	rows, err := sdb.QueryContext(ctx, textStatsSQL)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var muid string
		var division, name sql.NullString
		var segments, translated, hasWord, distinctHeadwords, covered int64
		if err := rows.Scan(&muid, &division, &name, &segments, &translated,
			&hasWord, &distinctHeadwords, &covered); err != nil {
			return err
		}
		if err := appendRow(muid, nullStr(division), nullStr(name), segments, translated,
			hasWord, distinctHeadwords, covered != 0); err != nil {
			return err
		}
	}
	return rows.Err()
}
