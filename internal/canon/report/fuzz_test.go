package report

import (
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/lavantien/quotes-manager/internal/canon/model"
)

func FuzzRenderJSONRoundTrip(f *testing.F) {
	f.Add("2026-10-05", "bilara_commit", "abc", "nodes", 2, "mn", 2, 1,
		"B005", "info", "mn1", "empty values: 3", "B005/info", 1, 200)
	f.Add("", "", "", "", 0, "", 0, 0, "", "", "", "", "", 0, -5)
	f.Add("", "", "", "", 0, "\xc5", 0, 0, "", "\xc5", "\xc5", "\xc5", "", 0, 5)
	f.Fuzz(func(t *testing.T, date, provKey, provVal, countKey string, countVal int,
		div string, texts, covered int, check, sev, muid, detail, dcountKey string,
		dcountN, max int,
	) {
		if max < 0 {
			max = -max
		}
		in := Input{
			Date:            date,
			Provenance:      map[string]string{provKey: provVal},
			Counts:          map[string]int64{countKey: int64(countVal)},
			Coverage:        []CoverageRow{{Division: div, Texts: texts, Covered: covered}},
			Discrepancies:   []model.Discrepancy{{Check: model.CheckID(check), Severity: model.Severity(sev), Muid: muid, Detail: detail}},
			DetailSampleMax: max,
		}
		if dcountKey != "" {
			in.DiscrepancyCounts = map[string]int{dcountKey: dcountN}
		}
		b, err := RenderJSON(in)
		if err != nil {
			t.Fatalf("render: %v", err)
		}
		var back Input
		if err := json.Unmarshal(b, &back); err != nil {
			t.Fatalf("round trip decode: %v (%s)", err, b)
		}
		wantMax := max
		if wantMax <= 0 {
			wantMax = defaultDetailSampleMax
		}
		if back.DetailSampleMax != wantMax {
			t.Fatalf("detail max not normalized: %d vs %d", back.DetailSampleMax, wantMax)
		}
		if len(back.Coverage) != 1 || len(back.Discrepancies) != 1 {
			t.Fatalf("content lost: %s", b)
		}
		if utf8.ValidString(div) && back.Coverage[0] != in.Coverage[0] {
			t.Fatalf("coverage row altered: %s", b)
		}
		if utf8.ValidString(check) && utf8.ValidString(sev) && utf8.ValidString(muid) &&
			utf8.ValidString(detail) && back.Discrepancies[0] != in.Discrepancies[0] {
			t.Fatalf("discrepancy altered: %s", b)
		}
		md := RenderMarkdown(in)
		if !strings.HasPrefix(md, "# triangulation report\n") || !strings.HasSuffix(md, "\n") {
			t.Fatalf("markdown frame broken: %q", md)
		}
	})
}
