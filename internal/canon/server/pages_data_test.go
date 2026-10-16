package server

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/lavantien/quotes-manager/internal/canon/web"
)

func TestCoverageBars(t *testing.T) {
	if got := coverageBars(nil); got != nil {
		t.Fatalf("coverageBars(nil) = %+v, want nil", got)
	}
	cov := []web.CoverRowView{
		{Division: "mn-div", Texts: 2, Covered: 1},
		{Division: "dn-div", Texts: 3, Covered: 3},
	}
	chart := coverageBars(cov)
	if chart == nil {
		t.Fatal("coverageBars over rows must render a chart")
	}
	if len(chart.Bars) != 2 ||
		chart.Bars[0] != (web.Bar{Label: "mn-div", Value: 1}) ||
		chart.Bars[1] != (web.Bar{Label: "dn-div", Value: 3}) {
		t.Fatalf("coverage bars = %+v", chart.Bars)
	}
}

func TestRunViewLabel(t *testing.T) {
	strPtr := func(s string) *string { return &s }
	if runView(nil) != nil {
		t.Fatal("runView without runs = non-nil")
	}
	r := runView([]runJSON{{
		ID: 7, DpdTag: strPtr("dpd-9"), BilaraCommit: strPtr("bc-9"),
		Started: strPtr("s"), Finished: strPtr("f"),
	}})
	if r == nil || r.ID != 7 || r.Label != "dpd-9" ||
		r.Started != "s" || r.Finished != "f" {
		t.Fatalf("run view = %+v", r)
	}
	r = runView([]runJSON{{ID: 8, BilaraCommit: strPtr("bc-8")}})
	if r == nil || r.Label != "bc-8" {
		t.Fatalf("run view fallback label = %+v", r)
	}
	r = runView([]runJSON{{ID: 9}})
	if r == nil || r.Label != "" {
		t.Fatalf("run view bare label = %+v", r)
	}
}

func TestTextHeading(t *testing.T) {
	srv, _ := newFixtureServer(t)
	ctx := t.Context()

	if got := srv.textHeading(ctx, "mn1"); got != "Mūlapariyāya" {
		t.Fatalf("named text heading = %q", got)
	}
	if got := srv.textHeading(ctx, "mn2"); got != "mn2" {
		t.Fatalf("unnamed text heading = %q, want mn2", got)
	}
	if got := srv.textHeading(ctx, "zz9"); got != "zz9" {
		t.Fatalf("unknown text heading = %q, want zz9", got)
	}
}

func TestUsageRefs(t *testing.T) {
	refs := []string{"mn1:0.1", "mn1:0.2", "mn2:0.1"}
	want := []web.UsageRef{{Muid: "mn1", SegKey: "0.1"}, {Muid: "mn1", SegKey: "0.2"}, {Muid: "mn2", SegKey: "0.1"}}
	got := usageRefs(refs)
	if len(got) != len(want) {
		t.Fatalf("usageRefs = %+v, want %+v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("usageRefs[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
	got = usageRefs([]string{"garbage", "mn1:0.1"})
	if len(got) != 1 || got[0] != (web.UsageRef{Muid: "mn1", SegKey: "0.1"}) {
		t.Fatalf("usageRefs malformed = %+v", got)
	}
	if got := usageRefs(nil); len(got) != 0 {
		t.Fatalf("usageRefs(nil) = %+v", got)
	}
}

func TestHeadwordEntry(t *testing.T) {
	srv, _ := newFixtureServer(t)
	ctx := t.Context()

	entries, err := srv.headwordEntry(ctx, "dhamma")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("dhamma entries = %+v", entries)
	}
	e := entries[0]
	if e.Lemma != "dhamma" || e.POS != "m" || e.Root != "dhṛ" ||
		e.FamilyRoot != "dhṛ" || e.Segments != 3 ||
		e.Meaning != "the teaching; nature" {
		t.Fatalf("dhamma entry = %+v", e)
	}

	entries, err = srv.headwordEntry(ctx, "nope")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("unknown lemma entries = %+v", entries)
	}
}

func TestAnalyticsPageLimit(t *testing.T) {
	srv := &Server{cfg: newFixtureConfig(t)}

	r := httptest.NewRequest("GET", "/analytics?limit=7", nil)
	if page := srv.analyticsPage(r); page.Limit != 7 {
		t.Fatalf("limit 7 page limit = %d", page.Limit)
	}
	r = httptest.NewRequest("GET", "/analytics?limit=abc", nil)
	if page := srv.analyticsPage(r); page.Limit != 50 {
		t.Fatalf("limit abc page limit = %d, want default 50", page.Limit)
	}
}

func TestAnalyticsPageSelected(t *testing.T) {
	srv := &Server{cfg: newFixtureConfig(t)}

	post := func(target, form string) *web.AnalyticsPage {
		t.Helper()
		r := httptest.NewRequest("POST", target, strings.NewReader(form))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		return srv.analyticsPage(r)
	}
	if page := post("/analytics", "aggregate=top-headwords"); page.Selected != "top-headwords" {
		t.Fatalf("post form selected = %q", page.Selected)
	}
	if page := post("/analytics?aggregate=coverage-by-division", "aggregate=top-headwords"); page.Selected != "coverage-by-division" {
		t.Fatalf("post query+form selected = %q, want the query value", page.Selected)
	}
	r := httptest.NewRequest("GET", "/analytics?aggregate=top-headwords", nil)
	if page := srv.analyticsPage(r); page.Selected != "top-headwords" {
		t.Fatalf("get selected = %q", page.Selected)
	}
}

func TestBarChart(t *testing.T) {
	rows := make([][]string, 12)
	for i := range rows {
		rows[i] = []string{string(rune('a' + i)), "10"}
	}
	chart := barChart(rows)
	if chart == nil || len(chart.Bars) != chartBarsMax {
		t.Fatalf("bars = %d, want the %d-bar cap", len(chart.Bars), chartBarsMax)
	}
	if chart.Bars[0].Label != "a" || chart.Bars[9].Label != "j" {
		t.Fatalf("capped bars = %+v", chart.Bars)
	}

	small := barChart([][]string{{"x", "1"}, {"y", "2"}, {"z", "3"}})
	if small == nil || len(small.Bars) != 3 {
		t.Fatalf("small chart = %+v", small)
	}
	if barChart([][]string{{"x"}, {"y", "abc"}}) != nil {
		t.Fatal("non-numeric rows must not render a chart")
	}
}
