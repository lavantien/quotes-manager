package graph

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/lavantien/quotes-manager/internal/canon/model"
	"github.com/lavantien/quotes-manager/internal/config"
)

func hasNUL(s string) bool { return strings.ContainsRune(s, 0) }

func FuzzEnsureNode(f *testing.F) {
	cases := [][2]string{
		{"mn1", `{"text":"x"}`},
		{"", `{}`},
		{"a:b:c", ""},
		{"quote\"key", "not json"},
		{"pariyāya", `{"v":1}`},
		{"pli:mn1:1.1", `{"nested":{"a":[1,2]}}`},
		{"canon-*.db", `{"path":"x"}`},
		{"'; DROP TABLE node;--", `{}`},
	}
	for _, c := range cases {
		f.Add(c[0], c[1])
	}
	f.Fuzz(func(t *testing.T, key, dataJSON string) {
		if hasNUL(key) || hasNUL(dataJSON) {
			return
		}
		s := openTempStore(t)
		l := newTestLoader(t, s, 4)
		id1, err1 := l.EnsureNode(model.NodeText, key, dataJSON)
		id2, err2 := l.EnsureNode(model.NodeText, key, dataJSON)
		if (err1 == nil) != (err2 == nil) {
			t.Fatalf("nondeterministic ensure: %v vs %v", err1, err2)
		}
		if err1 == nil && id1 != id2 {
			t.Fatalf("ids diverged: %d vs %d", id1, id2)
		}
		if err := l.Flush(); err != nil {
			t.Fatal(err)
		}
		if err1 == nil {
			var n int
			if err := s.DB().QueryRow(
				`SELECT COUNT(*) FROM node WHERE type = 'text' AND key = ?`, key,
			).Scan(&n); err != nil {
				t.Fatal(err)
			}
			if n != 1 {
				t.Fatalf("key %q stored %d rows, want 1", key, n)
			}
			var data string
			if err := s.DB().QueryRow(
				`SELECT data FROM node WHERE type = 'text' AND key = ?`, key,
			).Scan(&data); err != nil {
				t.Fatal(err)
			}
			if want := orEmptyJSON(dataJSON); data != want {
				t.Fatalf("data = %q, want %q", data, want)
			}
		}
	})
}

func FuzzSearchFTSText(f *testing.F) {
	for _, c := range []string{
		"pariyaya", "pariyāya", "", "the basis", `said "hi"`, "a OR b",
		"NEAR(x y)", "*", "^start", "col:*", "en:pali", "-", "\"",
		"'; DROP TABLE fts_segment;--", "छन्दो",
	} {
		f.Add(c)
	}
	f.Fuzz(func(t *testing.T, text string) {
		if hasNUL(text) {
			return
		}
		if strings.TrimSpace(text) == "" {
			return
		}
		s := loadFixture(t)
		for _, lang := range []string{"pali", "en"} {
			got, err := s.SearchFTS(context.Background(), text, lang, 10, 0)
			if err != nil {
				t.Fatalf("search %q/%s: %v", text, lang, err)
			}
			for _, p := range got {
				if p.Muid == "" || p.SegKey == "" {
					t.Fatalf("incomplete pair %+v for %q", p, text)
				}
			}
		}
		if _, err := s.SearchFTS(context.Background(), text, "fr", 10, 0); err == nil && strings.TrimSpace(text) != "" {
			t.Fatal("bad lang must error")
		}
	})
}

func FuzzSegmentsMuid(f *testing.F) {
	for _, c := range []string{"mn1", "mn10", "", "pli:mn1", "a:b", "'", "%_"} {
		f.Add(c)
	}
	f.Fuzz(func(t *testing.T, muid string) {
		if hasNUL(muid) {
			return
		}
		s := loadFixture(t)
		got, err := s.Segments(context.Background(), muid, false, 0, 0)
		if err != nil {
			t.Fatalf("segments %q: %v", muid, err)
		}
		for _, p := range got {
			if p.Muid != muid {
				t.Fatalf("muid bleed: %+v for query %q", p, muid)
			}
		}
	})
}

func FuzzTreeDivisionKey(f *testing.F) {
	for _, c := range []string{"mn-div", "", "'; --", "pli:mn1:0.1", "%"} {
		f.Add(c)
	}
	f.Fuzz(func(t *testing.T, divisionKey string) {
		if hasNUL(divisionKey) {
			return
		}
		s := loadFixture(t)
		for _, types := range []string{"all", "segment", "text"} {
			nodes, err := s.Tree(context.Background(), divisionKey, types, 5, 0)
			if err != nil {
				t.Fatalf("tree %q/%s: %v", divisionKey, types, err)
			}
			if len(nodes) > 5 {
				t.Fatalf("limit ignored: %d rows", len(nodes))
			}
		}
	})
}

