package pipeline

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/lavantien/quotes-manager/internal/canon/graph"
	"github.com/lavantien/quotes-manager/internal/canon/model"
	"github.com/lavantien/quotes-manager/internal/canon/norm"
	"github.com/lavantien/quotes-manager/internal/config"
)

type headwordPayload struct {
	Lemma1       string `json:"lemma_1"`
	Lemma2       string `json:"lemma_2,omitempty"`
	POS          string `json:"pos,omitempty"`
	Grammar      string `json:"grammar,omitempty"`
	Meaning1     string `json:"meaning_1,omitempty"`
	MeaningLit   string `json:"meaning_lit,omitempty"`
	RootKey      string `json:"root_key,omitempty"`
	RootSign     string `json:"root_sign,omitempty"`
	FamilyRoot   string `json:"family_root,omitempty"`
	Construction string `json:"construction,omitempty"`
	CompoundType string `json:"compound_type,omitempty"`
	Stem         string `json:"stem,omitempty"`
}

type wordFormsPayload struct {
	Forms []string `json:"forms"`
}

func loadDpd(cfg *config.Config, loader *graph.Loader, man Manifest) (map[string]int64, error) {
	if _, err := os.Stat(cfg.Canon.DpdHeadwordsFile()); err != nil {
		if os.IsNotExist(err) {
			return map[string]int64{}, nil
		}
		return nil, fmt.Errorf("load dpd: %w", err)
	}
	if _, err := loader.EnsureNode(model.NodeSource, "dpd", mustJSON(sourcePayload{
		Repo: cfg.Canon.Dpd.URL, Commit: man.Dpd.Tag, FetchedAt: man.Dpd.Fetched, SHA256: man.Dpd.SHA256,
	})); err != nil {
		return nil, err
	}

	rootIDs := map[string]int64{}
	err := streamLines(cfg.Canon.DpdRootsFile(), func(line []byte) error {
		var r model.DpdRoot
		if err := json.Unmarshal(line, &r); err != nil {
			return fmt.Errorf("bad dpd root line: %w", err)
		}
		id, err := loader.EnsureNode(model.NodeDpdRoot, "dpd-root:"+r.Root, mustJSON(r))
		if err != nil {
			return err
		}
		rootIDs[r.Root] = id
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("load dpd roots: %w", err)
	}

	hwIDs := map[string]int64{}
	err = streamLines(cfg.Canon.DpdHeadwordsFile(), func(line []byte) error {
		var h model.DpdHeadword
		if err := json.Unmarshal(line, &h); err != nil {
			return fmt.Errorf("bad dpd headword line: %w", err)
		}
		if h.Lemma1 == "" {
			return nil
		}
		id, err := loader.EnsureNode(model.NodeHeadword, "dpd:"+h.Lemma1, mustJSON(headwordPayload{
			Lemma1: h.Lemma1, Lemma2: h.Lemma2, POS: h.Pos, Grammar: h.Grammar,
			Meaning1: h.Meaning1, MeaningLit: h.MeaningLit,
			RootKey: h.RootKey, RootSign: h.RootSign, FamilyRoot: h.FamilyRoot,
			Construction: h.Construction, CompoundType: h.CompoundType, Stem: h.Stem,
		}))
		if err != nil {
			return err
		}
		hwIDs["dpd:"+h.Lemma1] = id
		if r, ok := rootIDs[h.RootKey]; ok {
			return loader.AddEdge(id, r, model.EdgeDerivedFrom, nil, "")
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("load dpd headwords: %w", err)
	}

	err = streamLines(cfg.Canon.DpdFormsFile(), func(line []byte) error {
		var f model.DpdForm
		if err := json.Unmarshal(line, &f); err != nil {
			return fmt.Errorf("bad dpd form line: %w", err)
		}
		_, err := loader.EnsureNode(model.NodeDpdForm, "dpd-form:"+f.Form, mustJSON(f))
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("load dpd forms: %w", err)
	}
	return hwIDs, nil
}

func linkWords(cfg *config.Config, loader *graph.Loader, paliIDs, hwIDs map[string]int64) error {
	forms := map[string][]string{}
	err := streamLines(cfg.Canon.DpdFormsFile(), func(line []byte) error {
		var f model.DpdForm
		if err := json.Unmarshal(line, &f); err != nil {
			return fmt.Errorf("bad dpd form line: %w", err)
		}
		forms[f.Form] = f.Headwords
		return nil
	})
	if err != nil {
		return err
	}
	if len(forms) == 0 {
		return nil
	}

	return streamNDJSON(filepath.Join(cfg.Canon.ParsedDir(), string(model.StreamRoot)+".ndjson"), func(rec segLine) error {
		segID, ok := paliIDs["pli:"+rec.Muid+":"+rec.SegKey]
		if !ok {
			return nil
		}
		perHeadword := map[string][]string{}
		for _, tok := range norm.TokenizePali(rec.Text, cfg.Canon.Dpd.MinTokenLength) {
			for _, h := range forms[tok] {
				key := "dpd:" + h
				if !slices.Contains(perHeadword[key], tok) {
					perHeadword[key] = append(perHeadword[key], tok)
				}
			}
		}
		keys := make([]string, 0, len(perHeadword))
		for k := range perHeadword {
			keys = append(keys, k)
		}
		sortStrings(keys)
		for _, k := range keys {
			id, ok := hwIDs[k]
			if !ok {
				continue
			}
			fs := perHeadword[k]
			sortStrings(fs)
			if err := loader.AddEdge(segID, id, model.EdgeHasWord, nil, mustJSON(wordFormsPayload{Forms: fs})); err != nil {
				return err
			}
		}
		return nil
	})
}

func streamLines(path string, fn func(line []byte) error) error {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer func() { _ = f.Close() }()
	r := bufio.NewReader(f)
	for {
		line, err := r.ReadString('\n')
		if trimmed := strings.TrimRight(line, "\r\n"); trimmed != "" {
			if ferr := fn([]byte(trimmed)); ferr != nil {
				return ferr
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
	}
}
