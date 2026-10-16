package quote

import (
	"strings"
	"testing"
)

func FuzzParseCanonical(f *testing.F) {
	for _, s := range []string{
		"*sabbe sattā sukhi attānaṁ dhārentu* - **the Buddha, SN 1.4**",
		"*alpha one*\n*alpha two* - **the Buddha, MN 22**",
		"*single* - **the bhikkhunis, SN 5.2**",
		RenderExportFile([]*Quote{
			New("MN 22", "the Buddha, MN 22", []string{`"first passage"`, "second passage"}),
			New("AN 5.34", "the Buddha, AN 5.34", []string{"beta single"}),
			New("KN Snp 2.14", "the Buddha, KN Snp 2.14", []string{"gamma one", "gamma two"}),
		}),
		"*a passage* - the Buddha, MN 9",
		"*an unknown speaker block* - **an unknown speaker**",
		"\ufeff*BOM passage* - **the Buddha, MN 1**",
		"*sabbe satta\u0304 NFD passage* - **the Buddha, AN 3.38**",
		"**nested bold** - **MN 1**",
		"*citation with stars* - **a*b**",
		"*broken citation",
		"*passage only*\n\n\n.  \n.  \n.\n\n\n*real* - **the Buddha, MN 1**",
		"*real* - **the Buddha, MN 1**\n\n\n.  \n.  \n.\n\n\n - **the Buddha, MN 2**",
		"*p* - ****",
		"***\n*after stars* - **the Buddha, MN 4**",
		"*a - **x** b* - **MN 3**",
		"*one* - **the Buddha, MN 1**\n\n\n.  \n.  \n.\n\n\n**still starred**\n*second* - **the Buddha, SN 2.1**",
		"*p*\r\n*q* - **the Buddha, DN 2**\r",
		".\n.\n.",
		"",
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, text string) {
		first := ParseCanonical(text)
		for _, q := range first {
			if len(q.Passages) == 0 {
				t.Fatalf("quote with no passages from %q", text)
			}
			for _, p := range q.Passages {
				if strings.TrimSpace(p) == "" {
					t.Fatalf("blank passage %q from %q", p, text)
				}
			}
			if strings.TrimSpace(q.Citation) == "" {
				t.Fatalf("blank citation from %q", text)
			}
			id := CanonicalSuttaID(q.Citation)
			if id != CanonicalSuttaID(id) {
				t.Fatalf("CanonicalSuttaID not idempotent: %q -> %q -> %q", q.Citation, id, CanonicalSuttaID(id))
			}
		}
		second := ParseCanonical(RenderExportFile(first))
		if len(first) != len(second) {
			t.Fatalf("re-export changed quote count: %d -> %d for %q", len(first), len(second), text)
		}
		for i := range first {
			if !quotesEqual(first[i], second[i]) {
				t.Fatalf("round-trip mismatch on quote %d:\n got  %+v\n want %+v\n input %q", i, second[i], first[i], text)
			}
		}
	})
}