func FuzzPublishPointer(f *testing.F) {
	for _, c := range []string{"run1", "", "é", `quote"`, "../../etc", "\xc3"} {
		f.Add(c)
	}
	f.Fuzz(func(t *testing.T, runID string) {
		if hasNUL(runID) {
			return
		}
		cfg := config.Defaults()
		cfg.Canon.Paths.DBDir = t.TempDir()
		db := touchDB(t, cfg.Canon.Paths.DBDir, "canon-fuzz.db")
		err := PublishDB(cfg, runID, db)
		if !utf8.ValidString(runID) {
			if err == nil {
				t.Fatal("invalid UTF-8 run id must be rejected")
			}
			return
		}
		if err != nil {
			t.Fatalf("publish %q: %v", runID, err)
		}
		gotID, gotPath, err := CurrentDB(cfg)
		if err != nil {
			t.Fatal(err)
		}
		if gotID != runID || gotPath != db {
			t.Fatalf("pointer round trip = %q,%q, want %q", gotID, gotPath, runID)
		}
	})
}

func TestEndToEndFlow(t *testing.T) {
	cfg := config.Defaults()
	cfg.Canon.Paths.DBDir = t.TempDir()
	dbPath := cfg.Canon.DBFile("e2e")
	if filepath.Base(dbPath) != "canon-e2e.db" {
		t.Fatalf("db file %q", dbPath)
	}

	s, err := Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Migrate(); err != nil {
		t.Fatal(err)
	}
	runID, err := s.NewRun("bilara-e2e", "editions-e2e", "")
	if err != nil {
		t.Fatal(err)
	}
	l := NewLoader(s, 3)
	ord := func(v int64) *int64 { return &v }
	col, err := l.EnsureNode(model.NodeCollection, "mn", "{}")
	if err != nil {
		t.Fatal(err)
	}
	div, err := l.EnsureNode(model.NodeDivision, "mn-div", "{}")
	if err != nil {
		t.Fatal(err)
	}
	if err := l.AddEdge(div, col, model.EdgeChildOf, ord(0), "{}"); err != nil {
		t.Fatal(err)
	}
	text, err := l.EnsureNode(model.NodeText, "mn1", "{}")
	if err != nil {
		t.Fatal(err)
	}
	if err := l.AddEdge(text, div, model.EdgeChildOf, ord(0), "{}"); err != nil {
		t.Fatal(err)
	}
	var lastSeg int64
	for i, pali := range []string{"pariyāya", "ekodhāna"} {
		pid, err := l.EnsureNode(model.NodeSegment, "pli:mn1:0."+string(rune('1'+i)), `{"text":"`+pali+`"}`)
		if err != nil {
			t.Fatal(err)
		}
		lastSeg = pid
		if err := l.AddEdge(pid, text, model.EdgeChildOf, ord(int64(i)), "{}"); err != nil {
			t.Fatal(err)
		}
		if err := l.SetFTS(pid, pali, ""); err != nil {
			t.Fatal(err)
		}
	}
	if err := l.Flush(); err != nil {
		t.Fatal(err)
	}
	if err := s.SetCoversEdge(context.Background(), "mn1", "sujato/en/mn", `{"tex":"mn.tex"}`); err == nil {
		t.Fatal("edition node missing: covers edge must fail")
	}
	if _, err := l.EnsureNode(model.NodeEdition, "sujato/en/mn", "{}"); err != nil {
		t.Fatal(err)
	}
	if err := l.Flush(); err != nil {
		t.Fatal(err)
	}
	if err := s.SetCoversEdge(context.Background(), "mn1", "sujato/en/mn", `{"tex":"mn.tex"}`); err != nil {
		t.Fatal(err)
	}
	if err := s.InsertDiscrepancies(context.Background(), runID, []model.Discrepancy{
		{Check: model.CheckEmptyValue, Severity: model.Info, Muid: "mn1", Detail: "e2e"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.FinishRun(runID); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	if err := PublishDB(cfg, "e2e", dbPath); err != nil {
		t.Fatal(err)
	}
	gotRun, gotPath, err := CurrentDB(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if gotRun != "e2e" || gotPath != dbPath {
		t.Fatalf("pointer = %q,%q", gotRun, gotPath)
	}

	s2, err := Open(gotPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s2.Close() }()
	segs, err := s2.Segments(context.Background(), "mn1", false, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(segs) != 2 || segs[0].Pali != "pariyāya" || segs[0].En != "" {
		t.Fatalf("segments = %+v", segs)
	}
	tree, err := s2.Tree(context.Background(), "mn-div", "all", 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(tree) != 4 {
		t.Fatalf("tree = %+v", tree)
	}
	stats, err := s2.Stats(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if stats["node:segment"] != 2 || stats["edge:COVERED_BY"] != 1 {
		t.Fatalf("stats = %v", stats)
	}
	var finished string
	if err := s2.DB().QueryRow(`SELECT finished FROM run WHERE id = ?`, runID).Scan(&finished); err != nil {
		t.Fatal(err)
	}
	if finished == "" {
		t.Fatal("run not finished")
	}
	_ = lastSeg
}
