package triangulate

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lavantien/quotes-manager/internal/canon/model"
	"github.com/lavantien/quotes-manager/internal/canon/norm"
	"github.com/lavantien/quotes-manager/internal/config"
)

func writeDpdForms(t *testing.T, dir string, forms []model.DpdForm) string {
	t.Helper()
	var b strings.Builder
	for _, f := range forms {
		raw, err := json.Marshal(f)
		if err != nil {
			t.Fatal(err)
		}
		b.Write(raw)
		b.WriteByte('\n')
	}
	p := filepath.Join(dir, "dpd-forms.ndjson")
	if err := os.WriteFile(p, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func writeRootStream(t *testing.T, dir string, recs []model.SegmentRecord) {
	t.Helper()
	var b strings.Builder
	for _, r := range recs {
		raw, err := json.Marshal(r)
		if err != nil {
			t.Fatal(err)
		}
		b.Write(raw)
		b.WriteByte('\n')
	}
	if err := os.WriteFile(filepath.Join(dir, "root.ndjson"), []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
}

func rootRec(muid, segkey, text string) model.SegmentRecord {
	return model.SegmentRecord{Stream: model.StreamRoot, Muid: muid, SegKey: segkey, Text: text}
}

func dpdEnv(t *testing.T, forms []model.DpdForm, roots ...model.SegmentRecord) Env {
	t.Helper()
	parsed := t.TempDir()
	writeRootStream(t, parsed, roots)
	return Env{
		ParsedDir:    parsed,
		DpdDBPath:    filepath.Join(t.TempDir(), "dpd.db"),
		DpdFormsFile: writeDpdForms(t, t.TempDir(), forms),
	}
}

func TestD001AllTokensKnown(t *testing.T) {
	env := dpdEnv(t,
		[]model.DpdForm{{Form: "evam"}, {Form: "me"}, {Form: "sutam"}},
		rootRec("mn1", "1.1", "evaṁ me sutaṁ"))
	ds, err := runD001(context.Background(), config.Defaults(), newTestStore(t), env)
	if err != nil {
		t.Fatalf("d001: %v", err)
	}
	if len(ds) != 0 {
		t.Fatalf("fully known muid must stay silent, got %v", ds)
	}
}

func TestD001UnknownBelowThresholdIsInfoWithSample(t *testing.T) {
	env := dpdEnv(t,
		[]model.DpdForm{{Form: "evam"}},
		rootRec("mn1", "1.1", "evam goblidook yabbadabbadoo"))
	ds, err := runD001(context.Background(), config.Defaults(), newTestStore(t), env)
	if err != nil {
		t.Fatalf("d001: %v", err)
	}
	if len(ds) != 1 {
		t.Fatalf("findings = %d, want 1: %v", len(ds), ds)
	}
	d := ds[0]
	if d.Severity != model.Info {
		t.Fatalf("below threshold severity = %s, want info (default base)", d.Severity)
	}
	if d.Muid != "mn1" {
		t.Fatalf("muid = %q, want mn1", d.Muid)
	}
	if !strings.Contains(d.Detail, "goblidook") || !strings.Contains(d.Detail, "yabbadabbadoo") {
		t.Fatalf("detail must sample the unknown forms: %q", d.Detail)
	}
	if !strings.Contains(d.Detail, "2") || !strings.Contains(d.Detail, "20") {
		t.Fatalf("detail must carry count and threshold: %q", d.Detail)
	}
}

func TestD001UnknownAtThresholdEscalatesToWarn(t *testing.T) {
	cfg := config.Defaults()
	cfg.Canon.Dpd.UnknownWarnPerMuid = 2
	env := dpdEnv(t,
		[]model.DpdForm{{Form: "evam"}},
		rootRec("mn1", "1.1", "evam goblidook"),
		rootRec("mn1", "1.2", "goblidook again yabbadabbadoo"))
	ds, err := runD001(context.Background(), cfg, newTestStore(t), env)
	if err != nil {
		t.Fatalf("d001: %v", err)
	}
	if len(ds) != 1 || ds[0].Severity != model.Warn {
		t.Fatalf("at-threshold must escalate to warn, got %v", ds)
	}
}

func TestD001CountsDistinctPerMuid(t *testing.T) {
	env := dpdEnv(t,
		[]model.DpdForm{{Form: "evam"}},
		rootRec("mn1", "1.1", "goblidook goblidook"),
		rootRec("mn1", "1.2", "goblidook"),
		rootRec("mn2", "1.1", "goblidook"))
	ds, err := runD001(context.Background(), config.Defaults(), newTestStore(t), env)
	if err != nil {
		t.Fatalf("d001: %v", err)
	}
	if len(ds) != 2 {
		t.Fatalf("one finding per muid with unknowns, got %v", ds)
	}
	for _, d := range ds {
		if !strings.HasPrefix(d.Detail, "unknown forms: 1 ") {
			t.Fatalf("distinct forms must collapse to 1, got %q", d.Detail)
		}
	}
}

func TestD001DeconstructorSplitResolution(t *testing.T) {
	env := dpdEnv(t,
		[]model.DpdForm{
			{Form: "dhamma"},
			{Form: "me"},
			{Form: "dhammaca", Splits: []string{"dhamma", "me"}},
			{Form: "sandhize", Splits: []string{"dhamma", "nonsenseword"}},
		},
		rootRec("mn1", "1.1", "dhammacā sandhize"))
	ds, err := runD001(context.Background(), config.Defaults(), newTestStore(t), env)
	if err != nil {
		t.Fatalf("d001: %v", err)
	}
	if len(ds) != 1 || !strings.Contains(ds[0].Detail, "sandhize") {
		t.Fatalf("unresolved split must be the only unknown, got %v", ds)
	}
	if strings.Contains(ds[0].Detail, "dhammaca") {
		t.Fatalf("resolved split must not appear, got %q", ds[0].Detail)
	}
}

func TestD001RawPlusSeparatedSplits(t *testing.T) {
	env := dpdEnv(t,
		[]model.DpdForm{
			{Form: "dhamma"},
			{Form: "me"},
			{Form: "dhammame", Splits: []string{"dhamma + me"}},
		},
		rootRec("mn1", "1.1", "dhammame"))
	ds, err := runD001(context.Background(), config.Defaults(), newTestStore(t), env)
	if err != nil {
		t.Fatalf("d001: %v", err)
	}
	if len(ds) != 0 {
		t.Fatalf("plus-joined split must resolve, got %v", ds)
	}
}

func TestD001FormsFileAbsentDegrades(t *testing.T) {
	for _, path := range []string{"", filepath.Join(t.TempDir(), "missing.ndjson")} {
		env := Env{ParsedDir: t.TempDir(), DpdFormsFile: path}
		ds, err := runD001(context.Background(), config.Defaults(), newTestStore(t), env)
		if err != nil {
			t.Fatalf("forms file %q absent must degrade, got %v", path, err)
		}
		if len(ds) != 0 {
			t.Fatalf("D002 owns the degraded note, D001 must stay silent: %v", ds)
		}
	}
}

func TestD001EmptyParsedDirSilent(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "dpd-forms.ndjson"), []byte("{\"form\":\"evam\"}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	env := Env{DpdFormsFile: filepath.Join(dir, "dpd-forms.ndjson")}
	ds, err := runD001(context.Background(), config.Defaults(), newTestStore(t), env)
	if err != nil || len(ds) != 0 {
		t.Fatalf("empty parsed dir must be silent, got ds=%v err=%v", ds, err)
	}
}

func TestD001PromisedRootStreamMissingErrors(t *testing.T) {
	env := dpdEnv(t, []model.DpdForm{{Form: "evam"}})
	if err := os.Remove(filepath.Join(env.ParsedDir, "root.ndjson")); err != nil {
		t.Fatal(err)
	}
	if _, err := runD001(context.Background(), config.Defaults(), newTestStore(t), env); err == nil {
		t.Fatal("promised-but-missing root stream must error, not degrade")
	}
}

func TestD001BadFormsLineErrors(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "dpd-forms.ndjson")
	if err := os.WriteFile(p, []byte("{not json\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	env := Env{ParsedDir: t.TempDir(), DpdFormsFile: p}
	if _, err := runD001(context.Background(), config.Defaults(), newTestStore(t), env); err == nil {
		t.Fatal("corrupt forms file must error")
	}
}

func TestD001EscalationHonorsDirectionOverrideAndNeverLowers(t *testing.T) {
	cfg := config.Defaults()
	cfg.Canon.Dpd.UnknownWarnPerMuid = 1
	cfg.Canon.Triangulate.Severity["D001.escalated"] = string(model.Error)
	env := dpdEnv(t, []model.DpdForm{}, rootRec("mn1", "1.1", "goblidook"))
	ds, err := runD001(context.Background(), cfg, newTestStore(t), env)
	if err != nil {
		t.Fatal(err)
	}
	if len(ds) != 1 || ds[0].Severity != model.Error {
		t.Fatalf("explicit escalated override must win, got %v", ds)
	}

	cfg2 := config.Defaults()
	cfg2.Canon.Dpd.UnknownWarnPerMuid = 1
	cfg2.Canon.Triangulate.Severity["D001"] = string(model.Error)
	ds2, err := runD001(context.Background(), cfg2, newTestStore(t), env)
	if err != nil {
		t.Fatal(err)
	}
	if len(ds2) != 1 || ds2[0].Severity != model.Error {
		t.Fatalf("base error must keep escalation at error, got %v", ds2)
	}
}

func TestD001SampleIsCapped(t *testing.T) {
	var text strings.Builder
	for i := range 30 {
		if i > 0 {
			text.WriteByte(' ')
		}
		fmt.Fprintf(&text, "unknownformx%c", rune('a'+i))
	}
	env := dpdEnv(t, []model.DpdForm{{Form: "evam"}},
		rootRec("mn1", "1.1", text.String()))
	ds, err := runD001(context.Background(), config.Defaults(), newTestStore(t), env)
	if err != nil {
		t.Fatalf("d001: %v", err)
	}
	if len(ds) != 1 {
		t.Fatalf("one finding, got %v", ds)
	}
	if !strings.Contains(ds[0].Detail, "(+") {
		t.Fatalf("capped sample must carry a (+N more) tail: %q", ds[0].Detail)
	}
}

func TestD002PresentIsSilent(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "dpd.db")
	if err := os.WriteFile(dbPath, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	forms := writeDpdForms(t, dir, []model.DpdForm{{Form: "evam"}})
	env := Env{DpdDBPath: dbPath, DpdFormsFile: forms}
	ds, err := runD002(context.Background(), config.Defaults(), newTestStore(t), env)
	if err != nil {
		t.Fatalf("d002: %v", err)
	}
	if len(ds) != 0 {
		t.Fatalf("fully present dpd inputs must stay silent, got %v", ds)
	}
}

func TestD002DegradedVariants(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "dpd.db")
	if err := os.WriteFile(dbPath, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	forms := writeDpdForms(t, dir, []model.DpdForm{{Form: "evam"}})
	emptyForms := filepath.Join(dir, "empty.ndjson")
	if err := os.WriteFile(emptyForms, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		env  Env
	}{
		{"db missing", Env{DpdFormsFile: forms}},
		{"forms missing", Env{DpdDBPath: dbPath}},
		{"forms empty", Env{DpdDBPath: dbPath, DpdFormsFile: emptyForms}},
		{"zero env", Env{}},
	}
	for _, tc := range cases {
		ds, err := runD002(context.Background(), config.Defaults(), newTestStore(t), tc.env)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if len(ds) != 1 {
			t.Fatalf("%s: want exactly one note, got %v", tc.name, ds)
		}
		d := ds[0]
		if d.Check != model.CheckDpdDegraded || d.Severity != model.Info {
			t.Fatalf("%s: bad finding %+v", tc.name, d)
		}
		if d.Detail != "dpd database absent; dpd checks degraded (D001 skipped)" {
			t.Fatalf("%s: detail = %q", tc.name, d.Detail)
		}
	}
}

func TestD002StatErrorPropagates(t *testing.T) {
	dir := t.TempDir()
	env := Env{DpdDBPath: dir, DpdFormsFile: writeDpdForms(t, dir, nil)}
	if _, err := runD002(context.Background(), config.Defaults(), newTestStore(t), env); err == nil {
		t.Fatal("stat failure must error, not degrade")
	}
}

func TestD001PlainRecordWinsOverBrokenSplit(t *testing.T) {
	env := dpdEnv(t,
		[]model.DpdForm{
			{Form: "dhammaca"},
			{Form: "dhammaca", Splits: []string{"dhamma", "nonsenseword"}},
			{Form: "dhamma"},
		},
		rootRec("mn1", "1.1", "dhammaca dhamma"))
	ds, err := runD001(context.Background(), config.Defaults(), newTestStore(t), env)
	if err != nil {
		t.Fatal(err)
	}
	if len(ds) != 0 {
		t.Fatalf("plain record must keep the form known, got %v", ds)
	}
}

func TestD001HeadwordBackedRecordWithSplitsIsKnown(t *testing.T) {
	env := dpdEnv(t,
		[]model.DpdForm{
			{Form: "dhamma"},
			{Form: "dhammaca", Headwords: []string{"dhammaca"}, Splits: []string{"dhamma + nagglewort"}},
		},
		rootRec("mn1", "1.1", "dhammacā dhamma"))
	ds, err := runD001(context.Background(), config.Defaults(), newTestStore(t), env)
	if err != nil {
		t.Fatal(err)
	}
	if len(ds) != 0 {
		t.Fatalf("headword-backed form must stay known despite the broken split, got %v", ds)
	}
}

func TestD001SplitPartsResolveAgainstCompleteRawSet(t *testing.T) {
	env := dpdEnv(t,
		[]model.DpdForm{
			{Form: "b"},
			{Form: "aab", Splits: []string{"b + zzz"}},
			{Form: "zzz", Splits: []string{"b + b"}},
		},
		rootRec("mn1", "1.1", "aab zzz b"))
	ds, err := runD001(context.Background(), config.Defaults(), newTestStore(t), env)
	if err != nil {
		t.Fatal(err)
	}
	if len(ds) != 0 {
		t.Fatalf("raw-key split resolution must be order independent, got %v", ds)
	}
}

func TestD001ThresholdZeroWarnsOnAnyUnknown(t *testing.T) {
	cfg := config.Defaults()
	cfg.Canon.Dpd.UnknownWarnPerMuid = 0
	env := dpdEnv(t,
		[]model.DpdForm{{Form: "evam"}},
		rootRec("mn1", "1.1", "evam goblidook"),
		rootRec("mn2", "1.1", "evam"))
	ds, err := runD001(context.Background(), cfg, newTestStore(t), env)
	if err != nil {
		t.Fatal(err)
	}
	if len(ds) != 1 || ds[0].Muid != "mn1" || ds[0].Severity != model.Warn {
		t.Fatalf("threshold 0 must warn once on the only unknown muid, got %v", ds)
	}
}

func TestD001FoldedLookupOfDiacriticTokens(t *testing.T) {
	env := dpdEnv(t,
		[]model.DpdForm{{Form: "sunati"}, {Form: "dhamma"}},
		rootRec("mn1", "1.1", "suṇāti dhammo"))
	ds, err := runD001(context.Background(), config.Defaults(), newTestStore(t), env)
	if err != nil {
		t.Fatal(err)
	}
	if len(ds) != 1 || !strings.Contains(ds[0].Detail, "dhammo") || strings.Contains(ds[0].Detail, "sunati") {
		t.Fatalf("fold must resolve suṇāti and leave dhammo, got %v", ds)
	}
}

func FuzzDpdKnownForms(f *testing.F) {
	f.Add(`{"form":"evam"}` + "\n" + `{"form":"dhammaca","splits":["dhamma","ca"]}` + "\n")
	f.Add(`{"form":"dhammame","splits":["dhamma + me"]}` + "\n")
	f.Add(`{"form":"suṇāti"}` + "\n")
	f.Add("")
	f.Add("{\"form\":\"DhAMMA\"}\n")
	f.Add(`{"form":"a","splits":[""]}` + "\n")
	f.Fuzz(func(t *testing.T, content string) {
		dir := t.TempDir()
		p := filepath.Join(dir, "dpd-forms.ndjson")
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		known, err := loadDpdKnownForms(p)
		if err != nil || known == nil {
			return
		}
		for k := range known {
			if norm.FoldPali(k) != k {
				t.Fatalf("known key %q is not folded", k)
			}
		}
		known2, err2 := loadDpdKnownForms(p)
		if err2 != nil {
			t.Fatalf("second load errored: %v", err2)
		}
		if len(known2) != len(known) {
			t.Fatalf("nondeterministic known set sizes %d vs %d", len(known2), len(known))
		}
		for k := range known2 {
			if !known[k] {
				t.Fatalf("key %q only in second load", k)
			}
		}
	})
}
