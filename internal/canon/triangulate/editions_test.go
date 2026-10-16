package triangulate

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lavantien/quotes-manager/internal/canon/model"
	"github.com/lavantien/quotes-manager/internal/config"
)

func TestTrigramJaccard(t *testing.T) {
	cases := []struct {
		a, b string
		want float64
	}{
		{"", "", 1},
		{"abc", "", 0},
		{"", "abc", 0},
		{"abc", "abc", 1},
		{"abc", "xyz", 0},
		{"abcd", "abce", 1.0 / 3.0},
		{"abcdefabcdef", "abcdef", 0.4},
		{"αβγδε", "αβγδζ", 0.5},
	}
	for _, tc := range cases {
		got := trigramJaccard(tc.a, tc.b)
		if math.Abs(got-tc.want) > 1e-9 {
			t.Fatalf("jaccard(%q,%q) = %v, want %v", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestE101TexFallbackCoverage(t *testing.T) {
	st := newTestStore(t)
	tex := t.TempDir()
	st.writeTex(t, tex, "mn1", "body")
	st.writeTex(t, tex, "xyz", "body")
	st.addEn(t, "sujato", "mn1", "0.1", "translated")
	st.addEn(t, "sujato", "an1", "0.1", "translated")

	ds, err := runE101(context.Background(), config.Defaults(), st, Env{TexParsedDir: tex})
	if err != nil {
		t.Fatalf("e101: %v", err)
	}
	if got := findDisc(ds, model.CheckEditionsCoverage, model.Warn, "no tex for an: an1"); len(got) != 1 {
		t.Fatalf("missing-in-tex = %d (%v), want 1", len(got), ds)
	}
	if got := findDisc(ds, model.CheckEditionsCoverage, model.Warn, "tex muid without bilara text: xyz"); len(got) != 1 {
		t.Fatalf("tex-without-bilara = %d (%v), want 1", len(got), ds)
	}
	if len(ds) != 2 {
		t.Fatalf("matched mn1 must yield nothing, got %v", ds)
	}
}

func TestE101TexFallbackSnpNotSn(t *testing.T) {
	st := newTestStore(t)
	tex := t.TempDir()
	st.writeTex(t, tex, "mn1", "body")
	st.addEn(t, "sujato", "sn1", "0.1", "a")
	st.addEn(t, "sujato", "snp1.1", "0.1", "b")

	ds, err := runE101(context.Background(), config.Defaults(), st, Env{TexParsedDir: tex})
	if err != nil {
		t.Fatalf("e101: %v", err)
	}
	if got := findDisc(ds, model.CheckEditionsCoverage, model.Warn, "no tex for sn: sn1"); len(got) != 1 {
		t.Fatalf("sn flagging wrong: %v", ds)
	}
	if got := findDisc(ds, model.CheckEditionsCoverage, model.Warn, "no tex for snp: snp1.1"); len(got) != 1 {
		t.Fatalf("snp flagging wrong: %v", ds)
	}
}

func TestE101Degraded(t *testing.T) {
	st := newTestStore(t)
	st.addEn(t, "sujato", "mn1", "0.1", "translated")

	env := Env{TexParsedDir: filepath.Join(t.TempDir(), "missing")}
	ds, err := runE101(context.Background(), config.Defaults(), st, env)
	if err != nil {
		t.Fatalf("e101: %v", err)
	}
	if len(ds) != 1 || ds[0].Severity != model.Info || !strings.Contains(ds[0].Detail, "degraded") {
		t.Fatalf("degraded e101 must emit one info note, got %v", ds)
	}
	if !strings.Contains(ds[0].Detail, "html stream absent") ||
		!strings.Contains(ds[0].Detail, env.TexParsedDir) {
		t.Fatalf("note must name the fallback and the dir: %+v", ds[0])
	}

	notConfigured := Env{}
	ds, err = runE101(context.Background(), config.Defaults(), st, notConfigured)
	if err != nil {
		t.Fatalf("e101: %v", err)
	}
	if len(ds) != 1 || !strings.Contains(ds[0].Detail, "html stream absent and tex parsed dir not configured") {
		t.Fatalf("unconfigured dir note = %+v", ds[0])
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

func TestE101HTMLIgnoresTexDir(t *testing.T) {
	st := newTestStore(t)
	st.addEn(t, "sujato", "mn1", "0.1", "translated")
	html := writeHtmlSegments(t, []model.HTMLSegment{
		{Author: "sujato", Muid: "mn1", SegKey: "0.1", Text: "one"},
	})
	notADir := filepath.Join(t.TempDir(), "plain.txt")
	if err := os.WriteFile(notADir, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	ds, err := runE101(context.Background(), config.Defaults(), st,
		Env{HtmlSegmentsFile: html, TexParsedDir: notADir})
	if err != nil {
		t.Fatalf("html-primary must not read the tex dir: %v", err)
	}
	if len(ds) != 0 {
		t.Fatalf("covered html stream must be clean, got %v", ds)
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

func TestE102CoveredAndDivergent(t *testing.T) {
	st := newTestStore(t)
	tex := t.TempDir()
	segs := []string{"So I have heard. ", "At one time near Ukkaṭṭhā. "}
	st.addRoot(t, "mn1", "0.1", "root 1")
	st.addRoot(t, "mn1", "0.2", "root 2")
	st.addEn(t, "sujato", "mn1", "0.1", segs[0])
	st.addEn(t, "sujato", "mn1", "0.2", segs[1])
	st.writeTex(t, tex, "mn1", strings.Join(segs, " "))

	st.addRoot(t, "mn2", "0.1", "root")
	st.addEn(t, "sujato", "mn2", "0.1", "alpha beta gamma")
	st.writeTex(t, tex, "mn2", "completely different words here")

	env := Env{TexParsedDir: tex, EditionsZips: []ZipRef{{Author: "sujato", Collection: "mn", BuildDate: "2026-01-01"}}}
	cfg := config.Defaults()
	ds, err := runE102(context.Background(), cfg, st, env)
	if err != nil {
		t.Fatalf("e102: %v", err)
	}
	if got := findDisc(ds, model.CheckEditionsContent, model.Warn, "mn2"); len(got) != 1 {
		t.Fatalf("divergent mn2 = %d (%v), want 1 warn", len(got), ds)
	}
	d := findDisc(ds, model.CheckEditionsContent, model.Warn, "mn2")[0]
	if !strings.Contains(d.Detail, "first divergence") || !strings.Contains(d.Detail, "similarity") {
		t.Fatalf("divergence detail = %q", d.Detail)
	}
	if len(st.covers) != 1 {
		t.Fatalf("covers calls = %d (%v), want exactly mn1", len(st.covers), st.covers)
	}
	c := st.covers[0]
	if c.muid != "mn1" || c.editionKey != "sujato/mn" {
		t.Fatalf("bad covers call: %+v", c)
	}
	var data struct {
		Similarity float64 `json:"similarity"`
		Source     string  `json:"source"`
	}
	if err := json.Unmarshal([]byte(c.data), &data); err != nil {
		t.Fatalf("covers data not json: %v (%s)", err, c.data)
	}
	if data.Similarity < 0.9 || data.Similarity > 1 {
		t.Fatalf("identical texts similarity = %v, want ~1", data.Similarity)
	}
	if data.Source != filepath.Join(tex, "mn1.txt") {
		t.Fatalf("source = %q", data.Source)
	}
}

func groupedTextData(fileMuid string, ord int) string {
	return fmt.Sprintf(`{"file_muid":%q,"file_ord":%d}`, fileMuid, ord)
}

func TestE102GroupedFileComparesOnceAndCoversEachText(t *testing.T) {
	st := newTestStore(t)
	tex := t.TempDir()
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
	st.writeTex(t, tex, "an1.1-2", "body of an1.1 body of an1.2")

	env := Env{TexParsedDir: tex, EditionsZips: []ZipRef{{Author: "sujato", Collection: "an", BuildDate: "2026-01-01"}}}
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

func TestE102FoldToleratesTypography(t *testing.T) {
	st := newTestStore(t)
	tex := t.TempDir()
	st.addRoot(t, "mn1", "0.1", "root")
	st.addEn(t, "sujato", "mn1", "0.1", "“Sāriputta,” he said — 50% pure.")
	st.writeTex(t, tex, "mn1", `"Sāriputta," he said - 50\% pure.`)

	env := Env{TexParsedDir: tex, EditionsZips: []ZipRef{{Author: "sujato", Collection: "mn", BuildDate: "2026-01-01"}}}
	ds, err := runE102(context.Background(), config.Defaults(), st, env)
	if err != nil {
		t.Fatalf("e102: %v", err)
	}
	if len(ds) != 0 || len(st.covers) != 1 {
		t.Fatalf("typography-only divergence must be covered, ds=%v covers=%v", ds, st.covers)
	}
}

func TestE102RepetitionWarns(t *testing.T) {
	st := newTestStore(t)
	tex := t.TempDir()
	st.addRoot(t, "mn1", "0.1", "root")
	st.addEn(t, "sujato", "mn1", "0.1", "the same words over and over")
	st.writeTex(t, tex, "mn1", strings.Repeat("the same words over and over ", 40))

	env := Env{TexParsedDir: tex, EditionsZips: []ZipRef{{Author: "sujato", Collection: "mn", BuildDate: "2026-01-01"}}}
	ds, err := runE102(context.Background(), config.Defaults(), st, env)
	if err != nil {
		t.Fatalf("e102: %v", err)
	}
	if len(ds) != 1 || len(st.covers) != 0 {
		t.Fatalf("repetition must warn, ds=%v covers=%v", ds, st.covers)
	}
}

func TestE102SegmentReorderTolerated(t *testing.T) {
	st := newTestStore(t)
	tex := t.TempDir()
	id := st.nodeID(t, "text", "mn1", textData("mn1"))
	a := st.nodeID(t, "segment", "en/sujato:mn1:1.1", segmentData("first part "))
	b := st.nodeID(t, "segment", "en/sujato:mn1:1.2", segmentData("second part"))
	st.addEdge(t, b, id, "CHILD_OF", 1)
	st.addEdge(t, a, id, "CHILD_OF", 2)
	st.writeTex(t, tex, "mn1", "first part second part")

	env := Env{TexParsedDir: tex, EditionsZips: []ZipRef{{Author: "sujato", Collection: "mn", BuildDate: "2026-01-01"}}}
	ds, err := runE102(context.Background(), config.Defaults(), st, env)
	if err != nil {
		t.Fatalf("e102: %v", err)
	}
	if len(ds) != 0 || len(st.covers) != 1 {
		t.Fatalf("reordered segments must still be covered, ds=%v covers=%v", ds, st.covers)
	}
}

func TestE103OneFindingPerEdition(t *testing.T) {
	env := Env{
		EditionsZips: []ZipRef{
			{Author: "sujato", Collection: "mn", BuildDate: "2026-01-01", ZipName: "A-sujato-2026-01-01-tex.zip"},
			{Author: "sujato", Collection: "mn", BuildDate: "2026-02-01", ZipName: "B-sujato-2026-02-01-tex.zip"},
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
	if !strings.Contains(ds[0].Detail, "B-sujato-2026-02-01-tex.zip") {
		t.Fatalf("finding must name the newest zip: %+v", ds[0])
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

func TestE102HTMLExactPath(t *testing.T) {
	st := newTestStore(t)
	tex := t.TempDir()
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
		TexParsedDir:     tex,
		HtmlSegmentsFile: html,
		EditionsZips:     []ZipRef{{Author: "sujato", Collection: "mn", BuildDate: "2026-01-01"}},
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
	tex := t.TempDir()
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
		TexParsedDir:     tex,
		HtmlSegmentsFile: html,
		EditionsZips:     []ZipRef{{Author: "sujato", Collection: "mn", BuildDate: "2026-01-01"}},
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
	tex := t.TempDir()
	st.addRoot(t, "mn1", "0.1", "root")
	st.addEn(t, "sujato", "mn1", "0.1", "")
	st.addEn(t, "sujato", "mn1", "1.1", "Real text.")
	html := writeHtmlSegments(t, []model.HTMLSegment{
		{Author: "sujato", Muid: "mn1", SegKey: "1.1", Text: "Real text."},
	})
	env := Env{
		TexParsedDir:     tex,
		HtmlSegmentsFile: html,
		EditionsZips:     []ZipRef{{Author: "sujato", Collection: "mn", BuildDate: "2026-01-01"}},
	}
	ds, err := runE102(context.Background(), config.Defaults(), st, env)
	if err != nil {
		t.Fatalf("e102: %v", err)
	}
	if len(ds) != 0 || len(st.covers) != 1 {
		t.Fatalf("empty markers must be ignored, ds=%v covers=%v", ds, st.covers)
	}
}

func TestE102FallsBackToTexWithoutHTML(t *testing.T) {
	st := newTestStore(t)
	tex := t.TempDir()
	st.addRoot(t, "mn1", "0.1", "root")
	st.addEn(t, "sujato", "mn1", "0.1", "alpha beta gamma")
	st.writeTex(t, tex, "mn1", "completely different words here")
	env := Env{
		TexParsedDir: tex,
		EditionsZips: []ZipRef{{Author: "sujato", Collection: "mn", BuildDate: "2026-01-01"}},
	}
	ds, err := runE102(context.Background(), config.Defaults(), st, env)
	if err != nil {
		t.Fatalf("e102: %v", err)
	}
	if len(ds) != 1 || !strings.Contains(ds[0].Detail, "similarity") {
		t.Fatalf("tex fallback must run the trigram path, got %v", ds)
	}
}

func TestE102ChildOrderFollowed(t *testing.T) {
	st := newTestStore(t)
	tex := t.TempDir()
	st.addRoot(t, "mn1", "0.1", "r1")
	st.addRoot(t, "mn1", "0.2", "r2")
	id2 := st.nodeID(t, "text", "mn1", textData("mn1"))
	idA := st.nodeID(t, "segment", "en/sujato:mn1:0.2", segmentData("second first "))
	st.addEdge(t, idA, id2, "CHILD_OF", 1)
	idB := st.nodeID(t, "segment", "en/sujato:mn1:0.1", segmentData("first second "))
	st.addEdge(t, idB, id2, "CHILD_OF", 2)
	st.writeTex(t, tex, "mn1", "second first  first second ")

	cfg := config.Defaults()
	cfg.Canon.Triangulate.SimilarityMin = 0.5
	env := Env{TexParsedDir: tex, EditionsZips: []ZipRef{{Author: "sujato", Collection: "mn", BuildDate: "2026-01-01"}}}
	ds, err := runE102(context.Background(), cfg, st, env)
	if err != nil {
		t.Fatalf("e102: %v", err)
	}
	if len(ds) != 0 {
		t.Fatalf("joined-in-ord text must be identical, got %v", ds)
	}
	if len(st.covers) != 1 {
		t.Fatalf("covers = %v", st.covers)
	}
}

func TestE102NoEnSegmentsForAuthor(t *testing.T) {
	st := newTestStore(t)
	tex := t.TempDir()
	st.writeTex(t, tex, "mn1", "body")
	st.addRoot(t, "mn1", "0.1", "root")
	env := Env{TexParsedDir: tex, EditionsZips: []ZipRef{{Author: "sujato", Collection: "mn", BuildDate: "2026-01-01"}}}
	ds, err := runE102(context.Background(), config.Defaults(), st, env)
	if err != nil {
		t.Fatalf("e102: %v", err)
	}
	if len(ds) != 0 {
		t.Fatalf("want zero discrepancies, got %v", ds)
	}
}

func TestE102ThresholdBoundary(t *testing.T) {
	st := newTestStore(t)
	tex := t.TempDir()
	st.addRoot(t, "mn1", "0.1", "root")
	st.addEn(t, "sujato", "mn1", "0.1", "abcd")
	st.writeTex(t, tex, "mn1", "abce")

	env := Env{TexParsedDir: tex, EditionsZips: []ZipRef{{Author: "sujato", Collection: "mn", BuildDate: "2026-01-01"}}}
	cfg := config.Defaults()
	cfg.Canon.Triangulate.SimilarityMin = 1.0 / 3.0
	ds, err := runE102(context.Background(), cfg, st, env)
	if err != nil {
		t.Fatalf("e102: %v", err)
	}
	if len(ds) != 0 || len(st.covers) != 1 {
		t.Fatalf("similarity == min must be covered, ds=%v covers=%v", ds, st.covers)
	}

	st2 := newTestStore(t)
	st2.addRoot(t, "mn1", "0.1", "root")
	st2.addEn(t, "sujato", "mn1", "0.1", "abcd")
	cfg.Canon.Triangulate.SimilarityMin = 0.34
	ds, err = runE102(context.Background(), cfg, st2, env)
	if err != nil {
		t.Fatalf("e102: %v", err)
	}
	if len(ds) != 1 || len(st2.covers) != 0 {
		t.Fatalf("similarity < min must warn, ds=%v covers=%v", ds, st2.covers)
	}
}

func TestE102SkipsUnparsableZipDates(t *testing.T) {
	st := newTestStore(t)
	tex := t.TempDir()
	st.writeTex(t, tex, "mn1", "body")
	st.addEn(t, "sujato", "mn1", "0.1", "body")
	env := Env{TexParsedDir: tex, EditionsZips: []ZipRef{
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

func TestE103Staleness(t *testing.T) {
	st := newTestStore(t)
	env := Env{
		EditionsZips: []ZipRef{
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
	got := newestPerEdition([]ZipRef{
		{Author: "sujato", Collection: "mn", BuildDate: "2026-01-01", ZipName: "a"},
		{Author: "sujato", Collection: "mn", BuildDate: "2026-03-01", ZipName: "older-name"},
		{Author: "sujato", Collection: "mn", BuildDate: "2026-03-01", ZipName: "z-newer-name"},
		{Author: "sujato", Collection: "mn", BuildDate: "2026-03-01", ZipName: "a-first"},
		{Author: "sujato", Collection: "sn", BuildDate: "2026-02-01", ZipName: "b"},
	})
	if len(got) != 2 {
		t.Fatalf("editions = %d, want 2: %+v", len(got), got)
	}
	if got[0].Collection != "mn" || got[0].BuildDate != "2026-03-01" || got[0].ZipName != "z-newer-name" {
		t.Fatalf("mn winner = %+v", got[0])
	}
	if got[1].Collection != "sn" || got[1].ZipName != "b" {
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
		EditionsZips:    []ZipRef{{Author: "sujato", Collection: "mn", BuildDate: "yesterday"}},
		BilaraNewerThan: func(string) (string, bool) { return "2026-02-01", true },
	}
	ds, err := runE103(context.Background(), config.Defaults(), st, env)
	if err != nil {
		t.Fatalf("e103: %v", err)
	}
	if got := findDisc(ds, model.CheckEditionsStaleness, model.Info, "build date unparsable"); len(got) != 1 {
		t.Fatalf("unparsable build date must surface as info, got %v", ds)
	}

	env.EditionsZips = []ZipRef{{Author: "sujato", Collection: "mn", BuildDate: "2026-01-01"}}
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

func TestE104LastRun(t *testing.T) {
	st := newTestStore(t)
	zips := []ZipRef{{Author: "sujato", Collection: "mn", BuildDate: "2026-01-02"}}
	env := Env{EditionsZips: zips, LastRunDate: "2026-01-01"}

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
		EditionsZips: []ZipRef{{Author: "sujato", Collection: "mn", BuildDate: "2026-01-02T15:04:05Z"}},
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

func TestReadDirNames(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "mn1.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "notes.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "sub.txt"), 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := readDirNames(dir)
	if err != nil {
		t.Fatalf("readDirNames: %v", err)
	}
	if len(got) != 1 || got[0] != "mn1" {
		t.Fatalf("readDirNames = %v, want [mn1]", got)
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

func TestE101TexFallbackWildcardAuthorDoesNotMatch(t *testing.T) {
	st := newTestStore(t)
	tex := t.TempDir()
	st.writeTex(t, tex, "mn1", "body")
	st.addEn(t, "sujato", "mn1", "0.1", "translated")

	cfg := config.Defaults()
	cfg.Canon.Editions.Authors = map[string][]string{"su%": {"mn"}}
	ds, err := runE101(context.Background(), cfg, st, Env{TexParsedDir: tex})
	if err != nil {
		t.Fatalf("e101: %v", err)
	}
	if got := findDisc(ds, model.CheckEditionsCoverage, model.Warn, "no tex for"); len(got) != 0 {
		t.Fatalf("wildcard author must not claim sujato segments: %v", ds)
	}
	if got := findDisc(ds, model.CheckEditionsCoverage, model.Warn, "tex muid without bilara text"); len(got) != 1 {
		t.Fatalf("wildcard author claims nothing, tex side must flag once: %v", ds)
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

func TestE101TexFallbackAuthorCaseDivergence(t *testing.T) {
	st := newTestStore(t)
	tex := t.TempDir()
	st.writeTex(t, tex, "mn1", "body")
	st.addEn(t, "sujato", "mn1", "0.1", "translated")

	cfg := config.Defaults()
	cfg.Canon.Editions.Authors = map[string][]string{"Sujato": {"mn"}}
	ds, err := runE101(context.Background(), cfg, st, Env{TexParsedDir: tex})
	if err != nil {
		t.Fatalf("e101: %v", err)
	}
	if got := findDisc(ds, model.CheckEditionsCoverage, model.Warn, "no tex for"); len(got) != 0 {
		t.Fatalf("wrongly-cased author must not claim mn1: %v", ds)
	}
	if got := findDisc(ds, model.CheckEditionsCoverage, model.Warn, "tex muid without bilara text: mn1"); len(got) != 1 {
		t.Fatalf("tex side must flag the unclaimed muid once: %v", ds)
	}
}

func TestE101FileAsTexDirErrors(t *testing.T) {
	notADir := filepath.Join(t.TempDir(), "plain.txt")
	if err := os.WriteFile(notADir, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := runE101(context.Background(), config.Defaults(), newTestStore(t), Env{TexParsedDir: notADir}); err == nil {
		t.Fatal("a read failure must not masquerade as degradation")
	}
}

func TestE102EmptyBothSidesWarns(t *testing.T) {
	st := newTestStore(t)
	tex := t.TempDir()
	st.writeTex(t, tex, "mn1", "   ")
	st.addEn(t, "sujato", "mn1", "0.1", "")
	env := Env{TexParsedDir: tex, EditionsZips: []ZipRef{{Author: "sujato", Collection: "mn", BuildDate: "2026-01-01"}}}

	ds, err := runE102(context.Background(), config.Defaults(), st, env)
	if err != nil {
		t.Fatalf("e102: %v", err)
	}
	if got := findDisc(ds, model.CheckEditionsContent, model.Warn, "empty bilara and tex text"); len(got) != 1 {
		t.Fatalf("empty-empty must warn, got %v", ds)
	}
	if len(st.covers) != 0 {
		t.Fatalf("empty texts must not be covered, got %v", st.covers)
	}
}

func TestE102OneSidedEmptyUsesSimilarity(t *testing.T) {
	st := newTestStore(t)
	tex := t.TempDir()
	st.writeTex(t, tex, "mn1", "actual edition body")
	st.addEn(t, "sujato", "mn1", "0.1", "")
	env := Env{TexParsedDir: tex, EditionsZips: []ZipRef{{Author: "sujato", Collection: "mn", BuildDate: "2026-01-01"}}}

	ds, err := runE102(context.Background(), config.Defaults(), st, env)
	if err != nil {
		t.Fatalf("e102: %v", err)
	}
	if len(ds) != 1 {
		t.Fatalf("one-sided empty must run the similarity path, got %v", ds)
	}
	if !strings.Contains(ds[0].Detail, "similarity 0.0000") {
		t.Fatalf("one-sided empty must report similarity, got %q", ds[0].Detail)
	}
}

func TestE102AbsentTexDirWithZipsSkips(t *testing.T) {
	st := newTestStore(t)
	st.addEn(t, "sujato", "mn1", "0.1", "body")
	env := Env{
		TexParsedDir: filepath.Join(t.TempDir(), "missing"),
		EditionsZips: []ZipRef{{Author: "sujato", Collection: "mn", BuildDate: "2026-01-01"}},
	}
	ds, err := runE102(context.Background(), config.Defaults(), st, env)
	if err != nil {
		t.Fatalf("absent tex dir must skip without error: %v", err)
	}
	if len(ds) != 0 || len(st.covers) != 0 {
		t.Fatalf("absent tex dir must yield nothing, ds=%v covers=%v", ds, st.covers)
	}
}

func TestE102CrossCollectionScope(t *testing.T) {
	st := newTestStore(t)
	tex := t.TempDir()
	st.addEn(t, "sujato", "mn1", "0.1", "mn one body")
	st.addEn(t, "sujato", "an1", "0.1", "an one body")
	st.writeTex(t, tex, "mn1", "mn one body")
	st.writeTex(t, tex, "an1", "an one body")

	cfg := config.Defaults()
	cfg.Canon.Editions.Authors = map[string][]string{"sujato": {"an", "mn"}}
	env := Env{TexParsedDir: tex, EditionsZips: []ZipRef{
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

func TestDivergenceDetailWindowAndEdges(t *testing.T) {
	a := strings.Repeat("p", 10) + "ABCDEFGHIJKLMNOPQRST" + "X" + strings.Repeat("q", 40)
	b := strings.Repeat("p", 10) + "ABCDEFGHIJKLMNOPQRST" + "Y" + strings.Repeat("q", 40)
	d := divergenceDetail(a, b, 0.5, 0.9, 100)
	if !strings.Contains(d, "first divergence at rune 30") {
		t.Fatalf("offset wrong: %q", d)
	}
	if !strings.Contains(d, "ABCDEFGHIJKLMNOPQRSTX") || strings.Contains(d, "pp") {
		t.Fatalf("window must start 20 runes before the divergence: %q", d)
	}

	if d := divergenceDetail("same", "same", 0.5, 0.9, 100); !strings.Contains(d, "texts are equal") {
		t.Fatalf("equal texts: %q", d)
	}
	if d := divergenceDetail("abcX", "abc", 0.5, 0.9, 100); !strings.Contains(d, "first divergence at rune 3") {
		t.Fatalf("prefix extension must report the length divergence: %q", d)
	}
}

func TestE103NilFuncWithZipsSkips(t *testing.T) {
	env := Env{EditionsZips: []ZipRef{{Author: "sujato", Collection: "mn", BuildDate: "2026-01-01"}}}
	ds, err := runE103(context.Background(), config.Defaults(), newTestStore(t), env)
	if err != nil {
		t.Fatalf("e103: %v", err)
	}
	if len(ds) != 0 {
		t.Fatalf("unknown commit dates must stay silent, got %v", ds)
	}
}

func TestSortedZips(t *testing.T) {
	in := []ZipRef{
		{Author: "sujato", Collection: "mn", BuildDate: "2026-01-05"},
		{Author: "brahmali", Collection: "pli-tv-vi", BuildDate: "2026-01-01"},
		{Author: "sujato", Collection: "an", BuildDate: "2026-02-01"},
		{Author: "sujato", Collection: "an", BuildDate: "2026-01-02"},
	}
	got := sortedZips(in)
	want := []ZipRef{in[1], in[3], in[2], in[0]}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("sortedZips = %v, want %v", got, want)
		}
	}
}

func TestE104NewestByInstant(t *testing.T) {
	st := newTestStore(t)
	env := Env{
		EditionsZips: []ZipRef{
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
		EditionsZips: []ZipRef{
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
		t.Fatalf("a newer later zip must win over the first zip, got %v", ds)
	}
}

func TestDivergenceDetailCap(t *testing.T) {
	a := strings.Repeat("a", 500) + "X" + strings.Repeat("b", 500)
	b := strings.Repeat("a", 500) + "Y" + strings.Repeat("b", 500)
	d := divergenceDetail(a, b, 0.42, 0.85, 200)
	if len([]rune(d)) > 400 {
		t.Fatalf("detail not capped: %d runes", len([]rune(d)))
	}
	if !strings.Contains(d, "first divergence at rune 500") {
		t.Fatalf("detail = %q", d)
	}
}
