package cli

import (
	"bytes"
	"context"
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
