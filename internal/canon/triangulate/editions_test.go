package triangulate

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lavantien/quotes-manager/internal/canon/model"
	"github.com/lavantien/quotes-manager/internal/config"
)

func TestE101Degraded(t *testing.T) {
	st := newTestStore(t)
	st.addEn(t, "sujato", "mn1", "0.1", "translated")

	emptyDir := t.TempDir()
	stream := filepath.Join(emptyDir, "html-editions.ndjson")
	if err := os.WriteFile(stream, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		env  Env
		want string
	}{
		{
			"not configured",
			Env{},
			"html editions stream not configured; editions checks degraded",
		},
		{
			"missing file",
			Env{HtmlSegmentsFile: filepath.Join(t.TempDir(), "missing.ndjson")},
			"html editions stream missing or empty",
		},
		{
			"empty file",
			Env{HtmlSegmentsFile: stream},
			"html editions stream missing or empty: " + stream,
		},
	} {
		ds, err := runE101(context.Background(), config.Defaults(), st, tc.env)
		if err != nil {
			t.Fatalf("%s: e101: %v", tc.name, err)
		}
		if len(ds) != 1 || ds[0].Severity != model.Info || !strings.Contains(ds[0].Detail, "degraded") {
			t.Fatalf("%s: degraded e101 must emit one info note, got %v", tc.name, ds)
		}
		if !strings.Contains(ds[0].Detail, tc.want) {
			t.Fatalf("%s: note = %q, want %q", tc.name, ds[0].Detail, tc.want)
		}
	}
}

func TestE101CorruptHTMLStreamErrors(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "html-editions.ndjson")
	if err := os.WriteFile(p, []byte("{not json\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := runE101(context.Background(), config.Defaults(), newTestStore(t), Env{HtmlSegmentsFile: p}); err == nil {
		t.Fatal("a read failure must not masquerade as degradation")
	}
}

func TestE101HTMLCoverage(t *testing.T) {
	st := newTestStore(t)
	st.addEn(t, "sujato", "mn1", "0.1", "translated")
	st.addEn(t, "sujato", "mn2", "0.1", "translated")
	html := writeHtmlSegments(t, []model.HTMLSegment{
		{Author: "sujato", Muid: "mn1", SegKey: "0.1", Text: "one"},
		{Author: "sujato", Muid: "xyz", SegKey: "0.1", Text: "stray"},
	})
	env := Env{HtmlSegmentsFile: html}

	ds, err := runE101(context.Background(), config.Defaults(), st, env)
	if err != nil {
		t.Fatalf("e101: %v", err)
	}
	if got := findDisc(ds, model.CheckEditionsCoverage, model.Warn, "no html segment for mn: mn2"); len(got) != 1 {
		t.Fatalf("missing html = %d (%v), want 1", len(got), ds)
	}
	if got := findDisc(ds, model.CheckEditionsCoverage, model.Warn, "html muid without bilara text: xyz"); len(got) != 1 {
		t.Fatalf("html-without-bilara = %d (%v), want 1", len(got), ds)
	}
	if len(ds) != 2 {
		t.Fatalf("matched mn1 must yield nothing, got %v", ds)
	}
}

func TestE101HTMLCleanHit(t *testing.T) {
	st := newTestStore(t)
	st.addEn(t, "sujato", "mn1", "0.1", "translated")
	st.addEn(t, "sujato", "mn2", "0.1", "translated")
	html := writeHtmlSegments(t, []model.HTMLSegment{
		{Author: "sujato", Muid: "mn1", SegKey: "0.1", Text: "one"},
		{Author: "sujato", Muid: "mn2", SegKey: "0.1", Text: "two"},
	})
	ds, err := runE101(context.Background(), config.Defaults(), st, Env{HtmlSegmentsFile: html})
	if err != nil {
		t.Fatalf("e101: %v", err)
	}
	if len(ds) != 0 {
		t.Fatalf("fully covered html stream must be clean, got %v", ds)
	}
}

