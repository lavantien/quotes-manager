package sutta

import "testing"

func TestKnownMuid(t *testing.T) {
	known := []string{
		"mn26", "an2.130", "sn12.1", "dn15", "dhp209", "snp4.2", "ud1.10",
		"iti1", "thag1.1", "thig1.1", "khp1", "ja360", "vv51",
		"pli-tv-bu-vb-pj1", "pli-tv-pi-vb-pj1",
	}
	for _, muid := range known {
		if !KnownMuid(muid) {
			t.Errorf("KnownMuid(%q) = false, want true", muid)
		}
	}
	unknown := []string{"zz9", "mn", "an", "vv", "dhp", "x1y2", "pli-tv", "mn26x", "", "mn 26"}
	for _, muid := range unknown {
		if KnownMuid(muid) {
			t.Errorf("KnownMuid(%q) = true, want false", muid)
		}
	}
}
