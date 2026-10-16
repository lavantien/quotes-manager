package bilara

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lavantien/quotes-manager/internal/canon/model"
)

func parse(t *testing.T, json string) (Result, error) {
	t.Helper()
	return ParseSegments(strings.NewReader(json), model.StreamRoot, "mn1")
}

func TestParseSegmentsOrderPreserved(t *testing.T) {
	in := `{"mn1:9.1":"i","mn1:1.1":"a","mn1:5.1":"m","mn1:3.1":"x","mn1:7.1":"q"}`
	res, err := parse(t, in)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"mn1:9.1", "mn1:1.1", "mn1:5.1", "mn1:3.1", "mn1:7.1"}
	if len(res.Records) != len(want) {
		t.Fatalf("got %d records; want %d", len(res.Records), len(want))
	}
	for i, k := range want {
		got := res.Records[i].Muid + ":" + res.Records[i].SegKey
		if got != k {
			t.Errorf("record[%d] = %q; want %q", i, got, k)
		}
	}
}

func TestParseSegmentsDuplicates(t *testing.T) {
	res, err := parse(t, `{"mn1:1.1":"first","mn1:1.2":"mid","mn1:1.1":"second","mn1:1.1":"third"}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Records) != 2 {
		t.Fatalf("got %d records; want 2 (first wins)", len(res.Records))
	}
	if res.Records[0].Text != "first" {
		t.Errorf("dup key must keep the first value, got %q", res.Records[0].Text)
	}
	if len(res.DupKeys) != 2 || res.DupKeys[0] != "mn1:1.1" || res.DupKeys[1] != "mn1:1.1" {
		t.Errorf("DupKeys = %v; want [mn1:1.1 mn1:1.1]", res.DupKeys)
	}
}

func TestParseSegmentsEmptyValuesKept(t *testing.T) {
	res, err := parse(t, `{"mn1:0.1":"","mn1:0.2":"x"}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Records) != 2 || res.Records[0].Text != "" || res.Records[0].SegKey != "0.1" {
		t.Fatalf("empty values must be kept verbatim, got %+v", res.Records)
	}
}

