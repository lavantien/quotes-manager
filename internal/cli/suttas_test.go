package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/lavantien/quotes-manager/internal/canon/graph"
	"github.com/lavantien/quotes-manager/internal/config"
	"github.com/lavantien/quotes-manager/internal/store"
)

type fixtureGraph struct {
	t   *testing.T
	dir string
	st  *graph.Store
}

func openFixtureGraph(t *testing.T) *fixtureGraph {
	t.Helper()
	dir := t.TempDir()
	st, err := graph.Open(filepath.Join(dir, "canon-test.db"))
	if err != nil {
		t.Fatalf("graph.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return &fixtureGraph{t: t, dir: dir, st: st}
}

func (f *fixtureGraph) addText(muid string, paliSegs, translatedSegs int) {
	f.t.Helper()
	db := f.st.DB()
	var textID int64
	if err := db.QueryRow("INSERT INTO node (type, key) VALUES ('text', ?) RETURNING id", muid).Scan(&textID); err != nil {
		f.t.Fatal(err)
	}
	for i := 1; i <= paliSegs; i++ {
		segKey := segKeyN(i)
		var paliID int64
		if err := db.QueryRow("INSERT INTO node (type, key) VALUES ('segment', ?) RETURNING id", "pli:"+muid+":"+segKey).Scan(&paliID); err != nil {
			f.t.Fatal(err)
		}
		if _, err := db.Exec("INSERT INTO edge (src, dst, type, ord) VALUES (?, ?, 'CHILD_OF', ?)", paliID, textID, i); err != nil {
			f.t.Fatal(err)
		}
		if i <= translatedSegs {
			var enID int64
			if err := db.QueryRow("INSERT INTO node (type, key) VALUES ('segment', ?) RETURNING id", "en/sujato:"+muid+":"+segKey).Scan(&enID); err != nil {
				f.t.Fatal(err)
			}
			if _, err := db.Exec("INSERT INTO edge (src, dst, type) VALUES (?, ?, 'TRANSLATES')", enID, paliID); err != nil {
				f.t.Fatal(err)
			}
		}
	}
}

func segKeyN(i int) string {
	return "1." + string(rune('0'+i))
}

func (f *fixtureGraph) publish(cfg *config.Config) {
	f.t.Helper()
	pointer := `{"current": "test", "path": "` + filepath.ToSlash(filepath.Join(f.dir, "canon-test.db")) + `", "updated": "now"}`
	if err := os.WriteFile(cfg.Canon.DBPointerFile(), []byte(pointer), 0o644); err != nil {
		f.t.Fatal(err)
	}
}

func TestRunSuttasLengthsFillsFromGraph(t *testing.T) {
	fg := openFixtureGraph(t)
	fg.addText("mn26", 3, 3)
	fg.addText("mn27", 2, 1)
	fg.addText("mn28", 4, 0)

	root := t.TempDir()
	quotesPath := filepath.Join(root, "quotes.db")
	st, err := store.Open(quotesPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.Close() }()
	for _, m := range []string{"mn26", "mn27", "mn28", "mn29"} {
		if _, err := st.DB().Exec("INSERT INTO suttas (muid, display, link) VALUES (?, ?, ?)", m, m, "https://suttacentral.net/"+m); err != nil {
			t.Fatal(err)
		}
	}

	cfg := config.Defaults()
	cfg.Canon.Paths.DBDir = t.TempDir()
	fg.publish(cfg)

	oldOut := stdout
	stdout = new(bytes.Buffer)
	err = runSuttas(context.Background(), cfg, []string{"--lengths", "--db", quotesPath})
	out := stdout.(*bytes.Buffer).String()
	stdout = oldOut

	if err != nil {
		t.Fatalf("runSuttas: %v", err)
	}
	if !bytes.Contains([]byte(out), []byte("2 lengths updated of 4")) {
		t.Errorf("summary line wrong: %q", out)
	}
	rows, err := st.ListSuttas()
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]int{}
	for _, s := range rows {
		got[s.Muid] = s.Length
	}
	for muid, want := range map[string]int{"mn26": 3, "mn27": 1, "mn28": 0, "mn29": 0} {
		if got[muid] != want {
			t.Errorf("%s length = %d, want %d", muid, got[muid], want)
		}
	}
}

func TestRunSuttasLengthsNoPublishedDB(t *testing.T) {
	root := t.TempDir()
	quotesPath := filepath.Join(root, "quotes.db")
	st, err := store.Open(quotesPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.Close() }()
	cfg := config.Defaults()
	cfg.Canon.Paths.DBDir = t.TempDir()
	if err := runSuttas(context.Background(), cfg, []string{"--lengths", "--db", quotesPath}); err == nil {
		t.Error("runSuttas must fail when no canon db is published")
	}
}

func TestRunSuttasLengthsMissingQuotesDB(t *testing.T) {
	cfg := config.Defaults()
	cfg.Canon.Paths.DBDir = t.TempDir()
	missing := filepath.Join(t.TempDir(), "gone.db")
	if err := runSuttas(context.Background(), cfg, []string{"--lengths", "--db", missing}); err == nil {
		t.Error("runSuttas must fail when the quotes db is missing")
	}
}

func TestRunSuttasWithoutFlagsPrintsUsage(t *testing.T) {
	oldOut := stdout
	stdout = new(bytes.Buffer)
	err := runSuttas(context.Background(), config.Defaults(), nil)
	out := stdout.(*bytes.Buffer).String()
	stdout = oldOut
	if err != nil {
		t.Fatalf("bare suttas must not fail: %v", err)
	}
	if !bytes.Contains([]byte(out), []byte("--lengths")) {
		t.Errorf("bare suttas must print usage, got %q", out)
	}
}

func suttaFixtureDB(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "suttas.db")
	st, err := store.Open(path)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	cid, err := st.CreateCollection(nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.AddSuttaToCollection(cid, "mn1", "MN 1", "https://suttacentral.net/mn1"); err != nil {
		t.Fatal(err)
	}
	if err := st.AddSuttaToCollection(cid, "mn2", "MN 2", "https://suttacentral.net/mn2"); err != nil {
		t.Fatal(err)
	}
	if err := st.AddSuttaToCollection(cid, "mn3", "MN 3", "https://suttacentral.net/mn3"); err != nil {
		t.Fatal(err)
	}
	if err := st.AddSuttaToCollection(cid, "mn4", "MN 4", "https://suttacentral.net/mn4"); err != nil {
		t.Fatal(err)
	}
	if err := st.SetSuttaLengths(map[string]int{"mn1": 5, "mn3": 2, "mn4": 1}); err != nil {
		t.Fatal(err)
	}
	g1, err := st.CreateCollectionGroup(cid, "First")
	if err != nil {
		t.Fatal(err)
	}
	g2, err := st.CreateCollectionGroup(cid, "Second")
	if err != nil {
		t.Fatal(err)
	}
	for m, g := range map[string]int64{"mn1": g1, "mn3": g1, "mn2": g2, "mn4": g2} {
		if err := st.SetSuttaGroup(cid, m, g); err != nil {
			t.Fatal(err)
		}
	}
	return path
}

func runQuerySuttasOut(t *testing.T, form, db string, collection int64) (string, error) {
	t.Helper()
	oldOut := stdout
	stdout = new(bytes.Buffer)
	err := runQuerySuttas(config.Defaults(), form, db, collection)
	out := stdout.(*bytes.Buffer).String()
	stdout = oldOut
	return out, err
}

func TestQuerySuttasRegistryQM(t *testing.T) {
	out, err := runQuerySuttasOut(t, "", suttaFixtureDB(t), 0)
	if err != nil {
		t.Fatal(err)
	}
	want := "MN 1 (https://suttacentral.net/mn1) 5\nMN 2 (https://suttacentral.net/mn2)\nMN 3 (https://suttacentral.net/mn3) 2\nMN 4 (https://suttacentral.net/mn4) 1\n"
	if out != want {
		t.Errorf("qm registry output = %q, want %q", out, want)
	}
}

func TestQuerySuttasRegistryJSON(t *testing.T) {
	out, err := runQuerySuttasOut(t, "json", suttaFixtureDB(t), 0)
	if err != nil {
		t.Fatal(err)
	}
	var rows []struct {
		Muid    string `json:"muid"`
		Display string `json:"display"`
		Link    string `json:"link"`
		Length  int    `json:"length"`
	}
	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		t.Fatalf("json: %v\n%s", err, out)
	}
	if len(rows) != 4 {
		t.Fatalf("rows = %d, want 4", len(rows))
	}
	if rows[0].Muid != "mn1" || rows[0].Length != 5 || rows[1].Muid != "mn2" || rows[1].Length != 0 {
		t.Errorf("registry order or lengths wrong: %+v", rows)
	}
}

