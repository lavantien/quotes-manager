package server

import (
	"reflect"
	"testing"

	"github.com/lavantien/quotes-manager/internal/store"
)

func TestSplitIDLines(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []string
	}{
		{"empty", "", nil},
		{"whitespace only", "   ", nil},
		{"single", "MN 22", []string{"MN 22"}},
		{"two lines", "MN 22\nAN 5", []string{"MN 22", "AN 5"}},
		{"crlf", "MN 22\r\nAN 5", []string{"MN 22", "AN 5"}},
		{"trailing newline", "MN 22\n", []string{"MN 22"}},
		{"surrounding spaces", "  MN 22  \n AN 5 ", []string{"MN 22", "AN 5"}},
		{"blank middle line", "MN 22\n\nAN 5", []string{"MN 22", "AN 5"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := splitIDLines(c.in); !reflect.DeepEqual(got, c.want) {
				t.Errorf("splitIDLines(%q) = %v, want %v", c.in, got, c.want)
			}
		})
	}
}

func TestRunCheck(t *testing.T) {
	corpus := []store.Quote{
		{ID: 1, SuttaID: "MN 22"},
		{ID: 2, SuttaID: "MN 22"},
		{ID: 3, SuttaID: "AN 5"},
	}
	cases := []struct {
		name   string
		corpus []store.Quote
		inputs []string
		want   []checkResult
	}{
		{"empty corpus empty inputs", nil, nil, nil},
		{"corpus empty inputs", corpus, nil, nil},
		{"found with count", corpus, []string{"MN 22"}, []checkResult{{Input: "MN 22", ID: "MN 22", Found: true, Count: 2}}},
		{"not found", corpus, []string{"SN 1"}, []checkResult{{Input: "SN 1", ID: "SN 1", Found: false, Count: 0}}},
		{"case insensitive", corpus, []string{"mn 22"}, []checkResult{{Input: "mn 22", ID: "mn 22", Found: true, Count: 2}}},
		{"canonical extraction", corpus, []string{"the Buddha, MN 22"}, []checkResult{{Input: "the Buddha, MN 22", ID: "MN 22", Found: true, Count: 2}}},
		{"non-canonical literal misses", corpus, []string{"MN22"}, []checkResult{{Input: "MN22", ID: "MN22", Found: false, Count: 0}}},
		{"dup inputs each reported", corpus, []string{"AN 5", "AN 5"}, []checkResult{
			{Input: "AN 5", ID: "AN 5", Found: true, Count: 1},
			{Input: "AN 5", ID: "AN 5", Found: true, Count: 1},
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := runCheck(c.corpus, c.inputs); !reflect.DeepEqual(got, c.want) {
				t.Errorf("runCheck(...) = %+v, want %+v", got, c.want)
			}
		})
	}
}
