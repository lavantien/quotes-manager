package graph

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/lavantien/quotes-manager/internal/canon/model"
)

func TestSegmentsWithEnHonorsLimit(t *testing.T) {
	s := openTempStore(t)
	l := newTestLoader(t, s, 100)
	text, err := l.EnsureNode(model.NodeText, "m1", "{}")
	if err != nil {
		t.Fatal(err)
	}
	mustLoadSegments(t, l, text, []segIn{
		{"pli:m1:0.1", "sabbe", "", ""},
		{"pli:m1:0.2", "atthi", "en/sujato:m1:0.2", "all"},
	})
	if err := l.Flush(); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	got, err := s.Segments(ctx, "m1", true, 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].SegKey != "0.2" || got[0].En == "" {
		t.Fatalf("withEn=true limit=1 = %+v, want the single translated row", got)
	}
}

func TestSegmentsSingleRowPerSegment(t *testing.T) {
	s := openTempStore(t)
	l := newTestLoader(t, s, 100)
	text, err := l.EnsureNode(model.NodeText, "m1", "{}")
	if err != nil {
		t.Fatal(err)
	}
	mustLoadSegments(t, l, text, []segIn{
		{"pli:m1:0.1", "sabbe", "en/sujato:m1:0.1", "all"},
	})
	if err := l.Flush(); err != nil {
		t.Fatal(err)
	}
	eid, err := l.EnsureNode(model.NodeSegment, "en/brahmali:m1:0.1", `{"text":"all (b)"}`)
	if err != nil {
		t.Fatal(err)
	}
	var pid int64
	if err := s.DB().QueryRow(
		`SELECT id FROM node WHERE key = 'pli:m1:0.1'`).Scan(&pid); err != nil {
		t.Fatal(err)
	}
	if err := l.AddEdge(eid, pid, model.EdgeTranslates, nil, "{}"); err != nil {
		t.Fatal(err)
	}
	if err := l.Flush(); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	got, err := s.Segments(ctx, "m1", false, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("segments = %+v, want 1 row despite two translators", got)
	}
	if got[0].Author != "sujato" || got[0].En != "all" {
		t.Fatalf("canonical partner = %+v, want sujato (lowest node id)", got[0])
	}
	hits, err := s.SearchFTS(ctx, "sabbe", "pali", 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 {
		t.Fatalf("search rows = %+v, want 1 per segment", hits)
	}
}

func TestTreeDiamondReturnsNodeOnce(t *testing.T) {
	s := openTempStore(t)
	l := newTestLoader(t, s, 100)
	div, err := l.EnsureNode(model.NodeDivision, "d", "{}")
	if err != nil {
		t.Fatal(err)
	}
	x, err := l.EnsureNode(model.NodeText, "x", "{}")
	if err != nil {
		t.Fatal(err)
	}
	y, err := l.EnsureNode(model.NodeText, "y", "{}")
	if err != nil {
		t.Fatal(err)
	}
	if err := l.AddEdge(x, div, model.EdgeChildOf, ptr(int64(0)), "{}"); err != nil {
		t.Fatal(err)
	}
	if err := l.AddEdge(y, div, model.EdgeChildOf, ptr(int64(1)), "{}"); err != nil {
		t.Fatal(err)
	}
	z, err := l.EnsureNode(model.NodeSegment, "pli:x:0.1", `{"text":"z"}`)
	if err != nil {
		t.Fatal(err)
	}
	o0, o5 := int64(0), int64(5)
	if err := l.AddEdge(z, x, model.EdgeChildOf, &o0, "{}"); err != nil {
		t.Fatal(err)
	}
	if err := l.AddEdge(z, y, model.EdgeChildOf, &o5, "{}"); err != nil {
		t.Fatal(err)
	}
	if err := l.Flush(); err != nil {
		t.Fatal(err)
	}

	all, err := s.Tree(context.Background(), "d", "all", 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 4 {
		t.Fatalf("tree = %+v, want 4 distinct nodes", all)
	}
	var zNode *TreeNode
	for i := range all {
		if all[i].Key == "pli:x:0.1" {
			zNode = &all[i]
		}
	}
	if zNode == nil || zNode.Depth != 2 || zNode.Ord == nil || *zNode.Ord != 0 {
		t.Fatalf("diamond node = %+v, want depth 2 and min ord 0", zNode)
	}
}

func TestTreeNegativeOrdOrdering(t *testing.T) {
	s := openTempStore(t)
	l := newTestLoader(t, s, 100)
	div, err := l.EnsureNode(model.NodeDivision, "d", "{}")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		key string
		ord int64
	}{
		{"zzz", -5}, {"aaa", -3}, {"mmm", 3}, {"bbb", 5},
	} {
		id, err := l.EnsureNode(model.NodeText, tc.key, "{}")
		if err != nil {
			t.Fatal(err)
		}
		if err := l.AddEdge(id, div, model.EdgeChildOf, ptr(tc.ord), "{}"); err != nil {
			t.Fatal(err)
		}
	}
	if err := l.Flush(); err != nil {
		t.Fatal(err)
	}
	all, err := s.Tree(context.Background(), "d", "text", 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"zzz", "aaa", "mmm", "bbb"}
	for i, k := range want {
		if all[i].Key != k {
			t.Fatalf("order[%d] = %s, want %s (%v)", i, all[i].Key, k, all)
		}
	}
}

func TestTreeCycleTerminates(t *testing.T) {
	s := openTempStore(t)
	l := newTestLoader(t, s, 100)
	div, err := l.EnsureNode(model.NodeDivision, "d", "{}")
	if err != nil {
		t.Fatal(err)
	}
	a, err := l.EnsureNode(model.NodeText, "a", "{}")
	if err != nil {
		t.Fatal(err)
	}
	b, err := l.EnsureNode(model.NodeText, "b", "{}")
	if err != nil {
		t.Fatal(err)
	}
	if err := l.AddEdge(a, div, model.EdgeChildOf, ptr(int64(0)), "{}"); err != nil {
		t.Fatal(err)
	}
	if err := l.AddEdge(b, div, model.EdgeChildOf, ptr(int64(1)), "{}"); err != nil {
		t.Fatal(err)
	}
	if err := l.AddEdge(a, b, model.EdgeChildOf, nil, "{}"); err != nil {
		t.Fatal(err)
	}
	if err := l.AddEdge(b, a, model.EdgeChildOf, nil, "{}"); err != nil {
		t.Fatal(err)
	}
	if err := l.Flush(); err != nil {
		t.Fatal(err)
	}
	all, err := s.Tree(context.Background(), "d", "all", 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 3 {
		t.Fatalf("tree over cycle = %+v, want 3 distinct nodes", all)
	}
	for _, n := range all {
		if n.Depth > 1 {
			t.Fatalf("node %s depth = %d, want minimum depth <= 1", n.Key, n.Depth)
		}
	}
}

func TestTreeLayeredDAGIsBounded(t *testing.T) {
	s := openTempStore(t)
	l := newTestLoader(t, s, 64)
	div, err := l.EnsureNode(model.NodeDivision, "d", "{}")
	if err != nil {
		t.Fatal(err)
	}
	const layers, width = 21, 2
	ids := make([][]int64, layers)
	for i := range layers {
		for j := range width {
			id, err := l.EnsureNode(model.NodeText, fmt.Sprintf("L%02dN%d", i, j), "{}")
			if err != nil {
				t.Fatal(err)
			}
			ids[i] = append(ids[i], id)
		}
	}
	for j, id := range ids[0] {
		if err := l.AddEdge(id, div, model.EdgeChildOf, ptr(int64(j)), "{}"); err != nil {
			t.Fatal(err)
		}
	}
	for i := 1; i < layers; i++ {
		for _, child := range ids[i] {
			for _, parent := range ids[i-1] {
				if err := l.AddEdge(child, parent, model.EdgeChildOf, nil, "{}"); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	if err := l.Flush(); err != nil {
		t.Fatal(err)
	}
	all, err := s.Tree(context.Background(), "d", "all", 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != layers*width+1 {
		t.Fatalf("tree rows = %d, want %d (each node once plus the division)", len(all), layers*width+1)
	}
}

func TestOpenRejectsQuestionMarkPath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("'?' is not a legal filename character on windows")
	}
	if _, err := Open(filepath.Join(t.TempDir(), "a?b.db")); err == nil {
		t.Fatal("path containing '?' must be rejected")
	}
}

func TestPublishRetirementFailure(t *testing.T) {
	cfg := tempConfig(t)
	dir := cfg.Canon.Paths.DBDir
	oldPath := touchDB(t, dir, "canon-old.db")
	oldStore, err := Open(oldPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = oldStore.Close() }()
	if err := PublishDB(cfg, "old", oldPath); err != nil {
		t.Fatal(err)
	}
	cur := touchDB(t, dir, "canon-new.db")

	err = PublishDB(cfg, "new", cur)
	if runtime.GOOS == "windows" {
		if err == nil {
			t.Fatal("open handle on the old db must make retirement fail on windows")
		}
		if !strings.Contains(err.Error(), "pointer now references") {
			t.Fatalf("error must state the pointer advanced: %v", err)
		}
	} else if err != nil {
		t.Fatalf("posix removes open files: %v", err)
	}
	runID, path, err := CurrentDB(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if runID != "new" || path != cur {
		t.Fatalf("pointer = %q,%q, want new", runID, path)
	}
	if _, err := os.Stat(cur); err != nil {
		t.Fatalf("new db must survive: %v", err)
	}
}

type segIn struct {
	key, pali    string
	enKey, enTxt string
}

func mustLoadSegments(t *testing.T, l *Loader, text int64, segs []segIn) {
	t.Helper()
	for i, sg := range segs {
		pid, err := l.EnsureNode(model.NodeSegment, sg.key, `{"text":"`+sg.pali+`"}`)
		if err != nil {
			t.Fatal(err)
		}
		if err := l.AddEdge(pid, text, model.EdgeChildOf, ptr(int64(i)), "{}"); err != nil {
			t.Fatal(err)
		}
		if err := l.SetFTS(pid, sg.pali, ""); err != nil {
			t.Fatal(err)
		}
		if sg.enKey != "" {
			eid, err := l.EnsureNode(model.NodeSegment, sg.enKey, `{"text":"`+sg.enTxt+`"}`)
			if err != nil {
				t.Fatal(err)
			}
			if err := l.AddEdge(eid, pid, model.EdgeTranslates, nil, "{}"); err != nil {
				t.Fatal(err)
			}
		}
	}
}
