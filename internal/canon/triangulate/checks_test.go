package triangulate

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lavantien/quotes-manager/internal/canon/model"
	"github.com/lavantien/quotes-manager/internal/config"
	_ "github.com/mattn/go-sqlite3"
)

type testStore struct {
	db       *sql.DB
	ord      map[string]int
	inserted []model.Discrepancy
	covers   []coverCall
	fail     error
}

type coverCall struct {
	muid, editionKey, data string
}

const testSchema = `
CREATE TABLE node (id INTEGER PRIMARY KEY, type TEXT NOT NULL, key TEXT NOT NULL, data TEXT NOT NULL DEFAULT '{}', UNIQUE(type, key));
CREATE TABLE edge (src INTEGER NOT NULL, dst INTEGER NOT NULL, type TEXT NOT NULL, ord INTEGER NOT NULL DEFAULT 0, data TEXT NOT NULL DEFAULT '{}');
CREATE INDEX idx_edge_src ON edge(src, type);
CREATE INDEX idx_edge_dst ON edge(dst, type);
CREATE INDEX idx_node_type_key ON node(type, key);
CREATE TABLE discrepancy (id INTEGER PRIMARY KEY, run_id INTEGER NOT NULL, "check" TEXT NOT NULL, severity TEXT NOT NULL, muid TEXT, node_id INTEGER, detail TEXT NOT NULL);
`

func newTestStore(t *testing.T) *testStore {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Exec(testSchema); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	return &testStore{db: db, ord: map[string]int{}}
}

func (s *testStore) DB() *sql.DB { return s.db }

func (s *testStore) InsertDiscrepancies(ctx context.Context, runID int64, ds []model.Discrepancy) error {
	if s.fail != nil {
		return s.fail
	}
	for _, d := range ds {
		var muid, nodeID any
		if d.Muid != "" {
			muid = d.Muid
		}
		if d.NodeID != 0 {
			nodeID = d.NodeID
		}
		if _, err := s.db.ExecContext(ctx,
			`INSERT INTO discrepancy (run_id, "check", severity, muid, node_id, detail) VALUES (?, ?, ?, ?, ?, ?)`,
			runID, string(d.Check), string(d.Severity), muid, nodeID, d.Detail); err != nil {
			return err
		}
	}
	s.inserted = append(s.inserted, ds...)
	return nil
}

func (s *testStore) SetCoversEdge(ctx context.Context, muid, editionKey, dataJSON string) error {
	if s.fail != nil {
		return s.fail
	}
	s.covers = append(s.covers, coverCall{muid, editionKey, dataJSON})
	return nil
}

func (s *testStore) nodeID(t *testing.T, typ, key, data string) int64 {
	t.Helper()
	if _, err := s.db.Exec(`INSERT INTO node (type, key, data) VALUES (?, ?, ?) ON CONFLICT DO NOTHING`, typ, key, data); err != nil {
		t.Fatalf("insert node %s: %v", key, err)
	}
	var id int64
	if err := s.db.QueryRow(`SELECT id FROM node WHERE type = ? AND key = ?`, typ, key).Scan(&id); err != nil {
		t.Fatalf("node id %s: %v", key, err)
	}
	return id
}

func (s *testStore) addEdge(t *testing.T, src, dst int64, typ string, ord int) {
	t.Helper()
	if _, err := s.db.Exec(`INSERT INTO edge (src, dst, type, ord) VALUES (?, ?, ?, ?)`, src, dst, typ, ord); err != nil {
		t.Fatalf("insert edge %s: %v", typ, err)
	}
}

type segJSON struct {
	Text string `json:"text"`
}

func textData(muid string) string {
	b, _ := json.Marshal(struct {
		FileMuid string `json:"file_muid"`
		FileOrd  int    `json:"file_ord"`
	}{FileMuid: muid, FileOrd: 1})
	return string(b)
}

func segmentData(text string) string {
	b, _ := json.Marshal(segJSON{Text: text})
	return string(b)
}

func (s *testStore) nextOrd(muid string) int {
	s.ord[muid]++
	return s.ord[muid]
}

