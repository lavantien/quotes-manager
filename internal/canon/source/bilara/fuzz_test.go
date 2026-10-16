package bilara

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/lavantien/quotes-manager/internal/canon/model"
	"github.com/lavantien/quotes-manager/internal/canon/norm"
)

var seedFiles = []string{
	"root/pli/ms/sutta/mn/mn1_root-pli-ms.json",
	"translation/en/sujato/sutta/mn/mn1_translation-en-sujato.json",
	"html/pli/ms/sutta/mn/mn1_html.json",
	"reference/pli/ms/sutta/mn/mn1_reference.json",
}

func FuzzBilaraSegmentJSON(f *testing.F) {
	for _, rel := range seedFiles {
		data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "bilara", filepath.FromSlash(rel)))
		if err != nil {
			f.Fatal(err)
		}
		f.Add(data)
	}
	f.Add([]byte(`{"mn1:1.1":"first","mn1:1.1":"second"}`))
	f.Add([]byte(`{}`))
	f.Add([]byte("\xef\xbb\xbf{\"mn1:1.1\":\"a\xcc\x84\"}"))
	f.Add([]byte("{\"mn1:1.1\":\"\xff\"}"))

	f.Fuzz(func(t *testing.T, data []byte) {
		res, err := ParseSegments(bytes.NewReader(data), model.StreamRoot, "mn1")
		if err != nil {
			return
		}
		o, ok := segmentOracle(data)
		if !ok {
			return
		}

		if len(res.Records)+len(res.DupKeys) != o.tokens {
			t.Fatalf("records %d + dups %d != key tokens %d", len(res.Records), len(res.DupKeys), o.tokens)
		}
		if len(res.Records) != len(o.keys) {
			t.Fatalf("records %d != distinct keys %d", len(res.Records), len(o.keys))
		}
		for i, rec := range res.Records {
			key := o.keys[i]
			if rec.Text != o.first[key] {
				t.Errorf("record %d (%q): text %q != first-win oracle %q", i, key, rec.Text, o.first[key])
			}
			if keyMuid, _, found := strings.Cut(key, ":"); found {
				if rec.Muid != keyMuid || rec.Muid+":"+rec.SegKey != key {
					t.Errorf("record %d (%q): split %s:%s", i, key, rec.Muid, rec.SegKey)
				}
			}
		}
		for _, k := range res.DupKeys {
			if o.counts[k] < 2 {
				t.Errorf("DupKeys entry %q occurs %d times in oracle", k, o.counts[k])
			}
		}
		if res.NFCChanged != o.nfcChanged {
			t.Errorf("NFCChanged = %v; oracle says %v", res.NFCChanged, o.nfcChanged)
		}
		gotMismatch := make(map[string]bool, len(res.PrefixMismatches))
		for _, k := range res.PrefixMismatches {
			if gotMismatch[k] {
				t.Errorf("PrefixMismatches %v repeats %q", res.PrefixMismatches, k)
			}
			gotMismatch[k] = true
		}
		if len(gotMismatch) != len(o.mismatch) {
			t.Fatalf("PrefixMismatches %v != oracle %v", res.PrefixMismatches, o.mismatch)
		}
		for k := range o.mismatch {
			if !gotMismatch[k] {
				t.Errorf("PrefixMismatches missing oracle mismatch %q", k)
			}
		}
	})
}

type segOracle struct {
	tokens     int
	keys       []string
	first      map[string]string
	counts     map[string]int
	mismatch   map[string]bool
	nfcChanged bool
}

func segmentOracle(data []byte) (segOracle, bool) {
	var o segOracle
	if !utf8.Valid(data) {
		return o, false
	}
	trimmed := bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
	dec := json.NewDecoder(bytes.NewReader(trimmed))
	tok, err := dec.Token()
	if err != nil {
		return o, false
	}
	if d, ok := tok.(json.Delim); !ok || d != '{' {
		return o, false
	}
	o.first = map[string]string{}
	o.counts = map[string]int{}
	o.mismatch = map[string]bool{}
	for dec.More() {
		keyTok, err := dec.Token()
		if err != nil {
			return o, false
		}
		key, ok := keyTok.(string)
		if !ok {
			return o, false
		}
		valTok, err := dec.Token()
		if err != nil {
			return o, false
		}
		val, ok := valTok.(string)
		if !ok {
			return o, false
		}
		o.tokens++
		key, kChanged := norm.NFC(key)
		val, vChanged := norm.NFC(val)
		o.counts[key]++
		if o.counts[key] == 1 {
			o.keys = append(o.keys, key)
			o.first[key] = val
			if kChanged || vChanged {
				o.nfcChanged = true
			}
		}
		muid, _, found := strings.Cut(key, ":")
		if !found || muid != "mn1" {
			o.mismatch[key] = true
		}
	}
	if _, err := dec.Token(); err != nil {
		return o, false
	}
	if _, err := dec.Token(); err == nil {
		return o, false
	}
	return o, true
}

func FuzzSegmentKey(f *testing.F) {
	for _, k := range []string{
		"mn1:2.1", "an2.49:1.6", "pli-tv-bu-vb-1:3.2", "mn1:", "mn1:1", ":1.1", "MN1:1.1", "mn1:1.x", "", "1:1.1",
	} {
		f.Add(k)
	}
	f.Fuzz(func(t *testing.T, k string) {
		valid := ValidFullKey(k)
		if k != strings.ToLower(k) && valid {
			t.Errorf("ValidFullKey(%q) = true with uppercase", k)
		}
		if _, changed := norm.NFC(k); valid && changed {
			t.Errorf("ValidFullKey(%q) = true but key is not NFC-stable", k)
		}
		if !valid && k == strings.ToLower(k) && ValidFullKey(strings.ToLower(k)) {
			t.Errorf("ValidFullKey(%q) = false but lowercase form accepted", k)
		}
	})
}
