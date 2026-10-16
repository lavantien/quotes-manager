package dpd

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func fixtureDB(t *testing.T) *sql.DB {
	t.Helper()
	dbPath, _, err := WriteFixture(t.TempDir())
	if err != nil {
		t.Fatalf("WriteFixture: %v", err)
	}
	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestStreamHeadwords(t *testing.T) {
	db := fixtureDB(t)
	var got []Headword
	if err := StreamHeadwords(context.Background(), db, func(h Headword) error {
		got = append(got, h)
		return nil
	}); err != nil {
		t.Fatalf("StreamHeadwords: %v", err)
	}
	var lemmas []string
	for _, h := range got {
		lemmas = append(lemmas, h.Lemma1)
	}
	wantOrder := []string{"dhamma", "eva", "gacchati", "me", "suṇāti"}
	if !reflect.DeepEqual(lemmas, wantOrder) {
		t.Fatalf("lemma order = %v, want %v", lemmas, wantOrder)
	}

	var dhamma, eva, me *Headword
	for i, h := range got {
		switch h.Lemma1 {
		case "dhamma":
			dhamma = &got[i]
		case "eva":
			eva = &got[i]
		case "me":
			me = &got[i]
		}
	}
	if dhamma == nil || eva == nil || me == nil {
		t.Fatalf("missing fixture headwords: %+v", got)
	}
	wantDhamma := Headword{
		ID: 2, Lemma1: "dhamma", Lemma2: "dhammā", Pos: "m", Grammar: "masc nom sg",
		Meaning1: "the teaching; nature; the way things are", MeaningLit: "bearer",
		Sanskrit: "dharma", RootKey: "dhṛ", RootSign: "i", FamilyRoot: "dhṛ",
		Construction: "dha + ma", CompoundType: "", Stem: "dhamma",
		Inflections: "dhammo, dhammā, dhammaṃ",
	}
	if *dhamma != wantDhamma {
		t.Errorf("dhamma = %+v, want %+v", *dhamma, wantDhamma)
	}
	if eva.MeaningLit != "" || eva.RootKey != "" || eva.Construction != "" || eva.Stem != "" {
		t.Errorf("eva NULL columns leaked: %+v", *eva)
	}
	if eva.Inflections != "evaṃ" {
		t.Errorf("eva inflections = %q, want evaṃ", eva.Inflections)
	}
	if me.Inflections != "" {
		t.Errorf("me inflections = %q, want empty for NULL", me.Inflections)
	}
	if forms := SplitInflections(got[4].Inflections); !reflect.DeepEqual(forms, []string{"suṇāsi", "sutam"}) {
		t.Errorf("suṇāti forms = %v", forms)
	}
}

func TestStreamRoots(t *testing.T) {
	db := fixtureDB(t)
	var got []Root
	if err := StreamRoots(context.Background(), db, func(r Root) error {
		got = append(got, r)
		return nil
	}); err != nil {
		t.Fatalf("StreamRoots: %v", err)
	}
	want := []Root{
		{Root: "dhṛ", RootInComps: "dharma", RootHasVerb: "･", RootGroup: "1", RootSign: "i", RootMeaning: "to hold, bear", SanskritRoot: "dhṛ"},
		{Root: "gam", RootInComps: "gaccha", RootHasVerb: "･", RootGroup: "1", RootSign: "", RootMeaning: "to go", SanskritRoot: "gam"},
		{Root: "suṇ", RootInComps: "suṇa", RootHasVerb: "×", RootGroup: "1", RootSign: "ā", RootMeaning: "to hear", SanskritRoot: "śru"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("roots = %+v, want %+v", got, want)
	}
}

func TestStreamLookup(t *testing.T) {
	db := fixtureDB(t)
	var got []LookupRow
	if err := StreamLookup(context.Background(), db, func(l LookupRow) error {
		got = append(got, l)
		return nil
	}); err != nil {
		t.Fatalf("StreamLookup: %v", err)
	}
	var keys []string
	for _, l := range got {
		keys = append(keys, l.LookupKey)
	}
	wantKeys := []string{"dhamma", "dhammaca", "evam", "me", "sutam"}
	if !reflect.DeepEqual(keys, wantKeys) {
		t.Fatalf("lookup keys = %v, want %v", keys, wantKeys)
	}
	byKey := map[string]LookupRow{}
	for _, l := range got {
		byKey[l.LookupKey] = l
	}
	if hw := UnpackList(byKey["evam"].Headwords); !reflect.DeepEqual(hw, []string{"eva"}) {
		t.Errorf("evam headwords = %v, want [eva]", hw)
	}
	if hw := UnpackList(byKey["sutam"].Headwords); !reflect.DeepEqual(hw, []string{"suṇāti"}) {
		t.Errorf("sutam headwords = %v, want [suṇāti]", hw)
	}
	if hw := UnpackList(byKey["me"].Headwords); !reflect.DeepEqual(hw, []string{"me"}) {
		t.Errorf("me headwords = %v, want [me]", hw)
	}
	if dc := UnpackList(byKey["dhammaca"].Deconstructor); !reflect.DeepEqual(dc, []string{"dhamma", "ca"}) {
		t.Errorf("dhammaca deconstructor = %v, want [dhamma ca]", dc)
	}
	if hw := UnpackList(byKey["dhammaca"].Headwords); hw != nil {
		t.Errorf("dhammaca headwords = %v, want nil for NULL", hw)
	}
}

func schemaDriftDB(t *testing.T, drop string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "drift.db")
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	headCols := "id INTEGER PRIMARY KEY, lemma_1 TEXT"
	for _, c := range []string{
		"lemma_2", "pos", "grammar", "meaning_1", "meaning_lit", "sanskrit",
		"root_key", "root_sign", "family_root", "construction",
		"compound_type", "stem", "inflections",
	} {
		if c != drop {
			headCols += ", " + c + " TEXT"
		}
	}
	rootCols := "root TEXT PRIMARY KEY, root_in_comps TEXT, root_has_verb TEXT, root_group TEXT, root_sign TEXT, root_meaning TEXT"
	if drop != "sanskrit_root" {
		rootCols += ", sanskrit_root TEXT"
	}
	lookupCols := "lookup_key TEXT PRIMARY KEY, headwords TEXT"
	if drop != "deconstructor" {
		lookupCols += ", deconstructor TEXT"
	}
	for _, stmt := range []string{
		"CREATE TABLE dpd_headwords (" + headCols + ")",
		"CREATE TABLE dpd_roots (" + rootCols + ")",
		"CREATE TABLE lookup (" + lookupCols + ")",
		"INSERT INTO dpd_headwords (id, lemma_1) VALUES (1, 'x')",
		"INSERT INTO dpd_roots (root) VALUES ('x')",
		"INSERT INTO lookup (lookup_key) VALUES ('x')",
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	return path
}

func TestStreamSchemaDriftFailsLoudly(t *testing.T) {
	cases := []struct {
		drop string
	}{
		{"stem"},
		{"inflections"},
		{"sanskrit_root"},
		{"deconstructor"},
	}
	for _, tc := range cases {
		t.Run("missing "+tc.drop, func(t *testing.T) {
			db, err := Open(schemaDriftDB(t, tc.drop))
			if err != nil {
				t.Fatalf("Open: %v", err)
			}
			defer func() { _ = db.Close() }()
			var headErr, rootErr, lookupErr error
			headErr = StreamHeadwords(context.Background(), db, func(Headword) error { return nil })
			rootErr = StreamRoots(context.Background(), db, func(Root) error { return nil })
			lookupErr = StreamLookup(context.Background(), db, func(LookupRow) error { return nil })
			for _, check := range []struct {
				name string
				drop string
				err  error
			}{
				{"StreamHeadwords", tc.drop, headErr},
				{"StreamRoots", tc.drop, rootErr},
				{"StreamLookup", tc.drop, lookupErr},
			} {
				affected := map[string][]string{
					"stem":          {"StreamHeadwords"},
					"inflections":   {"StreamHeadwords"},
					"sanskrit_root": {"StreamRoots"},
					"deconstructor": {"StreamLookup"},
				}[check.drop]
				shouldFail := false
				for _, a := range affected {
					if a == check.name {
						shouldFail = true
					}
				}
				if shouldFail {
					if check.err == nil {
						t.Errorf("%s with %q dropped: expected loud failure", check.name, check.drop)
						continue
					}
					if !strings.Contains(check.err.Error(), check.drop) {
						t.Errorf("%s error = %v, want it to name %q", check.name, check.err, check.drop)
					}
				} else if check.err != nil {
					t.Errorf("%s should not depend on %q: %v", check.name, check.drop, check.err)
				}
			}
		})
	}
}

func TestStreamMinimalHandmadeDB(t *testing.T) {
	db, err := Open(schemaDriftDB(t, ""))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	n := 0
	if err := StreamHeadwords(context.Background(), db, func(Headword) error { n++; return nil }); err != nil {
		t.Fatalf("StreamHeadwords on minimal db: %v", err)
	}
	if n != 1 {
		t.Errorf("hand-made db has 1 headword, got %d", n)
	}
	if err := StreamRoots(context.Background(), db, func(Root) error { n++; return nil }); err != nil {
		t.Fatalf("StreamRoots on minimal db: %v", err)
	}
	if err := StreamLookup(context.Background(), db, func(LookupRow) error { n++; return nil }); err != nil {
		t.Fatalf("StreamLookup on minimal db: %v", err)
	}
	if n != 3 {
		t.Errorf("total streamed rows = %d, want 3 (one per table)", n)
	}
}

func TestOpenNonDatabaseFails(t *testing.T) {
	junk := filepath.Join(t.TempDir(), "junk.db")
	if err := os.WriteFile(junk, []byte("not a database at all"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(junk); err == nil {
		t.Fatal("Open on a junk file: expected error")
	}
}

func TestStreamYieldErrorPropagates(t *testing.T) {
	db := fixtureDB(t)
	sentinel := errors.New("stop")
	for _, stream := range []struct {
		name string
		fn   func(func(error) error) error
	}{
		{"headwords", func(yield func(error) error) error {
			return StreamHeadwords(context.Background(), db, func(Headword) error { return yield(sentinel) })
		}},
		{"roots", func(yield func(error) error) error {
			return StreamRoots(context.Background(), db, func(Root) error { return yield(sentinel) })
		}},
		{"lookup", func(yield func(error) error) error {
			return StreamLookup(context.Background(), db, func(LookupRow) error { return yield(sentinel) })
		}},
	} {
		if err := stream.fn(func(e error) error { return e }); !errors.Is(err, sentinel) {
			t.Errorf("%s yield error not propagated: %v", stream.name, err)
		}
	}
}

func TestStreamCanceledContext(t *testing.T) {
	db := fixtureDB(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := StreamHeadwords(ctx, db, func(Headword) error { return nil }); err == nil {
		t.Error("StreamHeadwords with canceled ctx: expected error")
	}
}

func TestOpenReadOnly(t *testing.T) {
	db := fixtureDB(t)
	if _, err := db.Exec("INSERT INTO lookup (lookup_key) VALUES ('nope')"); err == nil {
		t.Error("write on a read-only dpd connection: expected failure")
	}
	if _, err := db.Exec("CREATE TABLE x (a)"); err == nil {
		t.Error("DDL on a read-only dpd connection: expected failure")
	}
}

func TestOpenMissingFileDoesNotCreate(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "absent.db")
	if _, err := Open(missing); err == nil {
		t.Fatal("Open on a missing db: expected error")
	}
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Errorf("Open must not create files, stat err = %v", err)
	}
}

func TestOpenRejectsUnusablePaths(t *testing.T) {
	dir := t.TempDir()
	for name, p := range map[string]string{
		"empty":   "",
		"query":   filepath.Join(dir, "a?b.db"),
		"percent": filepath.Join(dir, "a%b.db"),
	} {
		db, err := Open(p)
		if err == nil {
			_ = db.Close()
			t.Errorf("Open(%s): expected error", name)
		}
	}
}

func TestSplitInflections(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"dhammo, dhammā, dhammaṃ", []string{"dhammo", "dhammā", "dhammaṃ"}},
		{"evaṃ", []string{"evaṃ"}},
		{"", nil},
		{"   ", nil},
		{" , , ", nil},
		{"a,,b", []string{"a", "b"}},
		{"  gacchāmi ,gacchasi\t", []string{"gacchāmi", "gacchasi"}},
		{"suṇāsi, sutam", []string{"suṇāsi", "sutam"}},
	}
	for _, tc := range cases {
		got := SplitInflections(tc.in)
		if len(got) == 0 && tc.want == nil {
			continue
		}
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("SplitInflections(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestUnpackList(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{`["dhamma","ca"]`, []string{"dhamma", "ca"}},
		{`["eva"]`, []string{"eva"}},
		{`[]`, []string{}},
		{`[" evaṃ ", ""]`, []string{" evaṃ ", ""}},
		{"", nil},
		{"null", nil},
		{"not json", nil},
		{`["a"`, nil},
		{`[1,2]`, nil},
		{`{"a":"b"}`, nil},
		{`"just a string"`, nil},
	}
	for _, tc := range cases {
		got := UnpackList(tc.in)
		if tc.want == nil {
			if got != nil {
				t.Errorf("UnpackList(%q) = %q, want nil", tc.in, got)
			}
			continue
		}
		if len(got) != len(tc.want) {
			t.Errorf("UnpackList(%q) = %q, want %q", tc.in, got, tc.want)
			continue
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("UnpackList(%q) = %q, want %q", tc.in, got, tc.want)
				break
			}
		}
	}
}
