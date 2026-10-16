package pipeline

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/lavantien/quotes-manager/internal/canon/model"
	"github.com/lavantien/quotes-manager/internal/canon/norm"
	"github.com/lavantien/quotes-manager/internal/canon/source/dpd"
	"github.com/lavantien/quotes-manager/internal/config"
)

type DpdStats struct {
	DpdHeadwords int  `json:"dpd_headwords"`
	DpdRoots     int  `json:"dpd_roots"`
	DpdForms     int  `json:"dpd_forms"`
	DpdAbsent    bool `json:"dpd_absent"`
}

func parseDpd(cfg *config.Config) (DpdStats, error) {
	var stats DpdStats
	if _, err := os.Stat(cfg.Canon.DpdDB()); err != nil {
		if os.IsNotExist(err) {
			return DpdStats{DpdAbsent: true}, nil
		}
		return DpdStats{}, fmt.Errorf("parse dpd: %w", err)
	}
	db, err := dpd.Open(cfg.Canon.DpdDB())
	if err != nil {
		return DpdStats{}, fmt.Errorf("parse dpd: %w", err)
	}
	defer func() { _ = db.Close() }()

	headwordsOut, err := newNDJSON(cfg.Canon.ParsedDir(), "dpd-headwords.ndjson")
	if err != nil {
		return DpdStats{}, fmt.Errorf("parse dpd: %w", err)
	}
	rootsOut, err := newNDJSON(cfg.Canon.ParsedDir(), "dpd-roots.ndjson")
	if err != nil {
		return DpdStats{}, fmt.Errorf("parse dpd: %w", err)
	}
	defer func() {
		_ = headwordsOut.close()
		_ = rootsOut.close()
	}()

	forms := map[string]map[string]bool{}
	splits := map[string][]string{}
	add := func(form, lemma string) {
		if form == "" || lemma == "" {
			return
		}
		set, ok := forms[form]
		if !ok {
			set = map[string]bool{}
			forms[form] = set
		}
		set[lemma] = true
	}

	err = dpd.StreamHeadwords(context.Background(), db, func(h dpd.Headword) error {
		stats.DpdHeadwords++
		if werr := headwordsOut.write(model.DpdHeadword{
			Lemma1: h.Lemma1, Lemma2: h.Lemma2, Pos: h.Pos, Grammar: h.Grammar,
			Meaning1: h.Meaning1, MeaningLit: h.MeaningLit, Sanskrit: h.Sanskrit,
			RootKey: h.RootKey, RootSign: h.RootSign, FamilyRoot: h.FamilyRoot,
			Construction: h.Construction, CompoundType: h.CompoundType, Stem: h.Stem,
		}); werr != nil {
			return werr
		}
		add(norm.FoldPali(strings.TrimSpace(h.Lemma1)), h.Lemma1)
		add(norm.FoldPali(strings.TrimSpace(h.Lemma2)), h.Lemma1)
		for _, f := range dpd.SplitInflections(h.Inflections) {
			add(norm.FoldPali(f), h.Lemma1)
		}
		return nil
	})
	if err != nil {
		return DpdStats{}, fmt.Errorf("parse dpd headwords: %w", err)
	}

	err = dpd.StreamRoots(context.Background(), db, func(r dpd.Root) error {
		stats.DpdRoots++
		return rootsOut.write(model.DpdRoot{
			Root: r.Root, RootInComps: r.RootInComps, RootHasVerb: r.RootHasVerb,
			RootGroup: r.RootGroup, RootSign: r.RootSign, RootMeaning: r.RootMeaning,
			SanskritRoot: r.SanskritRoot,
		})
	})
	if err != nil {
		return DpdStats{}, fmt.Errorf("parse dpd roots: %w", err)
	}

	err = dpd.StreamLookup(context.Background(), db, func(l dpd.LookupRow) error {
		key := norm.FoldPali(strings.TrimSpace(l.LookupKey))
		if key == "" {
			return nil
		}
		if _, ok := forms[key]; !ok {
			forms[key] = map[string]bool{}
		}
		if s := dpd.UnpackList(l.Deconstructor); len(s) > 0 {
			splits[key] = s
		}
		return nil
	})
	if err != nil {
		return DpdStats{}, fmt.Errorf("parse dpd lookup: %w", err)
	}

	if err := errors.Join(headwordsOut.close(), rootsOut.close()); err != nil {
		return DpdStats{}, fmt.Errorf("parse dpd: %w", err)
	}

	keys := make([]string, 0, len(forms))
	for k := range forms {
		keys = append(keys, k)
	}
	sortStrings(keys)
	formsOut, err := newNDJSON(cfg.Canon.ParsedDir(), "dpd-forms.ndjson")
	if err != nil {
		return DpdStats{}, fmt.Errorf("parse dpd: %w", err)
	}
	for _, k := range keys {
		hw := make([]string, 0, len(forms[k]))
		for h := range forms[k] {
			hw = append(hw, h)
		}
		sortStrings(hw)
		sp := splits[k]
		if sp == nil {
			sp = []string{}
		}
		if err := formsOut.write(model.DpdForm{Form: k, Headwords: hw, Splits: sp}); err != nil {
			_ = formsOut.close()
			return DpdStats{}, fmt.Errorf("parse dpd forms: %w", err)
		}
	}
	if err := formsOut.close(); err != nil {
		return DpdStats{}, fmt.Errorf("parse dpd forms: %w", err)
	}
	stats.DpdForms = len(keys)
	return stats, nil
}
