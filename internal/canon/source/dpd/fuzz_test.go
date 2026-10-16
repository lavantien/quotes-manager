package dpd

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func FuzzDpdInflections(f *testing.F) {
	seeds := []string{
		"dhammo, dhammā, dhammaṃ",
		"evaṃ",
		" a ,, b ",
		"",
		`["dhamma","ca"]`,
		`[]`,
		"null",
		`["unterminated`,
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		parts := SplitInflections(s)
		for _, p := range parts {
			if p == "" {
				t.Fatalf("SplitInflections(%q) returned an empty element: %q", s, parts)
			}
			if p != strings.TrimSpace(p) {
				t.Fatalf("SplitInflections(%q) returned untrimmed element %q", s, p)
			}
		}
		joined := SplitInflections(strings.Join(parts, ","))
		if !reflect.DeepEqual(joined, parts) {
			t.Fatalf("SplitInflections not stable under comma join: %q -> %q -> %q", s, parts, joined)
		}

		list := UnpackList(s)
		if list == nil {
			return
		}
		b, err := json.Marshal(list)
		if err != nil {
			t.Fatalf("marshal %q: %v", list, err)
		}
		if again := UnpackList(string(b)); !reflect.DeepEqual(again, list) {
			t.Fatalf("UnpackList not stable through JSON: %q -> %s -> %q", list, b, again)
		}
	})
}
