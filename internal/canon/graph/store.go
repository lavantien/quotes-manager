package graph

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/lavantien/quotes-manager/internal/canon/model"

	_ "github.com/mattn/go-sqlite3"
)

type Store struct {
	db *sql.DB
}

func Open(path string) (*Store, error) {
	if strings.ContainsRune(path, '?') {
		return nil, fmt.Errorf("open sqlite: path %q contains %q, unusable as a driver DSN", path, "?")
	}
	dsn := path + "?_foreign_keys=on&_journal_mode=WAL&_synchronous=NORMAL"
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite %s: %w", path, err)
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("open sqlite %s: %w", path, err)
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) DB() *sql.DB { return s.db }

type Loader struct {
	s         *Store
	batchSize int
	tx        *sql.Tx
	pending   int
}

func NewLoader(s *Store, batchSize int) *Loader {
	return &Loader{s: s, batchSize: batchSize}
}

func (l *Loader) begin() error {
	if l.tx != nil {
		return nil
	}
	tx, err := l.s.db.Begin()
	if err != nil {
		return err
	}
	l.tx = tx
	return nil
}

func (l *Loader) tick() error {
	l.pending++
	if l.pending < l.batchSize {
		return nil
	}
	return l.Flush()
}

func (l *Loader) abort() {
	l.pending = 0
	if l.tx != nil {
		_ = l.tx.Rollback()
		l.tx = nil
	}
}

func (l *Loader) Flush() error {
	if l.tx == nil {
		l.pending = 0
		return nil
	}
	err := l.tx.Commit()
	l.tx = nil
	l.pending = 0
	if err != nil {
		return fmt.Errorf("commit batch: %w", err)
	}
	return nil
}

func orEmptyJSON(dataJSON string) string {
	if dataJSON == "" {
		return "{}"
	}
	return dataJSON
}

func (l *Loader) EnsureNode(typ model.NodeType, key, dataJSON string) (int64, error) {
	if err := l.begin(); err != nil {
		return 0, err
	}
	var id int64
	err := l.tx.QueryRow(
		`INSERT INTO node (type, key, data) VALUES (?, ?, ?)
		 ON CONFLICT (type, key) DO UPDATE SET data = excluded.data
		 RETURNING id`,
		string(typ), key, orEmptyJSON(dataJSON),
	).Scan(&id)
	if err != nil {
		l.abort()
		return 0, fmt.Errorf("ensure node %s/%s: %w", typ, key, err)
	}
	if err := l.tick(); err != nil {
		return 0, err
	}
	return id, nil
}

func (l *Loader) AddEdge(src, dst int64, typ model.EdgeType, ord *int64, dataJSON string) error {
	if err := l.begin(); err != nil {
		return err
	}
	if _, err := l.tx.Exec(
		`INSERT INTO edge (src, dst, type, ord, data) VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT (src, dst, type) DO UPDATE SET ord = excluded.ord, data = excluded.data`,
		src, dst, string(typ), ord, orEmptyJSON(dataJSON),
	); err != nil {
		l.abort()
		return fmt.Errorf("add edge %d-%s->%d: %w", src, typ, dst, err)
	}
	return l.tick()
}

func (l *Loader) SetFTS(paliNodeID int64, pali, en string) error {
	if err := l.begin(); err != nil {
		return err
	}
	if _, err := l.tx.Exec(
		`INSERT INTO fts_segment (rowid, pali, en) VALUES (?, ?, ?)`,
		paliNodeID, pali, en,
	); err != nil {
		l.abort()
		return fmt.Errorf("set fts row %d: %w", paliNodeID, err)
	}
	return l.tick()
}

func (s *Store) NewRun(bilaraCommit, editionsCommit, dpdTag string) (int64, error) {
	var tag any
	if dpdTag != "" {
		tag = dpdTag
	}
	res, err := s.db.Exec(
		`INSERT INTO run (bilara_commit, editions_commit, dpd_tag, started) VALUES (?, ?, ?, ?)`,
		bilaraCommit, editionsCommit, tag, time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		return 0, fmt.Errorf("new run: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("new run id: %w", err)
	}
	return id, nil
}

func (s *Store) FinishRun(runID int64) error {
	res, err := s.db.Exec(
		`UPDATE run SET finished = ? WHERE id = ?`,
		time.Now().UTC().Format(time.RFC3339), runID,
	)
	if err != nil {
		return fmt.Errorf("finish run %d: %w", runID, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("finish run %d: no such run", runID)
	}
	return nil
}

func (s *Store) InsertDiscrepancies(ctx context.Context, runID int64, ds []model.Discrepancy) error {
	if len(ds) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	stmt, err := tx.PrepareContext(ctx,
		`INSERT INTO discrepancy (run_id, "check", severity, muid, node_id, detail)
		 VALUES (?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer func() { _ = stmt.Close() }()
	for _, d := range ds {
		var muid, nodeID any
		if d.Muid != "" {
			muid = d.Muid
		}
		if d.NodeID != 0 {
			nodeID = d.NodeID
		}
		if _, err := stmt.ExecContext(ctx, runID, string(d.Check), string(d.Severity), muid, nodeID, d.Detail); err != nil {
			return fmt.Errorf("insert discrepancy %s: %w", d.Check, err)
		}
	}
	return tx.Commit()
}

func (s *Store) SetCoversEdge(ctx context.Context, muid, editionKey, dataJSON string) error {
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO edge (src, dst, type, ord, data)
		 SELECT t.id, e.id, 'COVERED_BY', NULL, ?
		 FROM node t, node e
		 WHERE t.type = 'text' AND t.key = ? AND e.type = 'edition' AND e.key = ?
		 ON CONFLICT (src, dst, type) DO UPDATE SET data = excluded.data`,
		orEmptyJSON(dataJSON), muid, editionKey,
	)
	if err != nil {
		return fmt.Errorf("covers edge %s->%s: %w", muid, editionKey, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("covers edge %s->%s: text or edition node missing", muid, editionKey)
	}
	return nil
}