func TestE101HTMLSnpNotSn(t *testing.T) {
	st := newTestStore(t)
	st.addEn(t, "sujato", "sn1", "0.1", "a")
	st.addEn(t, "sujato", "snp1.1", "0.1", "b")
	html := writeHtmlSegments(t, []model.HTMLSegment{
		{Author: "sujato", Muid: "sn1", SegKey: "0.1", Text: "a"},
	})
	ds, err := runE101(context.Background(), config.Defaults(), st, Env{HtmlSegmentsFile: html})
	if err != nil {
		t.Fatalf("e101: %v", err)
	}
	if got := findDisc(ds, model.CheckEditionsCoverage, model.Warn, "no html segment for snp: snp1.1"); len(got) != 1 {
		t.Fatalf("snp flagging wrong: %v", ds)
	}
	if got := findDisc(ds, model.CheckEditionsCoverage, model.Warn, "no html segment for sn:"); len(got) != 0 {
		t.Fatalf("covered sn1 must stay silent: %v", ds)
	}
	if len(ds) != 1 {
		t.Fatalf("want exactly the snp finding, got %v", ds)
	}
}

func TestE101HTMLGroupedFileJoinsInnerMuids(t *testing.T) {
	st := newTestStore(t)
	t1 := st.nodeID(t, "text", "an1.1", groupedTextData("an1.1-2", 1))
	t2 := st.nodeID(t, "text", "an1.2", groupedTextData("an1.1-2", 2))
	for _, f := range []struct {
		id int64
		m  string
	}{
		{t1, "an1.1"}, {t2, "an1.2"},
	} {
		seg := st.nodeID(t, "segment", "en/sujato:"+f.m+":1.1", segmentData("body"))
		st.addEdge(t, seg, f.id, "CHILD_OF", 1)
	}
	html := writeHtmlSegments(t, []model.HTMLSegment{
		{Author: "sujato", Muid: "an1.1", SegKey: "1.1", Text: "body"},
		{Author: "sujato", Muid: "an1.2", SegKey: "1.1", Text: "body"},
	})
	ds, err := runE101(context.Background(), config.Defaults(), st, Env{HtmlSegmentsFile: html})
	if err != nil {
		t.Fatalf("e101: %v", err)
	}
	if len(ds) != 0 {
		t.Fatalf("inner-muid join must cover both texts, got %v", ds)
	}

	htmlOne := writeHtmlSegments(t, []model.HTMLSegment{
		{Author: "sujato", Muid: "an1.1", SegKey: "1.1", Text: "body"},
	})
	ds, err = runE101(context.Background(), config.Defaults(), st, Env{HtmlSegmentsFile: htmlOne})
	if err != nil {
		t.Fatalf("e101: %v", err)
	}
	if got := findDisc(ds, model.CheckEditionsCoverage, model.Warn, "no html segment for an: an1.2"); len(got) != 1 {
		t.Fatalf("inner muid must flag by itself, got %v", ds)
	}
	if len(ds) != 1 {
		t.Fatalf("want exactly the an1.2 finding, got %v", ds)
	}
}

func TestE101HTMLAuthorCaseDivergence(t *testing.T) {
	st := newTestStore(t)
	st.addEn(t, "sujato", "mn1", "0.1", "translated")
	html := writeHtmlSegments(t, []model.HTMLSegment{
		{Author: "sujato", Muid: "mn1", SegKey: "0.1", Text: "one"},
	})
	cfg := config.Defaults()
	cfg.Canon.Editions.Authors = map[string][]string{"Sujato": {"mn"}}
	ds, err := runE101(context.Background(), cfg, st, Env{HtmlSegmentsFile: html})
	if err != nil {
		t.Fatalf("e101: %v", err)
	}
	if len(ds) != 0 {
		t.Fatalf("case-divergent author must claim neither half, got %v", ds)
	}
}

