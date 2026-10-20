package server

import (
	"net/url"
	"strings"
	"testing"
)

func FuzzSuttaRef(f *testing.F) {
	f.Add("mn26")
	f.Add("MN 26")
	f.Add("an2.130")
	f.Add("pli-tv-bu-vb-pj1")
	f.Add("vv51")
	f.Add("")
	f.Add("   ")
	f.Add("MN 26 #4")
	f.Add("KN Dhp 5")
	f.Add("tvvibupj1")
	f.Add("%00%1b[mn1")
	f.Fuzz(func(t *testing.T, muid string) {
		got, display, link, err := suttaRef(muid)
		if err != nil {
			return
		}
		if got == "" || display == "" || link == "" {
			t.Fatalf("suttaRef(%q) = %q %q %q, want all non-empty", muid, got, display, link)
		}
		if strings.ContainsAny(got, " \t\r\n") {
			t.Fatalf("suttaRef(%q) muid = %q, want compact", muid, got)
		}
		if rtMuid, _, _, rerr := suttaRef(got); rerr != nil || rtMuid != got {
			t.Fatalf("suttaRef(%q) = %q does not round-trip: %q %v", muid, got, rtMuid, rerr)
		}
		if !strings.HasPrefix(link, "https://suttacentral.net/"+got+"/") {
			t.Fatalf("suttaRef(%q) link = %q, want the canonical prefix", muid, link)
		}
	})
}

func FuzzGroupPayloads(f *testing.F) {
	f.Add("group=1", "renamed")
	f.Add("group=0", "")
	f.Add("group=-5", "%20")
	f.Add("group=abc", "wise%20attention")
	f.Add("group=99999999999999999999", "x%")
	f.Add("group=1", "%000")
	f.Add("", "a b")
	f.Fuzz(func(t *testing.T, form, prompt string) {
		values, err := url.ParseQuery(form)
		if err != nil {
			return
		}
		gid, ok := parseGroupID(values)
		if ok && gid < 0 {
			t.Fatalf("parseGroupID(%q) = %d, want non-negative", form, gid)
		}
		name := promptValueOf(prompt)
		if strings.ContainsRune(name, '\x00') && validGroupName(name) {
			t.Fatalf("validGroupName accepts the NUL-bearing prompt %q", prompt)
		}
		if validGroupName(name) && strings.TrimSpace(name) == "" {
			t.Fatalf("validGroupName accepts the whitespace-only prompt %q", prompt)
		}
	})
}