func TestParseSegmentsMarkupVerbatim(t *testing.T) {
	val := "*em* _root_ **bold** #counter “quoted” a—b …pe… {}"
	res, err := parse(t, `{"mn1:1.1":"`+val+`"}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Records) != 1 || res.Records[0].Text != val {
		t.Fatalf("markup must survive verbatim, got %+v", res.Records)
	}
	if res.NFCChanged {
		t.Error("NFCChanged must be false for NFC-clean markup")
	}
}

func TestParseSegmentsNFCChange(t *testing.T) {
	res, err := parse(t, "{\"mn1:1.1\":\"ā\"}")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Records) != 1 || res.Records[0].Text != "ā" {
		t.Fatalf("value must be NFC-composed, got %+v", res.Records)
	}
	if !res.NFCChanged {
		t.Error("NFCChanged must be true when a value changes form")
	}

	res, err = parse(t, "{\"ā:1.1\":\"x\"}")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Records) != 1 || res.Records[0].Muid != "ā" || res.Records[0].SegKey != "1.1" {
		t.Fatalf("key must be NFC-composed before splitting, got %+v", res.Records)
	}
	if !res.NFCChanged {
		t.Error("NFCChanged must be true when a key changes form")
	}
	if len(res.PrefixMismatches) != 1 || res.PrefixMismatches[0] != "ā:1.1" {
		t.Errorf("PrefixMismatches = %v; want [ā:1.1]", res.PrefixMismatches)
	}

	res, err = parse(t, "{\"mn1:1.1\":\"clean\",\"mn1:1.1\":\"ā\"}")
	if err != nil {
		t.Fatal(err)
	}
	if res.NFCChanged {
		t.Error("NFCChanged reflects emitted records only")
	}
	if res.Records[0].Text != "clean" {
		t.Errorf("first value must win, got %q", res.Records[0].Text)
	}
}

func TestParseSegmentsBOM(t *testing.T) {
	res, err := ParseSegments(strings.NewReader("\uFEFF{\"mn1:1.1\":\"x\"}"), model.StreamRoot, "mn1")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Records) != 1 || res.Records[0].Text != "x" {
		t.Fatalf("BOM must be skipped, got %+v err %v", res.Records, err)
	}
}

func TestParseSegmentsInvalidUTF8(t *testing.T) {
	cases := [][]byte{
		[]byte("{\"mn1:1.1\":\"\xff\"}"),
		{0xEF, 0xBB, 0xBF, '{', '"', 'a', '"', ':', 0xFF, '}'},
		[]byte("{\"mn1:1.1\":\"ok\",\"mn1:1.2\":\"\xc3\"}"),
		[]byte("{\"mn1:1.1\":\"split-\xe0\xa4\"}"),
		[]byte("{\"\x80\":\"v\"}"),
		{0x80},
		{0xE1},
	}
	for _, in := range cases {
		if _, err := ParseSegments(bytes.NewReader(in), model.StreamRoot, "mn1"); err == nil {
			t.Errorf("invalid UTF-8 input %q must be a hard error", in)
		}
	}
}

func TestParseSegmentsChunkBoundaries(t *testing.T) {
	const prefix = 12
	cases := []struct {
		name string
		pad  int
		rune string
	}{
		{"3byte straddles", 4096 - prefix - 2, "ṁ"},
		{"4byte straddles", 4096 - prefix - 3, "😀"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := `{"mn1:1.1":"` + strings.Repeat("a", tc.pad) + tc.rune + `b"}`
			if last := prefix + tc.pad + len(tc.rune) - 1; last < 4096 {
				t.Fatalf("test construction: rune ends at %d, want >= 4096", last)
			}
			res, err := ParseSegments(strings.NewReader(in), model.StreamRoot, "mn1")
			if err != nil {
				t.Fatalf("valid input straddling the chunk boundary must parse: %v", err)
			}
			want := strings.Repeat("a", tc.pad) + tc.rune + "b"
			if res.Records[0].Text != want {
				t.Errorf("text mangled across boundary: got %q tail", res.Records[0].Text[len(res.Records[0].Text)-8:])
			}
		})
	}

	pad := 4096 - prefix - 3 - 2
	in := `{"mn1:1.1":"` + strings.Repeat("a", pad) + "ṁ" + `"}`
	if len(in) != 4096 {
		t.Fatalf("test construction: input is %d bytes, want 4096", len(in))
	}
	res, err := ParseSegments(strings.NewReader(in), model.StreamRoot, "mn1")
	if err != nil {
		t.Fatalf("valid 4096-byte input ending in a complete rune must parse: %v", err)
	}
	if want := strings.Repeat("a", pad) + "ṁ"; res.Records[0].Text != want {
		t.Errorf("text mangled: got %q", res.Records[0].Text)
	}
}

func TestParseSegmentsTruncated(t *testing.T) {
	for _, in := range []string{
		`{"mn1:1.1":"x"`,
		`{"mn1:1.1":`,
		`{"mn1:1.1"`,
		`{`,
		``,
	} {
		if _, err := parse(t, in); err == nil {
			t.Errorf("truncated input %q must be a hard error", in)
		}
	}
}

func TestParseSegmentsBadValues(t *testing.T) {
	for _, in := range []string{
		`{"mn1:1.1":5}`,
		`{"mn1:1.1":null}`,
		`{"mn1:1.1":{"nested":"object"}}`,
		`{"mn1:1.1":["array"]}`,
		`{"mn1:1.1":true}`,
	} {
		if _, err := parse(t, in); err == nil {
			t.Errorf("non-string value in %q must be a hard error", in)
		}
	}
}

func TestParseSegmentsTopLevelNotObject(t *testing.T) {
	for _, in := range []string{
		`["mn1:1.1"]`,
		`5`,
		`"mn1:1.1"`,
		`null`,
		`true`,
	} {
		_, err := parse(t, in)
		if err == nil {
			t.Errorf("non-object top level %q must be a hard error", in)
			continue
		}
		if !strings.Contains(err.Error(), "top level") {
			t.Errorf("input %q: error %q must name the top-level problem", in, err)
		}
	}
	for _, in := range []string{`]`, `}`} {
		if _, err := parse(t, in); err == nil {
			t.Errorf("invalid opener %q must be a hard error", in)
		}
	}
}

func TestParseSegmentsTrailingData(t *testing.T) {
	for _, in := range []string{
		`{"mn1:1.1":"x"} trailing`,
		`{"mn1:1.1":"x"}{"mn1:1.2":"y"}`,
		`{"mn1:1.1":"x"} 5`,
		`{"mn1:1.1":"x"]`,
	} {
		if _, err := parse(t, in); err == nil {
			t.Errorf("trailing data after %q must be a hard error", in)
		}
	}
}

func TestParseSegmentsEmptyObject(t *testing.T) {
	res, err := parse(t, `{}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Records) != 0 || len(res.DupKeys) != 0 || len(res.PrefixMismatches) != 0 || res.NFCChanged {
		t.Fatalf("empty object must parse to an empty result, got %+v", res)
	}
}

func TestParseSegmentsPrefixMismatch(t *testing.T) {
	res, err := parse(t, `{"mn1:1.1":"a","mn2:2.1":"b",":3.1":"c","3.1":"d"}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Records) != 4 {
		t.Fatalf("mismatched keys must still produce records, got %d", len(res.Records))
	}
	wantMuids := []string{"mn1", "mn2", "", "mn1"}
	wantSegs := []string{"1.1", "2.1", "3.1", "3.1"}
	for i, rec := range res.Records {
		if rec.Muid != wantMuids[i] || rec.SegKey != wantSegs[i] {
			t.Errorf("record[%d] = %s:%s; want %s:%s", i, rec.Muid, rec.SegKey, wantMuids[i], wantSegs[i])
		}
	}
	want := []string{"mn2:2.1", ":3.1", "3.1"}
	if len(res.PrefixMismatches) != len(want) {
		t.Fatalf("PrefixMismatches = %v; want %v", res.PrefixMismatches, want)
	}
	for i, k := range want {
		if res.PrefixMismatches[i] != k {
			t.Errorf("PrefixMismatches[%d] = %q; want %q", i, res.PrefixMismatches[i], k)
		}
	}
}

func TestParseSegmentsStreamCarried(t *testing.T) {
	in := `{"mn1:1.1":"x"}`
	for _, s := range []model.Stream{model.StreamRoot, model.StreamTranslation, model.StreamHTML, model.StreamReference} {
		res, err := ParseSegments(strings.NewReader(in), s, "mn1")
		if err != nil {
			t.Fatal(err)
		}
		if res.Records[0].Stream != s {
			t.Errorf("record stream = %s; want %s", res.Records[0].Stream, s)
		}
	}
}

func realFile(t *testing.T, rel string) (Result, File) {
	t.Helper()
	f := File{Stream: model.StreamRoot, Muid: "mn1", Path: filepath.Join("..", "..", "testdata", "bilara", filepath.FromSlash(rel))}
	data, err := os.ReadFile(f.Path)
	if err != nil {
		t.Fatal(err)
	}
	res, err := ParseSegments(bytes.NewReader(data), f.Stream, f.Muid)
	if err != nil {
		t.Fatalf("%s: %v", f.Path, err)
	}
	return res, f
}

func TestParseSegmentsRealQuartet(t *testing.T) {
	for _, rel := range []string{
		"html/pli/ms/sutta/mn/mn1_html.json",
		"reference/pli/ms/sutta/mn/mn1_reference.json",
		"root/pli/ms/sutta/mn/mn1_root-pli-ms.json",
		"translation/en/sujato/sutta/mn/mn1_translation-en-sujato.json",
	} {
		res, f := realFile(t, rel)
		if len(res.Records) == 0 || len(res.DupKeys) != 0 || len(res.PrefixMismatches) != 0 {
			t.Fatalf("%s: got %d records, dups %v, mismatches %v", f.Path, len(res.Records), res.DupKeys, res.PrefixMismatches)
		}
		for _, rec := range res.Records {
			if rec.Muid != "mn1" || rec.Stream != model.StreamRoot || rec.SegKey == "" {
				t.Fatalf("%s: bad record %+v", f.Path, rec)
			}
		}
		if res.Records[0].SegKey != "0.1" {
			t.Errorf("%s: first record segkey = %q; want 0.1", f.Path, res.Records[0].SegKey)
		}
	}

	rootRes, _ := realFile(t, "root/pli/ms/sutta/mn/mn1_root-pli-ms.json")
	if got := rootRes.Records[0].Text; got != "Majjhima Nikāya 1 " {
		t.Errorf("root first value = %q; want %q", got, "Majjhima Nikāya 1 ")
	}
	htmlRes, _ := realFile(t, "html/pli/ms/sutta/mn/mn1_html.json")
	if !strings.Contains(htmlRes.Records[0].Text, "{}") {
		t.Errorf("html first value must keep the {} placeholder, got %q", htmlRes.Records[0].Text)
	}
	refRes, _ := realFile(t, "reference/pli/ms/sutta/mn/mn1_reference.json")
	if got := refRes.Records[0].Text; got != "" {
		t.Errorf("reference mn1:0.1 must be the empty string, got %q", got)
	}
	if got := refRes.Records[2].Text; !strings.HasPrefix(got, "bj10.2, cck12.1") {
		t.Errorf("reference mn1:1.1 must carry the edition list verbatim, got %q", got)
	}
}

func TestParseNames(t *testing.T) {
	nf, err := ParseNames(strings.NewReader(`{"mn":"Majjhima Nikāya","mn1":"Mūlapariyāyasutta"}`))
	if err != nil {
		t.Fatal(err)
	}
	if nf["mn"] != "Majjhima Nikāya" || nf["mn1"] != "Mūlapariyāyasutta" || len(nf) != 2 {
		t.Fatalf("ParseNames = %v", nf)
	}

	nf, err = ParseNames(strings.NewReader("\uFEFF{\"a\":\"ā\"}"))
	if err != nil {
		t.Fatal(err)
	}
	if nf["a"] != "ā" {
		t.Fatalf("names must be NFC-composed, got %q", nf["a"])
	}

	nf, err = ParseNames(strings.NewReader(`{"mn":"first","mn":"second"}`))
	if err != nil {
		t.Fatal(err)
	}
	if nf["mn"] != "first" {
		t.Fatalf("duplicate name must keep the first value, got %q", nf["mn"])
	}

	for _, in := range []string{
		`{"mn":5}`,
		`{"mn":{"nested":"x"}}`,
		`["mn"]`,
		`{"mn":"a"`,
	} {
		if _, err := ParseNames(strings.NewReader(in)); err == nil {
			t.Errorf("ParseNames(%q) must be a hard error", in)
		}
	}
}
