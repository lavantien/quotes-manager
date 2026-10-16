package bilara

import "testing"

func TestAllowedMuids(t *testing.T) {
	has := func(set map[string]bool, muids ...string) {
		t.Helper()
		for _, m := range muids {
			if !set[m] {
				t.Errorf("muid %q missing from allowed set %v", m, set)
			}
		}
	}
	lacks := func(set map[string]bool, muids ...string) {
		t.Helper()
		for _, m := range muids {
			if set[m] {
				t.Errorf("muid %q must not be allowed in %v", m, set)
			}
		}
	}

	plain := AllowedMuids("mn1", 500)
	has(plain, "mn1")
	lacks(plain, "mn2", "mn1.1")

	grouped := AllowedMuids("an1.1-10", 500)
	has(grouped, "an1.1", "an1.5", "an1.10", "an1.1-10")
	lacks(grouped, "an1.0", "an1.11", "an2.1")

	vin := AllowedMuids("pli-tv-bu-vb-as1-7", 500)
	has(vin, "pli-tv-bu-vb-as1", "pli-tv-bu-vb-as7")
	lacks(vin, "pli-tv-bu-vb-as8", "pli-tv-bu-vb-pj1")

	dashed := AllowedMuids("pli-tv-bu-pm", 500)
	has(dashed, "pli-tv-bu-pm")
	lacks(dashed, "pli-tv-bu-p")

	lacks(AllowedMuids("an5.9-3", 500), "an5.9")
	lacks(AllowedMuids("dhp1-900", 500), "dhp1")
	has(AllowedMuids("dhp1-20", 500), "dhp1", "dhp20")

	has(AllowedMuids("an5.10-509", 500), "an5.509")
	lacks(AllowedMuids("dhp1-501", 500), "dhp400")
	lacks(AllowedMuids("dhp1-900", 500), "dhp500")

	lacks(AllowedMuids("an1.9999999999999999999999-5", 500), "an1.0", "an1.5")
	lacks(AllowedMuids("an1.0-9999999999999999999999", 500), "an1.0")
}
