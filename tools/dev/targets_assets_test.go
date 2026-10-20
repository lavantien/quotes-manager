package main

import (
	"reflect"
	"testing"
)

func TestVendoredAssetPinsMatchMakefile(t *testing.T) {
	assets := vendoredAssets()
	if len(assets) != 2 {
		t.Fatalf("assets len = %d, want 2", len(assets))
	}
	htmx := assets[0]
	if htmx.Pkg != "htmx.org" || htmx.Version != "4.0.0" || htmx.File != "htmx.min.js" {
		t.Fatalf("htmx asset = %+v", htmx)
	}
	if htmx.SHA256 != "e484d9171a9db30a39c8f16e3d709d4137f3211c659f8e6125816635033d593f" {
		t.Fatalf("htmx sha = %q", htmx.SHA256)
	}
	cy := assets[1]
	if cy.Pkg != "cytoscape" || cy.Version != "3.34.4" || cy.File != "cytoscape.min.js" {
		t.Fatalf("cytoscape asset = %+v", cy)
	}
	if cy.SHA256 != "081591ab735b2b6a3b31a3c60f8f3a3e2fee8c92939e0c7820ad1208571b6cd8" {
		t.Fatalf("cytoscape sha = %q", cy.SHA256)
	}
}

func TestCompareVersionsSortsLikeMake(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"4.0.0", "3.9.9", 1},
		{"3.34.4", "3.34.10", -1},
		{"3.34.4", "3.34.4", 0},
		{"4.0", "4.0.1", -1},
		{"10.0.0", "9.0.0", 1},
	}
	for _, c := range cases {
		got := compareVersions(c.a, c.b)
		if (got > 0) != (c.want > 0) || (got < 0) != (c.want < 0) {
			t.Fatalf("compareVersions(%q, %q) = %d, want sign %d", c.a, c.b, got, c.want)
		}
	}
}

func TestIsStableVersion(t *testing.T) {
	for _, ok := range []string{"1.2.3", "0.1.0", "10.20.30"} {
		if !isStableVersion(ok) {
			t.Fatalf("isStableVersion(%q) = false, want true", ok)
		}
	}
	for _, bad := range []string{"", "next", "4.0.0-beta.1", "latest", "x.y.z"} {
		if isStableVersion(bad) {
			t.Fatalf("isStableVersion(%q) = true, want false", bad)
		}
	}
}

func TestSha256Bytes(t *testing.T) {
	got := sha256Bytes([]byte("abc"))
	want := "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"
	if got != want {
		t.Fatalf("sha256Bytes = %q, want %q", got, want)
	}
}

func TestSplitCommaDropsEmpty(t *testing.T) {
	if got := splitComma("a,,b,"); !reflect.DeepEqual(got, []string{"a", "b"}) {
		t.Fatalf("splitComma = %v", got)
	}
}
