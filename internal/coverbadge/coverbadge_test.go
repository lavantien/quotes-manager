package coverbadge

import (
	"strings"
	"testing"
)

func TestPct(t *testing.T) {
	tests := []struct {
		name    string
		profile string
		want    float64
	}{
		{"all covered", "mode: set\nfoo.go:1.1,2.2 3 1\nbar.go:5.3,6.4 2 1\n", 100.0},
		{"none covered", "mode: set\nfoo.go:1.1,2.2 3 0\n", 0.0},
		{"half covered", "mode: set\nfoo.go:1.1,2.2 4 1\nbar.go:5.3,6.4 4 0\n", 50.0},
		{"count mode covered when >0", "mode: count\nfoo.go:1.1,2.2 5 3\nfoo.go:3.1,3.9 5 0\n", 50.0},
		{"ignores blank and mode lines", "\nmode: set\n\nfoo.go:1.1,2.2 2 1\n", 100.0},
		{"merged profile collapses duplicate blocks", "mode: set\nfoo.go:1.1,2.2 3 1\nfoo.go:1.1,2.2 3 0\nfoo.go:1.1,2.2 3 0\n", 100.0},
		{"merged profile mixed coverage", "mode: set\nfoo.go:1.1,2.2 4 1\nfoo.go:1.1,2.2 4 0\nbar.go:5.3,6.4 4 0\nbar.go:5.3,6.4 4 0\n", 50.0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Pct(tc.profile)
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Errorf("Pct = %.2f, want %.2f", got, tc.want)
			}
		})
	}
}

func TestPctEmpty(t *testing.T) {
	if _, err := Pct("mode: set\n"); err == nil {
		t.Error("empty profile should error")
	}
}

func TestColor(t *testing.T) {
	cases := map[float64]string{
		90:   "brightgreen",
		80:   "brightgreen",
		79.9: "yellow",
		60:   "yellow",
		59.9: "red",
		0:    "red",
	}
	for pct, want := range cases {
		if got := Color(pct); got != want {
			t.Errorf("Color(%.1f) = %s, want %s", pct, got, want)
		}
	}
}

func TestRenderBadge(t *testing.T) {
	readme := "# project\n\n<!-- coverage:START -->\nold badge\n<!-- coverage:END -->\n"
	got := RenderBadge(readme, 87.3)
	for _, want := range []string{
		"# project",
		StartMarker,
		EndMarker,
		"87.3%25", // pct, percent-encoded
		"brightgreen",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	if strings.Contains(got, "old badge") {
		t.Errorf("old badge not replaced:\n%s", got)
	}
}

func TestRenderBadgeNoMarkers(t *testing.T) {
	readme := "# project\nno markers here\n"
	if got := RenderBadge(readme, 50); got != readme {
		t.Errorf("readme changed without markers:\n%s", got)
	}
}

func TestBadgeURL(t *testing.T) {
	got := BadgeURL(90)
	for _, want := range []string{"90.0%25", "brightgreen", "img.shields.io"} {
		if !strings.Contains(got, want) {
			t.Errorf("BadgeURL(90) = %q, missing %q", got, want)
		}
	}
}

func TestPctSkipsMalformedLine(t *testing.T) {
	// A line missing the stmt/count fields is skipped; the valid line still counts.
	got, err := Pct("mode: set\nfoo.go:1.1,2.2\nbar.go:1.1,2.2 3 1\n")
	if err != nil {
		t.Fatal(err)
	}
	if got != 100.0 {
		t.Errorf("Pct = %.2f, want 100 (malformed line skipped)", got)
	}
}

func TestPctBadNumStmt(t *testing.T) {
	if _, err := Pct("mode: set\nfoo.go:1.1,2.2 x 1\n"); err == nil {
		t.Error("non-numeric numStmt should error")
	}
}

func TestPctBadCount(t *testing.T) {
	if _, err := Pct("mode: set\nfoo.go:1.1,2.2 3 y\n"); err == nil {
		t.Error("non-numeric count should error")
	}
}

func TestRenderBadgeReversedMarkers(t *testing.T) {
	// End marker before start marker: readme returned unchanged.
	readme := EndMarker + "\nbadge\n" + StartMarker
	if got := RenderBadge(readme, 50); got != readme {
		t.Errorf("reversed markers should be a no-op:\n%s", got)
	}
}
