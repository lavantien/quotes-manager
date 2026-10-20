package sutta

import (
	"os"
	"testing"
)

const realDumpPath = "../../dumps/ariyasavaka-curated-list.md"

func mustParseReal(t *testing.T) Table {
	t.Helper()
	data, err := os.ReadFile(realDumpPath)
	if err != nil {
		t.Fatalf("read artifact: %v", err)
	}
	tab, err := ParseFile("ariyasavaka-curated-list.md", string(data))
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	return tab
}

func groupSuttaMap(tab Table) map[string]int {
	out := make(map[string]int)
	for gi, g := range tab.Groups {
		for _, s := range g.Suttas {
			if _, ok := out[s.Muid]; !ok {
				out[s.Muid] = gi
			}
		}
	}
	return out
}

func TestParseRealGroupNamesInOrder(t *testing.T) {
	tab := mustParseReal(t)
	want := []string{
		"Five Daily Reflections and Six Contemplations",
		"Ethics, Goodwill, Ugliness Perception, and Perception of Impermanence",
		"Senses Restraint",
		"Moderate in Eating",
		"Dedicate to Wakefulness",
		"Four Foundations of Mindfulness",
		"Situational Awareness",
		"Five Corruptions and Parasites",
		"Four Meditations",
		"Four Noble Truths",
		"Noble Eightfold Path",
		"Striving for Celibacy",
		"Analysis of Elements",
		"Meditation Toolkits",
		"Six Sets of Six",
		"Dependent Origination",
	}
	if len(tab.Groups) != len(want) {
		t.Fatalf("groups = %d, want %d", len(tab.Groups), len(want))
	}
	for i, g := range tab.Groups {
		if g.Name != want[i] {
			t.Errorf("group %d name = %q, want %q", i+1, g.Name, want[i])
		}
	}
	if tab.Name != "ariyasavaka-curated-list.md" {
		t.Errorf("table name = %q, want the file name", tab.Name)
	}
}

func TestParseRealGroupSizes(t *testing.T) {
	tab := mustParseReal(t)
	want := []int{86, 60, 20, 12, 14, 2, 6, 5, 4, 1, 24, 228, 1, 2, 1, 5}
	total := 0
	if len(tab.Groups) != len(want) {
		t.Fatalf("groups = %d, want %d", len(tab.Groups), len(want))
	}
	for i, g := range tab.Groups {
		if len(g.Suttas) != want[i] {
			t.Errorf("group %d (%s) size = %d, want %d", i+1, g.Name, len(g.Suttas), want[i])
		}
		total += len(g.Suttas)
	}
	if total != 471 {
		t.Errorf("total suttas = %d, want 471", total)
	}
}

func TestParseRealFirstAndLastSutta(t *testing.T) {
	tab := mustParseReal(t)
	first := tab.Groups[0].Suttas[0]
	if first.Muid != "mn26" || first.Display != "MN 26" {
		t.Errorf("first sutta = %+v, want mn26 MN 26", first)
	}
	wantLink := "https://suttacentral.net/mn26/en/sujato?lang=en&layout=plain&reference=none&notes=none&highlight=true&script=latin"
	if first.Link != wantLink {
		t.Errorf("first link = %q, want %q", first.Link, wantLink)
	}
	last := tab.Groups[len(tab.Groups)-1].Suttas[len(tab.Groups[len(tab.Groups)-1].Suttas)-1]
	if last.Muid != "ud1.10" || last.Display != "Ud 1.10" {
		t.Errorf("last sutta = %+v, want ud1.10 Ud 1.10", last)
	}
}

func TestParseRealHeadingLinksBelongToGroup(t *testing.T) {
	tab := mustParseReal(t)
	vi := tab.Groups[5]
	if len(vi.Suttas) != 2 || vi.Suttas[0].Muid != "mn10" || vi.Suttas[1].Muid != "mn119" {
		t.Errorf("group VI = %+v, want mn10 from the heading then mn119", vi.Suttas)
	}
	if x := tab.Groups[9]; len(x.Suttas) != 1 || x.Suttas[0].Muid != "mn141" {
		t.Errorf("group X = %+v, want only mn141", x.Suttas)
	}
	if xv := tab.Groups[14]; len(xv.Suttas) != 1 || xv.Suttas[0].Muid != "mn148" {
		t.Errorf("group XV = %+v, want only mn148", xv.Suttas)
	}
}

func TestParseRealRangeExpansion(t *testing.T) {
	tab := mustParseReal(t)
	byMuid := make(map[string]Sutta)
	for _, s := range tab.Groups[0].Suttas {
		byMuid[s.Muid] = s
	}
	rangeLink := "https://suttacentral.net/an2.130-140/en/sujato?lang=en&layout=plain&reference=none&notes=none&highlight=true&script=latin"
	for n := 130; n <= 140; n++ {
		muid := "an2." + itoa(n)
		s, ok := byMuid[muid]
		if !ok {
			t.Errorf("group I missing expanded %s", muid)
			continue
		}
		if s.Display != "AN 2."+itoa(n) {
			t.Errorf("%s display = %q, want derived AN 2.%d", muid, s.Display, n)
		}
		if s.Link != rangeLink {
			t.Errorf("%s link = %q, want the range url", muid, s.Link)
		}
	}
	xii := make(map[string]Sutta)
	for _, s := range tab.Groups[11].Suttas {
		xii[s.Muid] = s
	}
	for _, muid := range []string{"dhp209", "dhp210", "dhp220", "an1.1", "an1.10", "an1.394", "an1.480", "an1.574"} {
		if _, ok := xii[muid]; !ok {
			t.Errorf("group XII missing %s", muid)
		}
	}
	if got := xii["dhp210"].Display; got != "KN Dhp 210" {
		t.Errorf("dhp210 display = %q, want KN Dhp 210", got)
	}
	if got := xii["an1.394"].Display; got != "AN 1.394" {
		t.Errorf("an1.394 display = %q, want AN 1.394", got)
	}
}