func (s *testStore) addRoot(t *testing.T, muid, segkey, text string) int64 {
	t.Helper()
	id := s.nodeID(t, "segment", "pli:"+muid+":"+segkey, segmentData(text))
	s.addEdge(t, id, s.nodeID(t, "text", muid, textData(muid)), "CHILD_OF", s.nextOrd(muid))
	return id
}

func (s *testStore) addEn(t *testing.T, author, muid, segkey, text string) {
	t.Helper()
	id := s.nodeID(t, "segment", "en/"+author+":"+muid+":"+segkey, segmentData(text))
	s.addEdge(t, id, s.nodeID(t, "text", muid, textData(muid)), "CHILD_OF", s.nextOrd(muid))
	var rid int64
	if err := s.db.QueryRow(`SELECT id FROM node WHERE type = 'segment' AND key = ?`, "pli:"+muid+":"+segkey).Scan(&rid); err == nil {
		s.addEdge(t, id, rid, "TRANSLATES", 0)
	}
}

func (s *testStore) addCognate(t *testing.T, prefix, muid, segkey string) {
	t.Helper()
	id := s.nodeID(t, "segment", prefix+muid+":"+segkey, segmentData(""))
	s.addEdge(t, id, s.nodeID(t, "text", muid, textData(muid)), "CHILD_OF", s.nextOrd(muid))
}

