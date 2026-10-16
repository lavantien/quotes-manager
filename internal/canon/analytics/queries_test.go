package analytics

import (
	"errors"
	"testing"

	"github.com/lavantien/quotes-manager/internal/canon/model"
)

func TestAggregateTopHeadwords(t *testing.T) {
	d := openAnalytics(t, buildSidecar(t))
	tbl, err := d.Aggregate(t.Context(), "top-headwords", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(tbl.Rows) != 3 || len(tbl.Cols) != 3 {
		t.Fatalf("shape cols=%v rows=%d; want 3 cols, 3 rows", tbl.Cols, len(tbl.Rows))
	}
	want := [][]any{
		{"dhamma", int64(3), int64(2)},
		{"bhikkhu", int64(2), int64(2)},
		{"eva", int64(1), int64(1)},
	}
	for i, row := range tbl.Rows {
		for j, v := range want[i] {
			if row[j] != v {
				t.Errorf("row %d = %v; want %v", i, row, want[i])
				break
			}
		}
	}

	tbl, err = d.Aggregate(t.Context(), "top-headwords", "mn", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(tbl.Rows) != 3 {
		t.Fatalf("division-filtered rows = %d; want 3", len(tbl.Rows))
	}
	if tbl.Rows[0][0] != "dhamma" || tbl.Rows[0][1] != int64(2) || tbl.Rows[0][2] != int64(1) {
		t.Errorf("mn top headword = %v; want dhamma with 2 occurrences in 1 text", tbl.Rows[0])
	}
}

func TestAggregateRootRanking(t *testing.T) {
	d := openAnalytics(t, buildSidecar(t))
	tbl, err := d.Aggregate(t.Context(), "root-ranking", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(tbl.Rows) != 2 {
		t.Fatalf("root ranking rows = %d; want 2", len(tbl.Rows))
	}
	if tbl.Rows[0][0] != "bhikkh" || tbl.Rows[0][2] != int64(2) {
		t.Errorf("first root row = %v; want bhikkh with 2 occurrences", tbl.Rows[0])
	}
	if tbl.Rows[1][0] != "dhṛ" || tbl.Rows[1][2] != int64(3) {
		t.Errorf("second root row = %v; want dhṛ with 3 occurrences", tbl.Rows[1])
	}
}

func TestAggregateHeadwordDensity(t *testing.T) {
	d := openAnalytics(t, buildSidecar(t))
	tbl, err := d.Aggregate(t.Context(), "headword-density", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(tbl.Rows) != 3 {
		t.Fatalf("density rows = %d; want 3", len(tbl.Rows))
	}
	if tbl.Rows[0][0] != "mn1" || tbl.Rows[0][5] != float64(1) {
		t.Errorf("mn1 density row = %v; want density 1", tbl.Rows[0])
	}
	if tbl.Rows[2][0] != "sn1" || tbl.Rows[2][5] != float64(0.667) {
		t.Errorf("sn1 density row = %v; want density 0.667", tbl.Rows[2])
	}
}

func TestAggregateVocabOverlap(t *testing.T) {
	d := openAnalytics(t, buildSidecar(t))
	tbl, err := d.Aggregate(t.Context(), "vocab-overlap", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(tbl.Rows) != 1 {
		t.Fatalf("vocab overlap rows = %d; want 1", len(tbl.Rows))
	}
	row := tbl.Rows[0]
	if row[0] != "mn" || row[1] != "sn" || row[2] != int64(2) ||
		row[3] != int64(3) || row[4] != int64(2) || row[5] != float64(0.667) {
		t.Errorf("overlap row = %v; want mn/sn sharing 2 of 3+2 at jaccard 0.667", row)
	}
}

func TestAggregateCoverageByDivision(t *testing.T) {
	d := openAnalytics(t, buildSidecar(t))
	tbl, err := d.Aggregate(t.Context(), "coverage-by-division", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(tbl.Rows) != 2 {
		t.Fatalf("coverage rows = %d; want 2", len(tbl.Rows))
	}
	if tbl.Rows[0][0] != "mn" || tbl.Rows[0][1] != int64(2) || tbl.Rows[0][2] != int64(1) ||
		tbl.Rows[0][3] != float64(1.5) {
		t.Errorf("mn coverage = %v; want 2 texts, 1 covered, mean 1.5 segments", tbl.Rows[0])
	}
	if tbl.Rows[1][0] != "sn" || tbl.Rows[1][1] != int64(1) || tbl.Rows[1][2] != int64(0) ||
		tbl.Rows[1][3] != float64(3) {
		t.Errorf("sn coverage = %v; want 1 text, 0 covered, mean 3 segments", tbl.Rows[1])
	}
}

func TestAggregateDiscrepancyTrend(t *testing.T) {
	d := openAnalytics(t, buildSidecar(t))
	tbl, err := d.Aggregate(t.Context(), "discrepancy-trend", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(tbl.Rows) != 2 {
		t.Fatalf("discrepancy trend rows = %d; want 2", len(tbl.Rows))
	}
	if tbl.Rows[0][0] != int64(1) || tbl.Rows[0][2] != string(model.CheckEmptyValue) ||
		tbl.Rows[0][3] != int64(2) {
		t.Errorf("trend row 0 = %v; want run 1, empty check, 2 rows", tbl.Rows[0])
	}
	if tbl.Rows[1][2] != string(model.CheckDpdUnknownForms) || tbl.Rows[1][3] != int64(1) {
		t.Errorf("trend row 1 = %v; want unknown-forms check, 1 row", tbl.Rows[1])
	}
}

func TestAggregateUnknownName(t *testing.T) {
	d := openAnalytics(t, buildSidecar(t))
	if _, err := d.Aggregate(t.Context(), "no-such-aggregate", "", 0); !errors.Is(err, ErrUnknownAggregate) {
		t.Errorf("unknown aggregate error = %v; want ErrUnknownAggregate", err)
	}
}

func TestAggregateRejectsDivisionWhereUnsupported(t *testing.T) {
	d := openAnalytics(t, buildSidecar(t))
	if _, err := d.Aggregate(t.Context(), "vocab-overlap", "mn", 0); err == nil {
		t.Error("vocab-overlap must reject the division filter")
	}
}

func TestAggregateLimitClampsRows(t *testing.T) {
	d := openAnalytics(t, buildSidecar(t))
	tbl, err := d.Aggregate(t.Context(), "top-headwords", "", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(tbl.Rows) != 2 {
		t.Errorf("limit 2 returned %d rows; want 2", len(tbl.Rows))
	}
}

func TestCatalogListsEveryAggregate(t *testing.T) {
	got := Catalog()
	want := []string{
		"top-headwords", "root-ranking", "headword-density",
		"vocab-overlap", "discrepancy-trend", "coverage-by-division",
	}
	if len(got) != len(want) {
		t.Fatalf("catalog has %d entries; want %d", len(got), len(want))
	}
	for i, e := range got {
		if e.Name != want[i] {
			t.Errorf("catalog[%d] = %q; want %q", i, e.Name, want[i])
		}
		if e.Desc == "" {
			t.Errorf("catalog[%d] has no description", i)
		}
	}
	for _, e := range got {
		if !hasParam(e, "limit") {
			t.Errorf("%s: limit param undocumented", e.Name)
		}
		if hasParam(e, "division") != divisionCapable(e.Name) {
			t.Errorf("%s: division param doc mismatch", e.Name)
		}
	}
}

func hasParam(e CatalogEntry, name string) bool {
	for _, p := range e.Params {
		if p.Name == name {
			return true
		}
	}
	return false
}

func divisionCapable(name string) bool {
	a, ok := lookupAggregate(name)
	return ok && a.division
}
