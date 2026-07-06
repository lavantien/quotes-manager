package quote

import (
	"os"
	"reflect"
	"testing"
)

func quotesEqual(a, b *Quote) bool {
	return a.SuttaID == b.SuttaID && a.Citation == b.Citation && reflect.DeepEqual(a.Passages, b.Passages)
}

func TestParseCanonicalSingleRoundTrip(t *testing.T) {
	orig := New("MN 22", "the Buddha, MN 22", []string{`"first passage"`, "second passage", "last passage"})
	got := ParseCanonical(orig.BodyMD())
	if len(got) != 1 {
		t.Fatalf("got %d quotes, want 1", len(got))
	}
	if !quotesEqual(got[0], orig) {
		t.Errorf("round-trip mismatch:\n got  %+v\n want %+v", got[0], orig)
	}
}

func TestParseCanonicalMultipleWithDivider(t *testing.T) {
	qs := []*Quote{
		New("MN 22", "the Buddha, MN 22", []string{"alpha one", "alpha two"}),
		New("AN 5.34", "the Buddha, AN 5.34", []string{"beta single"}),
		New("SN 5.2", "the bhikkhunis, SN 5.2", []string{"gamma one", "gamma two", "gamma three"}),
	}
	got := ParseCanonical(RenderExportFile(qs))
	if len(got) != len(qs) {
		t.Fatalf("got %d quotes, want %d", len(got), len(qs))
	}
	for i := range qs {
		if !quotesEqual(got[i], qs[i]) {
			t.Errorf("quote %d mismatch:\n got  %+v\n want %+v", i, got[i], qs[i])
		}
	}
}

func TestParseCanonicalSkipsBlockMissingCitation(t *testing.T) {
	in := "*a passage with no citation*\n\n\n.  \n.  \n.\n\n\n*real* - **the Buddha, MN 1**"
	got := ParseCanonical(in)
	if len(got) != 1 {
		t.Fatalf("got %d quotes, want 1 (uncited block skipped)", len(got))
	}
	if got[0].SuttaID != "MN 1" {
		t.Errorf("SuttaID = %q, want MN 1", got[0].SuttaID)
	}
}

func TestParseCanonicalEmpty(t *testing.T) {
	for _, in := range []string{"", "   ", "just some prose\nwith no quotes"} {
		if got := ParseCanonical(in); len(got) != 0 {
			t.Errorf("ParseCanonical(%q) = %d quotes, want 0", in, len(got))
		}
	}
}

func TestParseCanonicalPlainTailFallback(t *testing.T) {
	// A lightly edited paste without the bold markers still imports.
	got := ParseCanonical("*a passage* - the Buddha, MN 9")
	if len(got) != 1 {
		t.Fatalf("got %d, want 1", len(got))
	}
	if got[0].SuttaID != "MN 9" {
		t.Errorf("SuttaID = %q, want MN 9", got[0].SuttaID)
	}
}

func TestParseCanonicalSeedExport(t *testing.T) {
	data, err := os.ReadFile("../../exports/shortest-first.md")
	if err != nil {
		t.Skipf("seed export not present: %v", err)
	}
	got := ParseCanonical(string(data))
	if len(got) != 109 {
		t.Errorf("got %d quotes from the seed export, want 109", len(got))
	}
}
