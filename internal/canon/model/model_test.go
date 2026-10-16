package model

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestDpdChecksKnown(t *testing.T) {
	if CheckDpdUnknownForms != "D001" || CheckDpdDegraded != "D002" {
		t.Fatalf("dpd check ids = %q, %q; want D001, D002", CheckDpdUnknownForms, CheckDpdDegraded)
	}
	for _, id := range []CheckID{CheckDpdUnknownForms, CheckDpdDegraded} {
		if !id.Known() {
			t.Errorf("check %q must be known", id)
		}
	}
	known := KnownChecks()
	found := 0
	for _, id := range known {
		if id == CheckDpdUnknownForms || id == CheckDpdDegraded {
			found++
		}
	}
	if found != 2 {
		t.Errorf("KnownChecks must list both dpd checks, found %d of 2", found)
	}
	if CheckID("D003").Known() {
		t.Error("D003 must not be known")
	}
	if len(KnownChecks()) != len(known) {
		t.Error("KnownChecks must return a copy")
	}
}

func TestDpdNodeAndEdgeTypes(t *testing.T) {
	if NodeHeadword != NodeType("headword") || NodeDpdRoot != NodeType("dpd_root") || NodeDpdForm != NodeType("dpd_form") {
		t.Errorf("node type values changed: %q %q %q", NodeHeadword, NodeDpdRoot, NodeDpdForm)
	}
	if EdgeDerivedFrom != EdgeType("DERIVED_FROM") || EdgeHasWord != EdgeType("HAS_WORD") {
		t.Errorf("edge type values changed: %q %q", EdgeDerivedFrom, EdgeHasWord)
	}
}

func jsonKeys(t *testing.T, b []byte) map[string]bool {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	keys := make(map[string]bool, len(m))
	for k := range m {
		keys[k] = true
	}
	return keys
}

func TestDpdRecordJSONRoundTrip(t *testing.T) {
	t.Run("headword", func(t *testing.T) {
		in := DpdHeadword{
			Lemma1: "dhamma", Lemma2: "dhammā", Pos: "masculine", Grammar: "m",
			Meaning1: "the teaching", MeaningLit: "that which holds", Sanskrit: "dharma",
			RootKey: "ḷī", RootSign: "i", FamilyRoot: "ḷī", Construction: "dha + ma",
			CompoundType: "kammadhāraya", Stem: "dhamma",
		}
		b, err := json.Marshal(in)
		if err != nil {
			t.Fatal(err)
		}
		want := map[string]bool{
			"lemma_1": true, "lemma_2": true, "pos": true, "grammar": true,
			"meaning_1": true, "meaning_lit": true, "sanskrit": true,
			"root_key": true, "root_sign": true, "family_root": true,
			"construction": true, "compound_type": true, "stem": true,
		}
		if got := jsonKeys(t, b); !reflect.DeepEqual(got, want) {
			t.Errorf("headword keys = %v, want %v", got, want)
		}
		var out DpdHeadword
		if err := json.Unmarshal(b, &out); err != nil {
			t.Fatal(err)
		}
		if out != in {
			t.Errorf("round trip changed headword: %+v", out)
		}
	})
	t.Run("root", func(t *testing.T) {
		in := DpdRoot{
			Root: "ḷī", RootInComps: "līy", RootHasVerb: "･", RootGroup: "1",
			RootSign: "i", RootMeaning: "to stick", SanskritRoot: "lī",
		}
		b, err := json.Marshal(in)
		if err != nil {
			t.Fatal(err)
		}
		want := map[string]bool{
			"root": true, "root_in_comps": true, "root_has_verb": true,
			"root_group": true, "root_sign": true, "root_meaning": true,
			"sanskrit_root": true,
		}
		if got := jsonKeys(t, b); !reflect.DeepEqual(got, want) {
			t.Errorf("root keys = %v, want %v", got, want)
		}
		var out DpdRoot
		if err := json.Unmarshal(b, &out); err != nil {
			t.Fatal(err)
		}
		if out != in {
			t.Errorf("round trip changed root: %+v", out)
		}
	})
	t.Run("form", func(t *testing.T) {
		in := DpdForm{Form: "dhammassa", Headwords: []string{"dhamma"}, Splits: []string{"dhamma", "assa"}}
		b, err := json.Marshal(in)
		if err != nil {
			t.Fatal(err)
		}
		want := map[string]bool{"form": true, "headwords": true, "splits": true}
		if got := jsonKeys(t, b); !reflect.DeepEqual(got, want) {
			t.Errorf("form keys = %v, want %v", got, want)
		}
		var out DpdForm
		if err := json.Unmarshal(b, &out); err != nil {
			t.Fatal(err)
		}
		if out.Form != in.Form || !reflect.DeepEqual(out.Headwords, in.Headwords) || !reflect.DeepEqual(out.Splits, in.Splits) {
			t.Errorf("round trip changed form: %+v", out)
		}
	})
}