func TestParseRealDedupFirstWins(t *testing.T) {
	tab := mustParseReal(t)
	owner := groupSuttaMap(tab)
	pairs := map[string]int{
		"mn22":    1,
		"mn39":    1,
		"an10.60": 1,
		"sn12.63": 0,
		"an7.50":  0,
		"an4.159": 0,
		"an5.28":  8,
		"an9.36":  8,
		"an2.9":   1,
		"an3.107": 2,
		"mn145":   2,
	}
	for muid, wantGroup := range pairs {
		if got, ok := owner[muid]; !ok {
			t.Errorf("muid %s missing entirely", muid)
		} else if got != wantGroup {
			t.Errorf("muid %s owned by group %d, want %d", muid, got+1, wantGroup+1)
		}
	}
	for _, absent := range []string{"mn152", "an3.108"} {
		if _, ok := owner[absent]; ok {
			t.Errorf("muid %s unexpectedly present, its link pointed at an earlier muid", absent)
		}
	}
	xi := groupSuttaMap(Table{Groups: tab.Groups[10:11]})
	for _, muid := range []string{"an2.1", "an2.8", "an2.10"} {
		if _, ok := xi[muid]; !ok {
			t.Errorf("group XI missing range member %s", muid)
		}
	}
	if _, ok := xi["an2.9"]; ok {
		t.Errorf("group XI holds an2.9, but group II linked it first")
	}
}

func TestParseRealPreambleAndCollectionLinksIgnored(t *testing.T) {
	tab := mustParseReal(t)
	owner := groupSuttaMap(tab)
	for _, absent := range []string{"mn107", "dhp", "ja", "editions", "discourses-guide-sujato", "pli-tv-vi", "pli-tv-bu-vb"} {
		if _, ok := owner[absent]; ok {
			t.Errorf("collection or preamble link became muid %s", absent)
		}
	}
	for _, want := range []string{"pli-tv-bu-vb-pj3", "pli-tv-bu-vb-pj1", "vv51"} {
		if _, ok := owner[want]; !ok {
			t.Errorf("muid %s missing", want)
		}
	}
}

func TestParseRealOddEntries(t *testing.T) {
	tab := mustParseReal(t)
	owner := groupSuttaMap(tab)
	all := make(map[string]Sutta)
	for _, g := range tab.Groups {
		for _, s := range g.Suttas {
			if _, ok := all[s.Muid]; !ok {
				all[s.Muid] = s
			}
		}
	}
	if got := all["vv51"].Display; got != "Vv 51" {
		t.Errorf("vv51 display = %q, want the author text Vv 51", got)
	}
	if got := all["pli-tv-bu-vb-pj3"].Display; got != "murder" {
		t.Errorf("pli-tv-bu-vb-pj3 display = %q, want the author text murder", got)
	}
	if got := all["pli-tv-bu-vb-pj1"].Display; got != "Vinaya" {
		t.Errorf("pli-tv-bu-vb-pj1 display = %q, want the author text Vinaya", got)
	}
	if got := all["an3.64"].Display; got != "AN 3.36" {
		t.Errorf("an3.64 display = %q, want the author text AN 3.36", got)
	}
	if got := all["an5.194"].Display; got != "AN 5.191" {
		t.Errorf("an5.194 display = %q, want the author text AN 5.191", got)
	}
	if got := all["an4.55"].Display; got != "AN 4.45" {
		t.Errorf("an4.55 display = %q, want the author text AN 4.45", got)
	}
	if got := all["an3.129"].Display; got != "AN 3.139" {
		t.Errorf("an3.129 display = %q, want the author text AN 3.139", got)
	}
	if _, ok := owner["an4.45"]; !ok || owner["an4.45"] != 0 {
		t.Errorf("an4.45 should be a real group I muid from its own link")
	}
	if _, ok := owner["an5.191"]; !ok || owner["an5.191"] != 1 {
		t.Errorf("an5.191 should be a real group II muid from its own link")
	}
	wantLink := "https://suttacentral.net/dn16/en/sujato?lang=en&layout=plain&reference=none&notes=none&highlight=false&script=latin#5.9.1"
	if got := all["dn16"].Link; got != wantLink {
		t.Errorf("dn16 link = %q, want the anchored url kept verbatim", got)
	}
}

func TestParseRealDeterministic(t *testing.T) {
	data, err := os.ReadFile(realDumpPath)
	if err != nil {
		t.Fatalf("read artifact: %v", err)
	}
	a, err := ParseFile("x.md", string(data))
	if err != nil {
		t.Fatalf("first parse: %v", err)
	}
	b, err := ParseFile("x.md", string(data))
	if err != nil {
		t.Fatalf("second parse: %v", err)
	}
	if len(a.Groups) != len(b.Groups) {
		t.Fatalf("group count differs between parses")
	}
	for i := range a.Groups {
		if a.Groups[i].Name != b.Groups[i].Name || len(a.Groups[i].Suttas) != len(b.Groups[i].Suttas) {
			t.Fatalf("group %d differs between parses", i+1)
		}
		for j := range a.Groups[i].Suttas {
			if a.Groups[i].Suttas[j] != b.Groups[i].Suttas[j] {
				t.Fatalf("group %d sutta %d differs between parses", i+1, j+1)
			}
		}
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}
