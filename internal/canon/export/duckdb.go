package export

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"os"

	"github.com/lavantien/quotes-manager/internal/canon/graph"
	"github.com/lavantien/quotes-manager/internal/config"

	"github.com/duckdb/duckdb-go/v2"
	_ "github.com/mattn/go-sqlite3"
)

const ddl = `
CREATE TABLE node (id BIGINT, type VARCHAR, key VARCHAR, data VARCHAR);
CREATE TABLE edge (id BIGINT, src BIGINT, dst BIGINT, type VARCHAR, ord BIGINT, data VARCHAR);
CREATE TABLE run (id BIGINT, bilara_commit VARCHAR, editions_commit VARCHAR, dpd_tag VARCHAR, started VARCHAR, finished VARCHAR);
CREATE TABLE discrepancy (id BIGINT, run_id BIGINT, "check" VARCHAR, severity VARCHAR, muid VARCHAR, node_id BIGINT, detail VARCHAR);
CREATE TABLE segment_pairs (pali_id BIGINT, muid VARCHAR, seg_key VARCHAR, pali VARCHAR, en VARCHAR, author VARCHAR, html VARCHAR, reference VARCHAR);
CREATE TABLE dpd_entries (id BIGINT, lemma_1 VARCHAR, pos VARCHAR, meaning_1 VARCHAR, root VARCHAR, family_root VARCHAR);
CREATE TABLE word_occurrences (pali_id BIGINT, muid VARCHAR, division VARCHAR, seg_key VARCHAR, headword VARCHAR, root VARCHAR, family_root VARCHAR, forms VARCHAR);
CREATE TABLE text_stats (muid VARCHAR, division VARCHAR, name VARCHAR, segments BIGINT, translated BIGINT, has_word_edges BIGINT, distinct_headwords BIGINT, covered BOOLEAN);
`

const segmentPairsSQL = `
SELECT p.id,
	substr(p.key, 5, instr(substr(p.key, 5), ':') - 1),
	substr(p.key, 5 + instr(substr(p.key, 5), ':')),
	json_extract(p.data, '$.text'),
	json_extract(pe.data, '$.text'),
	CASE WHEN pe.key IS NULL THEN NULL
		ELSE substr(pe.key, 4, instr(substr(pe.key, 4), ':') - 1) END,
	json_extract(h.data, '$.text'),
	json_extract(r.data, '$.text')
FROM node p
LEFT JOIN node pe ON pe.id = (
	SELECT MIN(tr.src) FROM edge tr
	WHERE tr.dst = p.id AND tr.type = 'TRANSLATES'
)
LEFT JOIN node h ON h.type = 'segment' AND h.key = 'html' || substr(p.key, 4)
LEFT JOIN node r ON r.type = 'segment' AND r.key = 'reference' || substr(p.key, 4)
WHERE p.type = 'segment' AND p.key LIKE 'pli:%'
ORDER BY p.key`

type tableLoad struct {
	name string
	load func(context.Context, *sql.DB, func(vals ...any) error) error
}

var tableLoads = []tableLoad{
	{"node", copyNode},
	{"edge", copyEdge},
	{"run", copyRun},
	{"discrepancy", copyDiscrepancy},
	{"segment_pairs", copySegmentPairs},
	{"dpd_entries", copyDpdEntries},
	{"word_occurrences", copyWordOccurrences},
	{"text_stats", copyTextStats},
}

func Rebuild(ctx context.Context, cfg *config.Config) (map[string]int64, error) {
	_, dbPath, err := graph.CurrentDB(cfg)
	if err != nil {
		return nil, fmt.Errorf("export: %w", err)
	}
	sdb, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("export: %w", err)
	}
	defer func() { _ = sdb.Close() }()

	for _, extra := range []string{"", ".wal", ".tmp"} {
		_ = os.Remove(cfg.Canon.DuckDBFile() + extra)
	}
	ddb, err := sql.Open("duckdb", cfg.Canon.DuckDBFile())
	if err != nil {
		return nil, fmt.Errorf("export: %w", err)
	}
	defer func() { _ = ddb.Close() }()
	if _, err := ddb.ExecContext(ctx, ddl); err != nil {
		return nil, fmt.Errorf("export ddl: %w", err)
	}

	counts := map[string]int64{}
	for _, t := range tableLoads {
		n, err := withAppender(ctx, ddb, t.name, func(appendRow func(vals ...any) error) error {
			return t.load(ctx, sdb, appendRow)
		})
		if err != nil {
			return nil, fmt.Errorf("export %s: %w", t.name, err)
		}
		counts[t.name] = n
	}
	if cfg.Canon.Export.Parquet {
		if err := writeParquet(ctx, ddb, cfg); err != nil {
			return nil, err
		}
	}
	return counts, nil
}