func TestQuerySuttasCollectionQM(t *testing.T) {
	out, err := runQuerySuttasOut(t, "", suttaFixtureDB(t), 1)
	if err != nil {
		t.Fatal(err)
	}
	want := "MN 3 (https://suttacentral.net/mn3) 2\nMN 1 (https://suttacentral.net/mn1) 5\n\nMN 4 (https://suttacentral.net/mn4) 1\nMN 2 (https://suttacentral.net/mn2)\n"
	if out != want {
		t.Errorf("qm collection output = %q, want %q", out, want)
	}
}

func TestQuerySuttasCollectionUngroupedLast(t *testing.T) {
	path := suttaFixtureDB(t)
	st, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.Close() }()
	if err := st.AddSuttaToCollection(1, "mn5", "MN 5", "https://suttacentral.net/mn5"); err != nil {
		t.Fatal(err)
	}
	out, err := runQuerySuttasOut(t, "", path, 1)
	if err != nil {
		t.Fatal(err)
	}
	want := "MN 3 (https://suttacentral.net/mn3) 2\nMN 1 (https://suttacentral.net/mn1) 5\n\nMN 4 (https://suttacentral.net/mn4) 1\nMN 2 (https://suttacentral.net/mn2)\n\nMN 5 (https://suttacentral.net/mn5)\n"
	if out != want {
		t.Errorf("qm collection output = %q, want %q", out, want)
	}
}

