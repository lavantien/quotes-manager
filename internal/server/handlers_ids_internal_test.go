package server

import (
	"reflect"
	"testing"
)

func TestUniqueSortedIDs(t *testing.T) {
	cases := []struct {
		name string
		in   []string
		want []string
	}{
		{"nil", nil, []string{}},
		{"empty", []string{}, []string{}},
		{"single", []string{"MN 22"}, []string{"MN 22"}},
		{"dups collapse", []string{"MN 22", "MN 22", "AN 5"}, []string{"AN 5", "MN 22"}},
		{"unsorted sorted", []string{"SN 12", "AN 5", "MN 22"}, []string{"AN 5", "MN 22", "SN 12"}},
		{"empties dropped", []string{"", "MN 22", "", ""}, []string{"MN 22"}},
		{"byte-order case-sensitive", []string{"an", "MN", "AN"}, []string{"AN", "MN", "an"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := uniqueSortedIDs(c.in); !reflect.DeepEqual(got, c.want) {
				t.Errorf("uniqueSortedIDs(%v) = %v, want %v", c.in, got, c.want)
			}
		})
	}
}