func TestE101WildcardAuthorClaimsNothing(t *testing.T) {
	st := newTestStore(t)
	st.addEn(t, "sujato", "mn1", "0.1", "translated")
	html := writeHtmlSegments(t, []model.HTMLSegment{
		{Author: "sujato", Muid: "mn1", SegKey: "0.1", Text: "one"},
	})

	cfg := config.Defaults()
	cfg.Canon.Editions.Authors = map[string][]string{"su%": {"mn"}}
	ds, err := runE101(context.Background(), cfg, st, Env{HtmlSegmentsFile: html})
	if err != nil {
		t.Fatalf("e101: %v", err)
	}
	if len(ds) != 0 {
		t.Fatalf("wildcard author must claim neither direction, got %v", ds)
	}
}

func writeHtmlSegments(t *testing.T, segs []model.HTMLSegment) string {
	t.Helper()
	var b strings.Builder
	for _, s := range segs {
		raw, err := json.Marshal(s)
		if err != nil {
			t.Fatal(err)
		}
		b.Write(raw)
		b.WriteByte('\n')
	}
	p := filepath.Join(t.TempDir(), "html-editions.ndjson")
	if err := os.WriteFile(p, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func groupedTextData(fileMuid string, ord int) string {
	return fmt.Sprintf(`{"file_muid":%q,"file_ord":%d}`, fileMuid, ord)
}

func TestE102HTMLExactPath(t *testing.T) {
	st := newTestStore(t)
	st.addRoot(t, "mn1", "0.1", "root")
	st.addEn(t, "sujato", "mn1", "0.1", "Middle Discourses 1")
	st.addEn(t, "sujato", "mn1", "1.1", "“So I have heard.”")
	st.addEn(t, "sujato", "mn1", "1.2", "At one time.")
	st.addEn(t, "sujato", "mn1", "1.3", "Real text.")
	html := writeHtmlSegments(t, []model.HTMLSegment{
		{Author: "sujato", Muid: "mn1", SegKey: "0.1", Text: "Middle Discourses 1"},
		{Author: "sujato", Muid: "mn1", SegKey: "1.1", Text: `"So I have heard." At one time.`},
		{Author: "sujato", Muid: "mn1", SegKey: "1.3", Text: "Real text."},
	})
	env := Env{
		HtmlSegmentsFile: html,
		EditionsHTML:     []HTMLRef{{Author: "sujato", Collection: "mn", BuildDate: "2026-01-01"}},
	}
	ds, err := runE102(context.Background(), config.Defaults(), st, env)
	if err != nil {
		t.Fatalf("e102: %v", err)
	}
	if len(ds) != 0 {
		t.Fatalf("exact html match must be clean, got %v", ds)
	}
	if len(st.covers) != 1 || st.covers[0].muid != "mn1" {
		t.Fatalf("covers = %v", st.covers)
	}
	var data struct {
		Similarity float64 `json:"similarity"`
		Source     string  `json:"source"`
	}
	if err := json.Unmarshal([]byte(st.covers[0].data), &data); err != nil {
		t.Fatal(err)
	}
	if data.Similarity != 1 || data.Source != "html sujato/mn" {
		t.Fatalf("covers data = %+v", data)
	}
}

func TestE102HTMLDivergenceFailsFast(t *testing.T) {
	st := newTestStore(t)
	st.addRoot(t, "mn1", "0.1", "root")
	st.addEn(t, "sujato", "mn1", "0.1", "Middle Discourses 1")
	st.addEn(t, "sujato", "mn1", "1.1", "So I have heard.")
	st.addEn(t, "sujato", "mn1", "1.2", "At one time.")
	html := writeHtmlSegments(t, []model.HTMLSegment{
		{Author: "sujato", Muid: "mn1", SegKey: "0.1", Text: "Middle Discourses 1"},
		{Author: "sujato", Muid: "mn1", SegKey: "1.1", Text: "So I have heard."},
		{Author: "sujato", Muid: "mn1", SegKey: "1.2", Text: "Once upon a time."},
		{Author: "sujato", Muid: "mn1", SegKey: "9.9", Text: "html-only segment"},
	})
	env := Env{
		HtmlSegmentsFile: html,
		EditionsHTML:     []HTMLRef{{Author: "sujato", Collection: "mn", BuildDate: "2026-01-01"}},
	}
	ds, err := runE102(context.Background(), config.Defaults(), st, env)
	if err != nil {
		t.Fatalf("e102: %v", err)
	}
	if len(ds) != 1 {
		t.Fatalf("divergent html = %d findings, want 1: %v", len(ds), ds)
	}
	d := ds[0]
	if d.Muid != "mn1" || !strings.Contains(d.Detail, "3 findings") ||
		!strings.Contains(d.Detail, `mn1:1.2 a="At one time." b="Once upon a time."`) ||
		!strings.Contains(d.Detail, "mn1:9.9") ||
		!strings.Contains(d.Detail, "mn1:1.2 no html segment") {
		t.Fatalf("divergence detail = %+v", d)
	}
	if len(st.covers) != 0 {
		t.Fatalf("divergent html must set no edge: %v", st.covers)
	}
}

func TestE102HTMLEmptyBilaraSegmentsIgnored(t *testing.T) {
	st := newTestStore(t)
	st.addRoot(t, "mn1", "0.1", "root")
	st.addEn(t, "sujato", "mn1", "0.1", "")
	st.addEn(t, "sujato", "mn1", "1.1", "Real text.")
	html := writeHtmlSegments(t, []model.HTMLSegment{
		{Author: "sujato", Muid: "mn1", SegKey: "1.1", Text: "Real text."},
	})
	env := Env{
		HtmlSegmentsFile: html,
		EditionsHTML:     []HTMLRef{{Author: "sujato", Collection: "mn", BuildDate: "2026-01-01"}},
	}
	ds, err := runE102(context.Background(), config.Defaults(), st, env)
	if err != nil {
		t.Fatalf("e102: %v", err)
	}
	if len(ds) != 0 || len(st.covers) != 1 {
		t.Fatalf("empty markers must be ignored, ds=%v covers=%v", ds, st.covers)
	}
}

func TestE102GroupedFileComparesOnceAndCoversEachText(t *testing.T) {
	st := newTestStore(t)
	t1 := st.nodeID(t, "text", "an1.1", groupedTextData("an1.1-2", 1))
	t2 := st.nodeID(t, "text", "an1.2", groupedTextData("an1.1-2", 2))
	for _, f := range []struct {
		id int64
		m  string
	}{
		{t1, "an1.1"}, {t2, "an1.2"},
	} {
		seg := st.nodeID(t, "segment", "en/sujato:"+f.m+":1.1", segmentData("body of "+f.m+" "))
		st.addEdge(t, seg, f.id, "CHILD_OF", 1)
	}
	html := writeHtmlSegments(t, []model.HTMLSegment{
		{Author: "sujato", Muid: "an1.1", SegKey: "1.1", Text: "body of an1.1"},
		{Author: "sujato", Muid: "an1.2", SegKey: "1.1", Text: "body of an1.2"},
	})

	env := Env{
		HtmlSegmentsFile: html,
		EditionsHTML:     []HTMLRef{{Author: "sujato", Collection: "an", BuildDate: "2026-01-01"}},
	}
	ds, err := runE102(context.Background(), config.Defaults(), st, env)
	if err != nil {
		t.Fatalf("e102: %v", err)
	}
	if len(ds) != 0 {
		t.Fatalf("grouped file must compare clean, got %v", ds)
	}
	covered := map[string]bool{}
	for _, c := range st.covers {
		if c.editionKey != "sujato/an" {
			t.Fatalf("bad edition key: %+v", c)
		}
		covered[c.muid] = true
	}
	if !covered["an1.1"] || !covered["an1.2"] {
		t.Fatalf("both inner texts must be covered: %v", st.covers)
	}
}

func TestE102ChildOrderFollowed(t *testing.T) {
	st := newTestStore(t)
	id := st.nodeID(t, "text", "mn1", textData("mn1"))
	idA := st.nodeID(t, "segment", "en/sujato:mn1:1.2", segmentData("beta "))
	st.addEdge(t, idA, id, "CHILD_OF", 1)
	idB := st.nodeID(t, "segment", "en/sujato:mn1:1.1", segmentData("alpha "))
	st.addEdge(t, idB, id, "CHILD_OF", 2)
	idC := st.nodeID(t, "segment", "en/sujato:mn1:1.3", segmentData("gamma"))
	st.addEdge(t, idC, id, "CHILD_OF", 3)
	html := writeHtmlSegments(t, []model.HTMLSegment{
		{Author: "sujato", Muid: "mn1", SegKey: "1.2", Text: "beta alpha"},
		{Author: "sujato", Muid: "mn1", SegKey: "1.3", Text: "gamma"},
	})
	env := Env{
		HtmlSegmentsFile: html,
		EditionsHTML:     []HTMLRef{{Author: "sujato", Collection: "mn", BuildDate: "2026-01-01"}},
	}
	ds, err := runE102(context.Background(), config.Defaults(), st, env)
	if err != nil {
		t.Fatalf("e102: %v", err)
	}
	if len(ds) != 0 {
		t.Fatalf("the html run must join bilara segments in CHILD_OF ord, got %v", ds)
	}
	if len(st.covers) != 1 {
		t.Fatalf("covers = %v", st.covers)
	}
}

func TestE102AbsentHTMLStreamSkips(t *testing.T) {
	st := newTestStore(t)
	st.addEn(t, "sujato", "mn1", "0.1", "body")
	env := Env{
		HtmlSegmentsFile: filepath.Join(t.TempDir(), "missing.ndjson"),
		EditionsHTML:     []HTMLRef{{Author: "sujato", Collection: "mn", BuildDate: "2026-01-01"}},
	}
	ds, err := runE102(context.Background(), config.Defaults(), st, env)
	if err != nil {
		t.Fatalf("absent html stream must skip without error: %v", err)
	}
	if len(ds) != 0 || len(st.covers) != 0 {
		t.Fatalf("absent html stream must yield nothing, ds=%v covers=%v", ds, st.covers)
	}
}

func TestE102NoEnSegmentsForAuthor(t *testing.T) {
	st := newTestStore(t)
	st.addRoot(t, "mn1", "0.1", "root")
	env := Env{EditionsHTML: []HTMLRef{{Author: "sujato", Collection: "mn", BuildDate: "2026-01-01"}}}
	ds, err := runE102(context.Background(), config.Defaults(), st, env)
	if err != nil {
		t.Fatalf("e102: %v", err)
	}
	if len(ds) != 0 {
		t.Fatalf("want zero discrepancies, got %v", ds)
	}
}

func TestE102SkipsUnparsableHTMLDates(t *testing.T) {
	st := newTestStore(t)
	st.addEn(t, "sujato", "mn1", "0.1", "body")
	html := writeHtmlSegments(t, []model.HTMLSegment{
		{Author: "sujato", Muid: "mn1", SegKey: "0.1", Text: "body"},
	})
	env := Env{HtmlSegmentsFile: html, EditionsHTML: []HTMLRef{
		{Author: "sujato", Collection: "mn", BuildDate: "not-a-date"},
	}}
	ds, err := runE102(context.Background(), config.Defaults(), st, env)
	if err != nil {
		t.Fatalf("e102: %v", err)
	}
	if len(st.covers) != 0 {
		t.Fatalf("covers must be empty, got %v", st.covers)
	}
	if len(ds) != 0 {
		t.Fatalf("no discrepancies expected, got %v", ds)
	}
}

func TestE102CrossCollectionScope(t *testing.T) {
	st := newTestStore(t)
	st.addEn(t, "sujato", "mn1", "0.1", "mn one body")
	st.addEn(t, "sujato", "an1", "0.1", "an one body")
	html := writeHtmlSegments(t, []model.HTMLSegment{
		{Author: "sujato", Muid: "mn1", SegKey: "0.1", Text: "mn one body"},
		{Author: "sujato", Muid: "an1", SegKey: "0.1", Text: "an one body"},
	})

	cfg := config.Defaults()
	cfg.Canon.Editions.Authors = map[string][]string{"sujato": {"an", "mn"}}
	env := Env{HtmlSegmentsFile: html, EditionsHTML: []HTMLRef{
		{Author: "sujato", Collection: "an", BuildDate: "2026-01-01"},
		{Author: "sujato", Collection: "mn", BuildDate: "2026-01-01"},
	}}
	ds, err := runE102(context.Background(), cfg, st, env)
	if err != nil {
		t.Fatalf("e102: %v", err)
	}
	if len(ds) != 0 {
		t.Fatalf("both matched texts must be clean, got %v", ds)
	}
	want := map[string]string{"mn1": "sujato/mn", "an1": "sujato/an"}
	if len(st.covers) != len(want) {
		t.Fatalf("covers = %v, want exactly %v", st.covers, want)
	}
	for _, c := range st.covers {
		if want[c.muid] != c.editionKey {
			t.Fatalf("muid %s covered by wrong edition %s, want %s", c.muid, c.editionKey, want[c.muid])
		}
	}
}

func TestE103OneFindingPerEdition(t *testing.T) {
	env := Env{
		EditionsHTML: []HTMLRef{
			{Author: "sujato", Collection: "mn", BuildDate: "2026-01-01", FileName: "A-sujato-an-2026-01-01.html"},
			{Author: "sujato", Collection: "mn", BuildDate: "2026-02-01", FileName: "B-sujato-an-2026-02-01.html"},
		},
		BilaraNewerThan: func(string) (string, bool) { return "2026-03-01", true },
	}
	ds, err := runE103(context.Background(), config.Defaults(), newTestStore(t), env)
	if err != nil {
		t.Fatalf("e103: %v", err)
	}
	if len(ds) != 1 {
		t.Fatalf("staleness findings = %d, want 1: %v", len(ds), ds)
	}
	if !strings.Contains(ds[0].Detail, "B-sujato-an-2026-02-01.html") {
		t.Fatalf("finding must name the newest html edition: %+v", ds[0])
	}
}

func TestE103Staleness(t *testing.T) {
	st := newTestStore(t)
	env := Env{
		EditionsHTML: []HTMLRef{
			{Author: "sujato", Collection: "mn", BuildDate: "2026-01-01"},
			{Author: "sujato", Collection: "dn", BuildDate: "2026-03-01"},
			{Author: "brahmali", Collection: "pli-tv-vi", BuildDate: "2026-01-01"},
		},
		BilaraNewerThan: func(author string) (string, bool) {
			if author == "sujato" {
				return "2026-02-01T10:00:00Z", true
			}
			return "", false
		},
	}
	ds, err := runE103(context.Background(), config.Defaults(), st, env)
	if err != nil {
		t.Fatalf("e103: %v", err)
	}
	if got := findDisc(ds, model.CheckEditionsStaleness, model.Warn, "sujato/mn"); len(got) != 1 {
		t.Fatalf("stale sujato/mn = %d (%v), want 1", len(got), ds)
	}
	if got := findDisc(ds, model.CheckEditionsStaleness, model.Warn, "sujato/dn"); len(got) != 0 {
		t.Fatalf("newer dn must not flag, got %v", ds)
	}
	if got := findDisc(ds, model.CheckEditionsStaleness, model.Warn, "brahmali"); len(got) != 0 {
		t.Fatalf("unknown author must not flag, got %v", ds)
	}
}

func TestNewestPerEdition(t *testing.T) {
	got := newestPerEdition([]HTMLRef{
		{Author: "sujato", Collection: "mn", BuildDate: "2026-01-01", FileName: "a"},
		{Author: "sujato", Collection: "mn", BuildDate: "2026-03-01", FileName: "older-name"},
		{Author: "sujato", Collection: "mn", BuildDate: "2026-03-01", FileName: "z-newer-name"},
		{Author: "sujato", Collection: "mn", BuildDate: "2026-03-01", FileName: "a-first"},
		{Author: "sujato", Collection: "sn", BuildDate: "2026-02-01", FileName: "b"},
	})
	if len(got) != 2 {
		t.Fatalf("editions = %d, want 2: %+v", len(got), got)
	}
	if got[0].Collection != "mn" || got[0].BuildDate != "2026-03-01" || got[0].FileName != "z-newer-name" {
		t.Fatalf("mn winner = %+v", got[0])
	}
	if got[1].Collection != "sn" || got[1].FileName != "b" {
		t.Fatalf("sn edition = %+v", got[1])
	}
}

func TestLoadHTMLSegmentsSkipsIncomplete(t *testing.T) {
	p := writeHtmlSegments(t, []model.HTMLSegment{
		{Author: "sujato", Muid: "mn1", SegKey: "1.1", Text: "a"},
		{Muid: "mn1", SegKey: "1.2", Text: "no author"},
		{Author: "sujato", SegKey: "1.3", Text: "no muid"},
		{Author: "sujato", Muid: "mn1", Text: "no segkey"},
		{Author: "sujato", Muid: "mn1", SegKey: "2.1", Text: "b"},
	})
	got, err := loadHTMLSegments(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || len(got["sujato"]) != 1 {
		t.Fatalf("authors/muids = %v", got)
	}
	segs := got["sujato"]["mn1"]
	if len(segs) != 2 || segs[0].SegKey != "1.1" || segs[1].SegKey != "2.1" {
		t.Fatalf("segments = %+v", segs)
	}
	if got, err := loadHTMLSegments(filepath.Join(t.TempDir(), "missing.ndjson")); err != nil || got != nil {
		t.Fatalf("missing file = %v/%v, want nil/nil", got, err)
	}
}

func TestHTMLMarkupRows(t *testing.T) {
	st := newTestStore(t)
	st.nodeID(t, "segment", prefixHTML+"mn1:1.1", segmentData("<h2>{}</h2>"))
	st.nodeID(t, "segment", prefixHTML+"mn1:2.1", segmentData("<p>"))
	st.nodeID(t, "segment", prefixHTML+"mn2:1.1", segmentData("<p>"))
	got, err := htmlMarkupRows(context.Background(), st.db, "mn1")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got["1.1"] != "<h2>{}</h2>" || got["2.1"] != "<p>" {
		t.Fatalf("markup rows = %v", got)
	}
}

func TestE103UnparsableDates(t *testing.T) {
	st := newTestStore(t)
	env := Env{
		EditionsHTML:    []HTMLRef{{Author: "sujato", Collection: "mn", BuildDate: "yesterday"}},
		BilaraNewerThan: func(string) (string, bool) { return "2026-02-01", true },
	}
	ds, err := runE103(context.Background(), config.Defaults(), st, env)
	if err != nil {
		t.Fatalf("e103: %v", err)
	}
	if got := findDisc(ds, model.CheckEditionsStaleness, model.Info, "html sujato/mn build date unparsable"); len(got) != 1 {
		t.Fatalf("unparsable build date must surface as info, got %v", ds)
	}

	env.EditionsHTML = []HTMLRef{{Author: "sujato", Collection: "mn", BuildDate: "2026-01-01"}}
	env.BilaraNewerThan = func(author string) (string, bool) {
		if author == "sujato" {
			return "not a date", true
		}
		return "", false
	}
	ds, err = runE103(context.Background(), config.Defaults(), st, env)
	if err != nil {
		t.Fatalf("e103: %v", err)
	}
	if got := findDisc(ds, model.CheckEditionsStaleness, model.Info, "bilara date unparsable"); len(got) != 1 {
		t.Fatalf("unparsable bilara date must surface as info, got %v", ds)
	}
}

func TestE103NilFuncWithHTMLSkips(t *testing.T) {
	env := Env{EditionsHTML: []HTMLRef{{Author: "sujato", Collection: "mn", BuildDate: "2026-01-01"}}}
	ds, err := runE103(context.Background(), config.Defaults(), newTestStore(t), env)
	if err != nil {
		t.Fatalf("e103: %v", err)
	}
	if len(ds) != 0 {
		t.Fatalf("unknown commit dates must stay silent, got %v", ds)
	}
}

func TestE104LastRun(t *testing.T) {
	st := newTestStore(t)
	htmls := []HTMLRef{{Author: "sujato", Collection: "mn", BuildDate: "2026-01-02"}}
	env := Env{EditionsHTML: htmls, LastRunDate: "2026-01-01"}

	ds, err := runE104(context.Background(), config.Defaults(), st, env)
	if err != nil {
		t.Fatalf("e104: %v", err)
	}
	if got := findDisc(ds, model.CheckEditionsLastRun, model.Info, ""); len(got) != 1 {
		t.Fatalf("differing last run = %d (%v), want 1 info", len(got), ds)
	}

	env.LastRunDate = "2026-01-02"
	ds, err = runE104(context.Background(), config.Defaults(), st, env)
	if err != nil {
		t.Fatalf("e104: %v", err)
	}
	if len(ds) != 0 {
		t.Fatalf("matching dates must not flag, got %v", ds)
	}

	env.LastRunDate = ""
	ds, err = runE104(context.Background(), config.Defaults(), st, env)
	if err != nil {
		t.Fatalf("e104: %v", err)
	}
	if len(ds) != 0 {
		t.Fatalf("missing last run date must skip, got %v", ds)
	}
}

func TestE104IgnoresTimeInDates(t *testing.T) {
	st := newTestStore(t)
	env := Env{
		EditionsHTML: []HTMLRef{{Author: "sujato", Collection: "mn", BuildDate: "2026-01-02T15:04:05Z"}},
		LastRunDate:  "2026-01-02",
	}
	ds, err := runE104(context.Background(), config.Defaults(), st, env)
	if err != nil {
		t.Fatalf("e104: %v", err)
	}
	if len(ds) != 0 {
		t.Fatalf("same calendar day must not flag, got %v", ds)
	}
}

func TestSortedHTML(t *testing.T) {
	in := []HTMLRef{
		{Author: "sujato", Collection: "mn", BuildDate: "2026-01-05"},
		{Author: "brahmali", Collection: "pli-tv-vi", BuildDate: "2026-01-01"},
		{Author: "sujato", Collection: "an", BuildDate: "2026-02-01"},
		{Author: "sujato", Collection: "an", BuildDate: "2026-01-02"},
	}
	got := sortedHTML(in)
	want := []HTMLRef{in[1], in[3], in[2], in[0]}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("sortedHTML = %v, want %v", got, want)
		}
	}
}

func TestE104NewestByInstant(t *testing.T) {
	st := newTestStore(t)
	env := Env{
		EditionsHTML: []HTMLRef{
			{Author: "sujato", Collection: "mn", BuildDate: "2026-01-06T00:00:00Z"},
			{Author: "sujato", Collection: "mn", BuildDate: "2026-01-06T01:00:00+14:00"},
		},
		LastRunDate: "2026-01-05",
	}
	ds, err := runE104(context.Background(), config.Defaults(), st, env)
	if err != nil {
		t.Fatalf("e104: %v", err)
	}
	if len(ds) != 1 || !strings.Contains(ds[0].Detail, "newest edition build 2026-01-06T00:00:00Z") {
		t.Fatalf("newest must be picked by instant, got %v", ds)
	}

	env2 := Env{
		EditionsHTML: []HTMLRef{
			{Author: "sujato", Collection: "mn", BuildDate: "2026-01-05"},
			{Author: "sujato", Collection: "mn", BuildDate: "2026-01-06T00:00:00Z"},
		},
		LastRunDate: "2026-01-05",
	}
	ds, err = runE104(context.Background(), config.Defaults(), st, env2)
	if err != nil {
		t.Fatalf("e104: %v", err)
	}
	if len(ds) != 1 || !strings.Contains(ds[0].Detail, "newest edition build 2026-01-06T00:00:00Z") {
		t.Fatalf("a newer later html file must win over the first, got %v", ds)
	}
}

func TestEscapeLike(t *testing.T) {
	cases := []struct{ in, want string }{
		{"sujato", "sujato"},
		{`a\b`, `a\\b`},
		{"a%b", `a\%b`},
		{"a_b", `a\_b`},
	}
	for _, tc := range cases {
		if got := escapeLike(tc.in); got != tc.want {
			t.Fatalf("escapeLike(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
