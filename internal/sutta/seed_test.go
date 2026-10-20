package sutta

import (
	"os"
	"strings"
	"testing"
)

func TestRenderSeedSQLSmallExact(t *testing.T) {
	tables := []Table{{
		Name: "a.md",
		Groups: []Group{
			{Name: "One's", Suttas: []Sutta{{Muid: "mn1", Display: "MN 1", Link: "https://suttacentral.net/mn1"}}},
			{Name: "Two", Suttas: []Sutta{
				{Muid: "sn3.4", Display: "SN 3.4", Link: "l"},
				{Muid: "an5", Display: "AN 5", Link: "l2"},
			}},
		},
	}}
	want := "DELETE FROM suttas;\n" +
		"DELETE FROM collection_sutta_items;\n" +
		"DELETE FROM collection_groups;\n" +
		"DELETE FROM collections;\n" +
		"INSERT INTO suttas (muid, display, link) VALUES ('mn1', 'MN 1', 'https://suttacentral.net/mn1');\n" +
		"INSERT INTO suttas (muid, display, link) VALUES ('sn3.4', 'SN 3.4', 'l');\n" +
		"INSERT INTO suttas (muid, display, link) VALUES ('an5', 'AN 5', 'l2');\n" +
		"INSERT INTO collections (id, name) VALUES (1, 'Ariya''s List');\n" +
		"INSERT INTO collection_groups (id, collection_id, name, position) VALUES (1, 1, 'One''s', 1);\n" +
		"INSERT INTO collection_groups (id, collection_id, name, position) VALUES (2, 1, 'Two', 2);\n" +
		"INSERT INTO collection_sutta_items (collection_id, muid, position, group_id) VALUES (1, 'mn1', 1, 1);\n" +
		"INSERT INTO collection_sutta_items (collection_id, muid, position, group_id) VALUES (1, 'sn3.4', 2, 2);\n" +
		"INSERT INTO collection_sutta_items (collection_id, muid, position, group_id) VALUES (1, 'an5', 3, 2);\n"
	if got := RenderSeedSQL(tables, "Ariya's List"); got != want {
		t.Errorf("RenderSeedSQL mismatch:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestRenderSeedSQLCrossTableDedup(t *testing.T) {
	tables := []Table{
		{
			Name:   "a.md",
			Groups: []Group{{Name: "A", Suttas: []Sutta{{Muid: "mn1", Display: "MN 1", Link: "first"}}}},
		},
		{
			Name: "b.md",
			Groups: []Group{
				{Name: "B", Suttas: []Sutta{{Muid: "mn1", Display: "MN 1", Link: "second"}, {Muid: "mn2", Display: "MN 2", Link: "l"}}},
			},
		},
	}
	out := RenderSeedSQL(tables, "C")
	if n := strings.Count(out, "'mn1'"); n != 2 {
		t.Errorf("mn1 appears %d times, want 2 (one registry row, one item row)", n)
	}
	if !strings.Contains(out, "('mn1', 'MN 1', 'first')") {
		t.Errorf("registry keeps the first-seen row, got:\n%s", out)
	}
	if strings.Contains(out, "second") {
		t.Errorf("later duplicate must be dropped, got:\n%s", out)
	}
	if !strings.Contains(out, "VALUES (2, 1, 'B', 2)") {
		t.Errorf("group ids continue across tables, got:\n%s", out)
	}
	if !strings.Contains(out, "(1, 'mn2', 2, 2)") {
		t.Errorf("positions stay global across tables, got:\n%s", out)
	}
}

func TestRenderSeedSQLEmptyTables(t *testing.T) {
	out := RenderSeedSQL(nil, "Empty")
	if !strings.HasPrefix(out, "DELETE FROM suttas;\nDELETE FROM collection_sutta_items;\nDELETE FROM collection_groups;\nDELETE FROM collections;\n") {
		t.Errorf("output must open with the four deletes, got:\n%s", out)
	}
	if !strings.Contains(out, "INSERT INTO collections (id, name) VALUES (1, 'Empty');") {
		t.Errorf("collection row must always render, got:\n%s", out)
	}
}

func TestRenderSeedSQLRealArtifactCounts(t *testing.T) {
	data, err := os.ReadFile(realDumpPath)
	if err != nil {
		t.Fatalf("read artifact: %v", err)
	}
	tables, err := ParseFiles([]File{{Name: "ariyasavaka-curated-list.md", Content: string(data)}})
	if err != nil {
		t.Fatalf("ParseFiles: %v", err)
	}
	out := RenderSeedSQL(tables, "AriyaSavaka")
	if n := strings.Count(out, "INSERT INTO suttas "); n != 471 {
		t.Errorf("suttas inserts = %d, want 471", n)
	}
	if n := strings.Count(out, "INSERT INTO collections "); n != 1 {
		t.Errorf("collections inserts = %d, want 1", n)
	}
	if n := strings.Count(out, "INSERT INTO collection_groups "); n != 16 {
		t.Errorf("groups inserts = %d, want 16", n)
	}
	if n := strings.Count(out, "INSERT INTO collection_sutta_items "); n != 471 {
		t.Errorf("items inserts = %d, want 471", n)
	}
	if !strings.Contains(out, "(1, 'AriyaSavaka')") {
		t.Errorf("collection name missing:\n%s", out[:400])
	}
	if !strings.Contains(out, "(1, 'mn26', 1, 1)") {
		t.Error("first item row must be mn26 at position 1 in group 1")
	}
	if !strings.Contains(out, "(1, 'ud1.10', 471, 16)") {
		t.Error("last item row must be ud1.10 at position 471 in group 16")
	}
	if again := RenderSeedSQL(tables, "AriyaSavaka"); again != out {
		t.Error("RenderSeedSQL must be deterministic")
	}
}