func TestQuerySuttasCollectionJSON(t *testing.T) {
	out, err := runQuerySuttasOut(t, "json", suttaFixtureDB(t), 1)
	if err != nil {
		t.Fatal(err)
	}
	var rows []struct {
		Muid      string `json:"muid"`
		Length    int    `json:"length"`
		GroupID   int64  `json:"group_id"`
		GroupName string `json:"group_name"`
		GroupPos  int    `json:"group_position"`
	}
	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		t.Fatalf("json: %v\n%s", err, out)
	}
	wantOrder := []string{"mn3", "mn1", "mn4", "mn2"}
	if len(rows) != len(wantOrder) {
		t.Fatalf("rows = %d, want %d", len(rows), len(wantOrder))
	}
	for i, m := range wantOrder {
		if rows[i].Muid != m {
			t.Fatalf("row %d = %s, want %s", i, rows[i].Muid, m)
		}
	}
	if rows[0].GroupName != "First" || rows[0].GroupPos != 1 || rows[2].GroupName != "Second" || rows[2].GroupPos != 2 {
		t.Errorf("group fields wrong: %+v", rows)
	}
}

func TestQuerySuttasUnknownCollection(t *testing.T) {
	if _, err := runQuerySuttasOut(t, "", suttaFixtureDB(t), 99); err == nil {
		t.Error("unknown collection must error")
	}
}

func TestQuerySuttasMissingDB(t *testing.T) {
	if _, err := runQuerySuttasOut(t, "", filepath.Join(t.TempDir(), "gone.db"), 0); err == nil {
		t.Error("missing db must error")
	}
}

func TestQuerySuttasBadFormat(t *testing.T) {
	if _, err := runQuerySuttasOut(t, "xml", suttaFixtureDB(t), 0); err == nil {
		t.Error("bad format must error")
	}
}
