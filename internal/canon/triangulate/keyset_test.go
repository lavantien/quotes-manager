package triangulate

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lavantien/quotes-manager/internal/canon/model"
	"github.com/lavantien/quotes-manager/internal/config"
)

func TestB001UntranslatedPerAuthor(t *testing.T) {
	st := newTestStore(t)
	st.addRoot(t, "mn1", "0.1", "Evaṁ me sutaṁ—")
	st.addRoot(t, "mn1", "0.2", "second root segment")
	st.addEn(t, "sujato", "mn1", "0.1", "So I have heard. ")
	st.addRoot(t, "pli-tv-bu-pm", "1.1", "vinaya root")
	st.addEn(t, "brahmali", "pli-tv-bu-pm", "1.1", "vinaya translation")

	ds, err := runB001(context.Background(), config.Defaults(), st, Env{})
	if err != nil {
		t.Fatalf("b001: %v", err)
	}
	got := findDisc(ds, model.CheckKeysetRootTranslation, model.Warn, "untranslated")
	if len(got) != 1 {
		t.Fatalf("untranslated discrepancies = %d (%v), want 1", len(got), ds)
	}
	d := got[0]
	if d.Muid != "mn1" || !strings.Contains(d.Detail, "untranslated by sujato: mn1:0.2") {
		t.Fatalf("bad discrepancy: %+v", d)
	}
	if d.NodeID == 0 {
		t.Fatal("untranslated discrepancy should carry the root node id")
	}
}

