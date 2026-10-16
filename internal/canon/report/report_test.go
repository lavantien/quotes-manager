package report

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lavantien/quotes-manager/internal/canon/model"
)

var update = flag.Bool("update", false, "rewrite golden files")

func sampleInput() Input {
	return Input{
		Date: "2026-10-05",
		Provenance: map[string]string{
			"bilara_commit":   "abc123",
			"editions_commit": "def456",
		},
		Counts: map[string]int64{
			"nodes": 100,
			"edges": 250,
		},
		Coverage: []CoverageRow{
			{Division: "mn", Texts: 2, Covered: 1},
			{Division: "an", Texts: 1, Covered: 1},
		},
		Discrepancies: []model.Discrepancy{
			{
				Check: model.CheckKeysetRootTranslation, Severity: model.Warn, Muid: "mn1",
				Detail: "untranslated by sujato: mn1:0.3",
			},
			{
				Check: model.CheckKeysetRootTranslation, Severity: model.Error, Muid: "mn1",
				Detail: "orphan translation en/sujato:mn1:9.9",
			},
		},
		DiscrepancyCounts: map[string]int{
			"B001/warn":  1,
			"B001/error": 1,
		},
		DetailSampleMax: 200,
	}
}

func TestRenderMarkdownGolden(t *testing.T) {
	got := RenderMarkdown(sampleInput())
	golden := filepath.Join("testdata", "report.md")
	if *update {
		if err := os.WriteFile(golden, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	if got != string(want) {
		t.Fatalf("markdown mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestRenderJSONGolden(t *testing.T) {
	got, err := RenderJSON(sampleInput())
	if err != nil {
		t.Fatalf("render json: %v", err)
	}
	golden := filepath.Join("testdata", "report.json")
	if *update {
		if err := os.WriteFile(golden, got, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	if string(got) != string(want) {
		t.Fatalf("json mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestRenderDeterministic(t *testing.T) {
	a := sampleInput()
	b := sampleInput()
	b.Provenance = map[string]string{"editions_commit": "def456", "bilara_commit": "abc123"}
	b.Counts = map[string]int64{"edges": 250, "nodes": 100}
	b.Coverage = []CoverageRow{{Division: "an", Texts: 1, Covered: 1}, {Division: "mn", Texts: 2, Covered: 1}}
	b.Discrepancies = []model.Discrepancy{
		a.Discrepancies[1], a.Discrepancies[0],
	}
	b.DiscrepancyCounts = map[string]int{"B001/error": 1, "B001/warn": 1}

	if RenderMarkdown(a) != RenderMarkdown(b) {
		t.Fatal("markdown output must not depend on input ordering")
	}
	ja, err := RenderJSON(a)
	if err != nil {
		t.Fatal(err)
	}
	jb, err := RenderJSON(b)
	if err != nil {
		t.Fatal(err)
	}
	if string(ja) != string(jb) {
		t.Fatal("json output must not depend on input ordering")
	}
}

func TestDetailCapping(t *testing.T) {
	in := Input{
		Date: "2026-10-05",
		Discrepancies: []model.Discrepancy{{
			Check:    model.CheckEmptyValue,
			Severity: model.Info,
			Muid:     "mn1",
			Detail:   strings.Repeat("x", 500),
		}},
		DiscrepancyCounts: map[string]int{"B005/info": 1},
		DetailSampleMax:   50,
	}
	md := RenderMarkdown(in)
	if strings.Count(md, strings.Repeat("x", 51)) != 0 {
		t.Fatal("detail sample must be capped at DetailSampleMax runes")
	}
	if !strings.Contains(md, "[...]") {
		t.Fatal("capped details must carry the truncation marker")
	}
}

func TestSampleBucketCap(t *testing.T) {
	n := samplesPerBucket + 2
	in := Input{Date: "2026-10-05", DetailSampleMax: 200}
	for i := 0; i < n; i++ {
		in.Discrepancies = append(in.Discrepancies, model.Discrepancy{
			Check:    model.CheckEmptyValue,
			Severity: model.Info,
			Muid:     "mn1",
			Detail:   "detail",
		})
	}
	in.DiscrepancyCounts = map[string]int{"B005/info": n}
	md := RenderMarkdown(in)
	if got := strings.Count(md, "- mn1: detail"); got != samplesPerBucket {
		t.Fatalf("sample bullets = %d, want the cap %d", got, samplesPerBucket)
	}
	if !strings.Contains(md, "(+2 more)") {
		t.Fatalf("overflow must be announced, got:\n%s", md)
	}
}

func TestStalenessNote(t *testing.T) {
	fresh := Input{Date: "2026-10-05", DetailSampleMax: 200}
	if !strings.Contains(RenderMarkdown(fresh), "staleness-free") {
		t.Fatal("no E103 counts must render staleness-free")
	}
	stale := Input{
		Date:              "2026-10-05",
		DiscrepancyCounts: map[string]int{"E103/warn": 2},
		DetailSampleMax:   200,
	}
	if !strings.Contains(RenderMarkdown(stale), "stale editions: 2") {
		t.Fatal("E103 counts must render the stale note")
	}
	infoOnly := Input{
		Date:              "2026-10-05",
		DiscrepancyCounts: map[string]int{"E103/info": 5},
		DetailSampleMax:   200,
	}
	if !strings.Contains(RenderMarkdown(infoOnly), "staleness-free") {
		t.Fatal("info-only E103 rows are not staleness verdicts")
	}
}

func TestTruncateExactCap(t *testing.T) {
	cases := []struct {
		in   string
		max  int
		want string
	}{
		{"short", 10, "short"},
		{strings.Repeat("x", 60), 50, strings.Repeat("x", 44) + " [...]"},
		{strings.Repeat("x", 60), 6, "xxxxxx"},
		{strings.Repeat("x", 60), 4, "xxxx"},
	}
	for _, tc := range cases {
		if got := truncate(tc.in, tc.max); got != tc.want {
			t.Fatalf("truncate(len %d, max %d) = %q (len %d), want %q", len(tc.in), tc.max, got, len(got), tc.want)
		}
		if got := truncate(tc.in, tc.max); len([]rune(got)) > tc.max {
			t.Fatalf("truncate exceeded the cap: %d > %d", len([]rune(got)), tc.max)
		}
	}
}

func TestCoveragePipeEscaped(t *testing.T) {
	in := Input{
		Date:            "2026-10-05",
		Coverage:        []CoverageRow{{Division: "a|b", Texts: 1, Covered: 1}},
		DetailSampleMax: 200,
	}
	md := RenderMarkdown(in)
	if !strings.Contains(md, "| a\\|b | 1 | 1 |") {
		t.Fatalf("pipe in division must be escaped:\n%s", md)
	}
}

func TestDiscrepancySortOrder(t *testing.T) {
	in := Input{
		Date: "2026-10-05",
		Discrepancies: []model.Discrepancy{
			{Check: model.CheckEmptyValue, Severity: model.Error, Muid: "mn1", Detail: "b"},
			{Check: model.CheckKeysetRootTranslation, Severity: model.Warn, Muid: "zz1", Detail: "y"},
			{Check: model.CheckKeysetRootTranslation, Severity: model.Warn, Muid: "mn1", Detail: "z"},
			{Check: model.CheckKeysetRootTranslation, Severity: model.Warn, Muid: "mn1", Detail: "a"},
		},
		DetailSampleMax: 200,
	}
	b, err := RenderJSON(in)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	iB001 := strings.Index(s, `"check": "B001"`)
	iB005 := strings.Index(s, `"check": "B005"`)
	if iB001 < 0 || iB005 < 0 || iB001 > iB005 {
		t.Fatalf("check id must dominate severity in the json order:\n%s", s)
	}
	md := RenderMarkdown(in)
	posA := strings.Index(md, "- mn1: a")
	posZ := strings.Index(md, "- mn1: z")
	if posA < 0 || posZ < 0 || posA > posZ {
		t.Fatalf("detail must break ties within identical check+severity+muid:\n%s", md)
	}
}

func TestSampleOrdering(t *testing.T) {
	in := Input{
		Date: "2026-10-05",
		Discrepancies: []model.Discrepancy{
			{Check: model.CheckEmptyValue, Severity: model.Info, Muid: "mn2", Detail: "a"},
			{Check: model.CheckKeysetRootTranslation, Severity: model.Warn, Muid: "aa1", Detail: "x"},
			{Check: model.CheckKeysetRootTranslation, Severity: model.Error, Muid: "zz1", Detail: "y"},
			{Check: model.CheckEmptyValue, Severity: model.Info, Muid: "mn1", Detail: "z"},
		},
		DetailSampleMax: 200,
	}
	md := RenderMarkdown(in)
	bullets := []string{"- zz1: y", "- aa1: x", "- mn1: z", "- mn2: a"}
	last := -1
	for _, b := range bullets {
		i := strings.Index(md, b)
		if i < 0 || i <= last {
			t.Fatalf("samples out of order (want %v):\n%s", bullets, md)
		}
		last = i
	}
}

func TestEmptyInputSections(t *testing.T) {
	md := RenderMarkdown(Input{Date: "2026-10-05"})
	for _, want := range []string{"## provenance", "- none", "## counts", "## coverage", "## discrepancies", "none"} {
		if !strings.Contains(md, want) {
			t.Fatalf("markdown missing %q:\n%s", want, md)
		}
	}
}