func withAppender(ctx context.Context, db *sql.DB, table string, fn func(appendRow func(vals ...any) error) error) (int64, error) {
	conn, err := db.Conn(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = conn.Close() }()
	var ap *duckdb.Appender
	if err := conn.Raw(func(driverConn any) error {
		rc, ok := driverConn.(driver.Conn)
		if !ok {
			return fmt.Errorf("export: raw connection is %T, not driver.Conn", driverConn)
		}
		a, err := duckdb.NewAppenderFromConn(rc, "", table)
		ap = a
		return err
	}); err != nil {
		return 0, err
	}
	n := int64(0)
	err = fn(func(vals ...any) error {
		row := make([]driver.Value, len(vals))
		for i, v := range vals {
			row[i] = v
		}
		if err := ap.AppendRow(row...); err != nil {
			return err
		}
		n++
		return nil
	})
	if err != nil {
		_ = ap.Clear()
		_ = ap.Close()
		return 0, err
	}
	if err := ap.Flush(); err != nil {
		return 0, err
	}
	return n, ap.Close()
}

func copyNode(ctx context.Context, sdb *sql.DB, appendRow func(vals ...any) error) error {
	rows, err := sdb.QueryContext(ctx, `SELECT id, type, key, data FROM node ORDER BY id`)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var id int64
		var typ, key, data string
		if err := rows.Scan(&id, &typ, &key, &data); err != nil {
			return err
		}
		if err := appendRow(id, typ, key, data); err != nil {
			return err
		}
	}
	return rows.Err()
}

func copyEdge(ctx context.Context, sdb *sql.DB, appendRow func(vals ...any) error) error {
	rows, err := sdb.QueryContext(ctx, `SELECT id, src, dst, type, ord, data FROM edge ORDER BY id`)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var id, src, dst int64
		var typ, data string
		var ord sql.NullInt64
		if err := rows.Scan(&id, &src, &dst, &typ, &ord, &data); err != nil {
			return err
		}
		if err := appendRow(id, src, dst, typ, nullInt(ord), data); err != nil {
			return err
		}
	}
	return rows.Err()
}

func copyRun(ctx context.Context, sdb *sql.DB, appendRow func(vals ...any) error) error {
	rows, err := sdb.QueryContext(ctx,
		`SELECT id, bilara_commit, editions_commit, dpd_tag, started, finished FROM run ORDER BY id`)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var id int64
		var bc, ec, tag, started, finished sql.NullString
		if err := rows.Scan(&id, &bc, &ec, &tag, &started, &finished); err != nil {
			return err
		}
		if err := appendRow(id, nullStr(bc), nullStr(ec), nullStr(tag), nullStr(started), nullStr(finished)); err != nil {
			return err
		}
	}
	return rows.Err()
}

func copyDiscrepancy(ctx context.Context, sdb *sql.DB, appendRow func(vals ...any) error) error {
	rows, err := sdb.QueryContext(ctx, `
		SELECT id, run_id, "check", severity, muid, node_id, detail FROM discrepancy ORDER BY id`)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var id, runID int64
		var check, severity, detail string
		var muid sql.NullString
		var nodeID sql.NullInt64
		if err := rows.Scan(&id, &runID, &check, &severity, &muid, &nodeID, &detail); err != nil {
			return err
		}
		if err := appendRow(id, runID, check, severity, nullStr(muid), nullInt(nodeID), detail); err != nil {
			return err
		}
	}
	return rows.Err()
}

func copySegmentPairs(ctx context.Context, sdb *sql.DB, appendRow func(vals ...any) error) error {
	rows, err := sdb.QueryContext(ctx, segmentPairsSQL)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var paliID int64
		var muid, segKey, pali string
		var en, author, html, ref sql.NullString
		if err := rows.Scan(&paliID, &muid, &segKey, &pali, &en, &author, &html, &ref); err != nil {
			return err
		}
		if err := appendRow(paliID, muid, segKey, pali,
			nullStr(en), nullStr(author), nullStr(html), nullStr(ref)); err != nil {
			return err
		}
	}
	return rows.Err()
}

func copyDpdEntries(ctx context.Context, sdb *sql.DB, appendRow func(vals ...any) error) error {
	rows, err := sdb.QueryContext(ctx, `
		SELECT n.id,
			json_extract(n.data, '$.lemma_1'),
			json_extract(n.data, '$.pos'),
			json_extract(n.data, '$.meaning_1'),
			json_extract(n.data, '$.root_key'),
			json_extract(n.data, '$.family_root')
		FROM node n
		WHERE n.type = 'headword'
		ORDER BY n.key`)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var id int64
		var lemma, pos, meaning, root, family sql.NullString
		if err := rows.Scan(&id, &lemma, &pos, &meaning, &root, &family); err != nil {
			return err
		}
		if err := appendRow(id, nullStr(lemma), nullStr(pos), nullStr(meaning), nullStr(root), nullStr(family)); err != nil {
			return err
		}
	}
	return rows.Err()
}

func nullStr(v sql.NullString) any {
	if v.Valid {
		return v.String
	}
	return nil
}

func nullInt(v sql.NullInt64) any {
	if v.Valid {
		return v.Int64
	}
	return nil
}