func writeMeta(t *testing.T, dir string, files map[string]parsedFile) {
	t.Helper()
	b, err := json.Marshal(parsedMeta{Files: files})
	if err != nil {
		t.Fatalf("marshal meta: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "meta.json"), b, 0o644); err != nil {
		t.Fatalf("write meta: %v", err)
	}
}

func findDisc(ds []model.Discrepancy, check model.CheckID, sev model.Severity, substr string) []model.Discrepancy {
	var out []model.Discrepancy
	for _, d := range ds {
		if d.Check != check || d.Severity != sev {
			continue
		}
		if substr == "" || d.Muid == substr || strings.Contains(d.Detail, substr) {
			out = append(out, d)
		}
	}
	return out
}

func TestRunAggregatesAndInserts(t *testing.T) {
	st := newTestStore(t)
	st.addEn(t, "sujato", "mn1", "9.9", "orphan body")
	st.addCognate(t, "html:", "mn1", "8.8")

	sum, err := Run(context.Background(), config.Defaults(), st, Env{})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if !sum.HasErrors {
		t.Fatal("orphan discrepancy must set HasErrors")
	}
	if got := sum.Counts["B001/error"]; got != 1 {
		t.Fatalf("B001/error count = %d, want 1 (counts %v)", got, sum.Counts)
	}
	if got := sum.Counts["B002/warn"]; got != 1 {
		t.Fatalf("B002/warn count = %d, want 1 (counts %v)", got, sum.Counts)
	}
	if got := sum.Counts["E101/info"]; got != 1 {
		t.Fatalf("E101/info degradation note count = %d, want 1 (counts %v)", got, sum.Counts)
	}
	if got := sum.Counts["D002/info"]; got != 1 {
		t.Fatalf("D002/info degradation note count = %d, want 1 (counts %v)", got, sum.Counts)
	}
	if sum.Total != 4 || len(st.inserted) != 4 {
		t.Fatalf("total = %d, inserted = %d, want 4/4", sum.Total, len(st.inserted))
	}
}

func TestRunIsIdempotentPerRun(t *testing.T) {
	st := newTestStore(t)
	st.addEn(t, "sujato", "mn1", "9.9", "orphan body")
	env := Env{RunID: 7}
	for range 2 {
		if _, err := Run(context.Background(), config.Defaults(), st, env); err != nil {
			t.Fatal(err)
		}
	}
	var n int
	if err := st.db.QueryRow(`SELECT COUNT(*) FROM discrepancy WHERE run_id = 7`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	var want int
	if err := st.db.QueryRow(`SELECT COUNT(*) FROM discrepancy`).Scan(&want); err != nil {
		t.Fatal(err)
	}
	if n != want || want == 0 {
		t.Errorf("discrepancy rows = %d (run 7 = %d); reruns must not append", want, n)
	}
}

func TestRunCheckFailureBecomesDiscrepancy(t *testing.T) {
	st := newTestStore(t)
	st.addEn(t, "sujato", "mn1", "9.9", "orphan body")
	env := Env{RunID: 3, ParsedDir: filepath.Join(t.TempDir(), "missing")}

	sum, err := Run(context.Background(), config.Defaults(), st, env)
	if err != nil {
		t.Fatalf("run must survive a failing check: %v", err)
	}
	if !sum.HasErrors {
		t.Fatal("check failure must surface as error severity")
	}
	runFailed := 0
	for _, d := range st.inserted {
		if d.Severity == model.Error && strings.HasPrefix(d.Detail, "check run failed") {
			runFailed++
		}
	}
	if runFailed == 0 {
		t.Fatalf("no 'check run failed' discrepancies in %v", st.inserted)
	}
	if got := sum.Counts["B001/error"]; got != 1 {
		t.Errorf("B001 orphan must still run: counts %v", sum.Counts)
	}
}

func TestRunInsertFailurePropagates(t *testing.T) {
	st := newTestStore(t)
	st.fail = errors.New("boom")
	_, err := Run(context.Background(), config.Defaults(), st, Env{})
	if err == nil || !strings.Contains(err.Error(), "insert discrepancies") {
		t.Fatalf("want wrapped insert error, got %v", err)
	}
}

func TestRunEmptyEnvDegrades(t *testing.T) {
	st := newTestStore(t)
	sum, err := Run(context.Background(), config.Defaults(), st, Env{})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if sum.HasErrors {
		t.Fatal("empty env must not produce errors")
	}
	if sum.Total != 2 || sum.Counts["E101/info"] != 1 || sum.Counts["D002/info"] != 1 {
		t.Fatalf("empty env total = %d, want exactly the E101 and D002 notes (counts %v)", sum.Total, sum.Counts)
	}
}

func TestRunNilStoreFails(t *testing.T) {
	if _, err := Run(context.Background(), config.Defaults(), nil, Env{}); err == nil {
		t.Fatal("nil store must error")
	}
}

func TestDirectionSeverity(t *testing.T) {
	cfg := config.Defaults()
	if got := directionSeverity(cfg, "B001.orphan", model.Error); got != model.Error {
		t.Fatalf("default orphan = %s, want error", got)
	}
	cfg.Canon.Triangulate.Severity["B001.orphan"] = string(model.Warn)
	if got := directionSeverity(cfg, "B001.orphan", model.Error); got != model.Warn {
		t.Fatalf("explicit orphan override = %s, want warn", got)
	}
	cfg2 := config.Defaults()
	cfg2.Canon.Triangulate.Severity[string(model.CheckKeysetRootTranslation)] = string(model.Error)
	if got := directionSeverity(cfg2, "B001.orphan", model.Error); got != model.Error {
		t.Fatalf("base-only override still = %s, want error", got)
	}
}

func TestCapJoin(t *testing.T) {
	if got := capJoin([]string{"a", "b"}, 10); got != "a, b" {
		t.Fatalf("fitting join = %q, got %q", "a, b", got)
	}
	got := capJoin([]string{"mn1:1.1", "mn1:2.2", "mn1:3.3"}, 10)
	if strings.Contains(got, "mn1:2.2") || !strings.Contains(got, "(+2 more)") {
		t.Fatalf("capped join = %q", got)
	}
	if got := capJoin([]string{"abcdefghij", "x"}, 10); got != "abcdefghij (+1 more)" {
		t.Fatalf("boundary join = %q", got)
	}
}

func TestRunNilCfgDefaults(t *testing.T) {
	st := newTestStore(t)
	st.addEn(t, "sujato", "mn1", "9.9", "orphan body")
	sum, err := Run(context.Background(), nil, st, Env{})
	if err != nil {
		t.Fatalf("run with nil cfg must default, got %v", err)
	}
	if !sum.HasErrors {
		t.Fatal("default orphan severity is error, HasErrors must be set")
	}
}