func TestB001EveryAuthorOfMuidChecked(t *testing.T) {
	st := newTestStore(t)
	st.addRoot(t, "mn1", "0.1", "root")
	st.addEn(t, "sujato", "mn1", "0.1", "s")
	st.addEn(t, "brahmali", "mn1", "0.2", "b")
	st.addRoot(t, "mn1", "0.3", "root 3")

	ds, err := runB001(context.Background(), config.Defaults(), st, Env{})
	if err != nil {
		t.Fatalf("b001: %v", err)
	}
	got := findDisc(ds, model.CheckKeysetRootTranslation, model.Warn, "untranslated")
	if len(got) != 3 {
		t.Fatalf("untranslated discrepancies = %d (%v), want 3", len(got), ds)
	}
	want := []string{
		"untranslated by brahmali: mn1:0.1",
		"untranslated by sujato: mn1:0.3",
		"untranslated by brahmali: mn1:0.3",
	}
	for _, w := range want {
		found := false
		for _, d := range got {
			if strings.Contains(d.Detail, w) {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("missing expected flag %q in %v", w, ds)
		}
	}
	for _, d := range got {
		if strings.Contains(d.Detail, "by sujato: mn1:0.1") {
			t.Fatalf("sujato translated 0.1, must not flag: %+v", d)
		}
	}
}

func TestB001OrphanSeverityResolution(t *testing.T) {
	st := newTestStore(t)
	st.addEn(t, "sujato", "mn1", "9.9", "orphan body")

	cfg := config.Defaults()
	ds, err := runB001(context.Background(), cfg, st, Env{})
	if err != nil {
		t.Fatalf("b001: %v", err)
	}
	got := findDisc(ds, model.CheckKeysetRootTranslation, model.Error, "orphan")
	if len(got) != 1 {
		t.Fatalf("orphan default must be error, got %d orphans (%v)", len(got), ds)
	}
	if got[0].Muid != "mn1" || !strings.Contains(got[0].Detail, "en/sujato:mn1:9.9") {
		t.Fatalf("bad orphan discrepancy: %+v", got[0])
	}

	cfg.Canon.Triangulate.Severity["B001.orphan"] = string(model.Warn)
	ds, err = runB001(context.Background(), cfg, st, Env{})
	if err != nil {
		t.Fatalf("b001 override: %v", err)
	}
	if got := findDisc(ds, model.CheckKeysetRootTranslation, model.Warn, "orphan"); len(got) != 1 {
		t.Fatalf("explicit orphan override must be respected, got %v", ds)
	}
}

func TestB002BothDirections(t *testing.T) {
	st := newTestStore(t)
	st.addRoot(t, "mn1", "0.1", "root with html")
	st.addCognate(t, "html:", "mn1", "0.1")
	st.addCognate(t, "html:", "mn1", "9.9")
	st.addRoot(t, "mn1", "0.3", "root without html")

	ds, err := runB002(context.Background(), config.Defaults(), st, Env{})
	if err != nil {
		t.Fatalf("b002: %v", err)
	}
	if got := findDisc(ds, model.CheckKeysetRootHTML, model.Warn, "html:mn1:9.9"); len(got) != 1 {
		t.Fatalf("extra html = %d (%v), want 1", len(got), ds)
	}
	if got := findDisc(ds, model.CheckKeysetRootHTML, model.Info, "pli:mn1:0.3"); len(got) != 1 {
		t.Fatalf("missing html = %d (%v), want 1", len(got), ds)
	}
	if got := findDisc(ds, model.CheckKeysetRootHTML, model.Warn, ""); len(got) != 1 {
		t.Fatalf("total extra html = %d (%v), want exactly 1", len(got), ds)
	}
	if got := findDisc(ds, model.CheckKeysetRootHTML, model.Info, ""); len(got) != 1 {
		t.Fatalf("total missing html = %d (%v), want exactly 1", len(got), ds)
	}
}

func TestB003BothDirections(t *testing.T) {
	st := newTestStore(t)
	st.addRoot(t, "mn1", "0.1", "root with ref")
	st.addCognate(t, "reference:", "mn1", "0.1")
	st.addCognate(t, "reference:", "mn1", "9.9")
	st.addRoot(t, "mn1", "0.3", "root without ref")

	ds, err := runB003(context.Background(), config.Defaults(), st, Env{})
	if err != nil {
		t.Fatalf("b003: %v", err)
	}
	if got := findDisc(ds, model.CheckKeysetReference, model.Error, "reference:mn1:9.9"); len(got) != 1 {
		t.Fatalf("extra ref = %d (%v), want 1", len(got), ds)
	}
	if got := findDisc(ds, model.CheckKeysetReference, model.Info, "pli:mn1:0.3"); len(got) != 1 {
		t.Fatalf("missing ref = %d (%v), want 1", len(got), ds)
	}
	if got := findDisc(ds, model.CheckKeysetReference, model.Error, ""); len(got) != 1 {
		t.Fatalf("total extra ref = %d (%v), want exactly 1", len(got), ds)
	}
	if got := findDisc(ds, model.CheckKeysetReference, model.Info, ""); len(got) != 1 {
		t.Fatalf("total missing ref = %d (%v), want exactly 1", len(got), ds)
	}
}

func TestB001TwoAuthorsShareRootSilently(t *testing.T) {
	st := newTestStore(t)
	st.addRoot(t, "mn1", "0.1", "shared root")
	st.addEn(t, "sujato", "mn1", "0.1", "s")
	st.addEn(t, "brahmali", "mn1", "0.1", "b")

	ds, err := runB001(context.Background(), config.Defaults(), st, Env{})
	if err != nil {
		t.Fatalf("b001: %v", err)
	}
	if got := findDisc(ds, model.CheckKeysetRootTranslation, model.Warn, "untranslated"); len(got) != 0 {
		t.Fatalf("a root translated by every author of its muid must not flag: %v", ds)
	}
}

func TestB001SkipsMalformedKeys(t *testing.T) {
	st := newTestStore(t)
	st.addRoot(t, "mn1", "0.1", "root")
	st.addEn(t, "sujato", "mn1", "0.1", "translation")
	st.addRoot(t, "mn1", "", "root without segkey")
	st.addEn(t, "", "mn1", "0.2", "authorless")

	ds, err := runB001(context.Background(), config.Defaults(), st, Env{})
	if err != nil {
		t.Fatalf("b001: %v", err)
	}
	if got := findDisc(ds, model.CheckKeysetRootTranslation, model.Warn, "untranslated"); len(got) != 0 {
		t.Fatalf("malformed keys must be skipped, not flagged: %v", ds)
	}
}

func TestB007PrefixMismatches(t *testing.T) {
	dir := t.TempDir()
	writeMeta(t, dir, map[string]parsedFile{
		"root/pli/ms/sutta/mn/mn1_root-pli-ms.json": {
			PrefixMismatches: []string{"mn2:9.9", "introduction"},
			Muid:             "mn1",
			Stream:           string(model.StreamRoot),
		},
		"root/pli/ms/sutta/mn/mn2_root-pli-ms.json": {Muid: "mn2", Stream: "root"},
	})

	ds, err := runB007(context.Background(), config.Defaults(), newTestStore(t), Env{ParsedDir: dir})
	if err != nil {
		t.Fatalf("b007: %v", err)
	}
	if len(ds) != 1 {
		t.Fatalf("b007 discrepancies = %d, want 1 (%v)", len(ds), ds)
	}
	d := ds[0]
	if d.Check != model.CheckPrefixMismatch || d.Severity != model.Warn || d.Muid != "mn1" {
		t.Fatalf("bad b007 discrepancy: %+v", d)
	}
	for _, want := range []string{"mn1_root-pli-ms.json", "mn2:9.9", "introduction"} {
		if !strings.Contains(d.Detail, want) {
			t.Fatalf("detail must carry %q: %+v", want, d)
		}
	}
}

func TestB004DuplicateKeysCapped(t *testing.T) {
	dir := t.TempDir()
	writeMeta(t, dir, map[string]parsedFile{
		"root/pli/ms/sutta/mn/mn1_root-pli-ms.json": {
			DupKeys: []string{"mn1:1.1", "mn1:2.2", "mn1:3.3"},
			Muid:    "mn1",
			Stream:  string(model.StreamRoot),
		},
		"root/pli/ms/sutta/mn/mn2_root-pli-ms.json": {Muid: "mn2", Stream: "root"},
	})
	cfg := config.Defaults()
	cfg.Canon.Triangulate.DetailSampleMax = 10

	ds, err := runB004(context.Background(), cfg, newTestStore(t), Env{ParsedDir: dir})
	if err != nil {
		t.Fatalf("b004: %v", err)
	}
	if len(ds) != 1 {
		t.Fatalf("b004 discrepancies = %d, want 1 (%v)", len(ds), ds)
	}
	d := ds[0]
	if d.Check != model.CheckDuplicateKeys || d.Severity != model.Error || d.Muid != "mn1" {
		t.Fatalf("bad b004 discrepancy: %+v", d)
	}
	if !strings.Contains(d.Detail, "root/pli/ms/sutta/mn/mn1_root-pli-ms.json") {
		t.Fatalf("detail must name the file: %+v", d)
	}
	if strings.Contains(d.Detail, "mn1:2.2") || !strings.Contains(d.Detail, "(+2 more)") {
		t.Fatalf("dup key sample must be capped: %+v", d)
	}
}

func TestB005EmptyValueThresholds(t *testing.T) {
	dir := t.TempDir()
	writeMeta(t, dir, map[string]parsedFile{
		"root/pli/ms/sutta/mn/mn1_root-pli-ms.json":                     {EmptyValues: 5, Muid: "mn1", Stream: "root"},
		"translation/en/sujato/sutta/mn/mn1_translation-en-sujato.json": {EmptyValues: 6, Muid: "mn1", Stream: "translation"},
		"root/pli/ms/sutta/mn/mn2_root-pli-ms.json":                     {EmptyValues: 20, Muid: "mn2", Stream: "root"},
		"html/pli/ms/sutta/mn/mn3_html.json":                            {EmptyValues: 100, Muid: "mn3", Stream: "html"},
	})

	ds, err := runB005(context.Background(), config.Defaults(), newTestStore(t), Env{ParsedDir: dir})
	if err != nil {
		t.Fatalf("b005: %v", err)
	}
	if got := findDisc(ds, model.CheckEmptyValue, model.Info, "mn1"); len(got) != 1 {
		t.Fatalf("mn1 below threshold must be info once, got %v", ds)
	}
	if got := findDisc(ds, model.CheckEmptyValue, model.Warn, "mn2"); len(got) != 1 {
		t.Fatalf("mn2 at threshold must be warn once, got %v", ds)
	}
	if got := findDisc(ds, model.CheckEmptyValue, model.Info, "mn3"); len(got) != 0 {
		t.Fatalf("html stream empties must be excluded, got %v", ds)
	}
	for _, d := range ds {
		if !strings.Contains(d.Detail, "threshold 20") {
			t.Fatalf("detail must carry the threshold: %+v", d)
		}
	}
}

func TestB005ZeroCountsSilent(t *testing.T) {
	dir := t.TempDir()
	writeMeta(t, dir, map[string]parsedFile{
		"root/pli/ms/sutta/mn/mn1_root-pli-ms.json":                     {Muid: "mn1", Stream: "root"},
		"translation/en/sujato/sutta/mn/mn1_translation-en-sujato.json": {EmptyValues: 1, Muid: "mn1", Stream: "translation"},
	})

	cfg := config.Defaults()
	cfg.Canon.Triangulate.EmptyWarnPerMuid = 0
	ds, err := runB005(context.Background(), cfg, newTestStore(t), Env{ParsedDir: dir})
	if err != nil {
		t.Fatalf("b005: %v", err)
	}
	if len(ds) != 1 || ds[0].Severity != model.Warn || ds[0].Muid != "mn1" {
		t.Fatalf("threshold 0 must warn only on the muid with empties: %v", ds)
	}
}

func TestB006NFCChanged(t *testing.T) {
	dir := t.TempDir()
	writeMeta(t, dir, map[string]parsedFile{
		"root/pli/ms/sutta/mn/mn1_root-pli-ms.json": {NFCChanged: true, Muid: "mn1", Stream: "root"},
	})

	ds, err := runB006(context.Background(), config.Defaults(), newTestStore(t), Env{ParsedDir: dir})
	if err != nil {
		t.Fatalf("b006: %v", err)
	}
	if len(ds) != 1 || ds[0].Severity != model.Info || !strings.Contains(ds[0].Detail, "mn1_root-pli-ms.json") {
		t.Fatalf("bad b006 discrepancies: %v", ds)
	}
}

func TestParsedMetaDegradation(t *testing.T) {
	ctx := context.Background()
	for _, run := range []func(context.Context, *config.Config, Store, Env) ([]model.Discrepancy, error){
		runB004, runB005, runB006,
	} {
		ds, err := run(ctx, config.Defaults(), newTestStore(t), Env{})
		if err != nil {
			t.Fatalf("zero env must degrade without error: %v", err)
		}
		if len(ds) != 0 {
			t.Fatalf("degraded run must yield zero discrepancies: %v", ds)
		}
	}
}

func TestParsedMetaMissingFileErrors(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "nested-marker"), 0o755); err != nil {
		t.Fatal(err)
	}
	env := Env{ParsedDir: filepath.Join(dir, "nested-marker", "missing-parent")}
	if _, err := runB004(context.Background(), config.Defaults(), newTestStore(t), env); err == nil {
		t.Fatal("promised parsed dir without meta.json must error")
	}
}
