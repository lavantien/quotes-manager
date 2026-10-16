package triangulate

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/lavantien/quotes-manager/internal/canon/model"
	"github.com/lavantien/quotes-manager/internal/canon/norm"
	"github.com/lavantien/quotes-manager/internal/config"
)

func runD001(ctx context.Context, cfg *config.Config, st Store, env Env) ([]model.Discrepancy, error) {
	known, err := loadDpdKnownForms(env.DpdFormsFile)
	if err != nil {
		return nil, fmt.Errorf("d001: %w", err)
	}
	if known == nil {
		return nil, nil
	}
	unknownByMuid, err := streamUnknownForms(env.ParsedDir, known, cfg.Canon.Dpd.MinTokenLength)
	if err != nil {
		return nil, fmt.Errorf("d001: %w", err)
	}
	below := model.Severity(cfg.Canon.SeverityFor(string(model.CheckDpdUnknownForms)))
	atOrAbove := directionSeverity(cfg, string(model.CheckDpdUnknownForms)+".escalated", model.Warn)
	if severityRank(atOrAbove) < severityRank(below) {
		atOrAbove = below
	}
	var ds []model.Discrepancy
	for _, muid := range sortedFormMuids(unknownByMuid) {
		forms := sortedKeys(unknownByMuid[muid])
		sev := below
		if len(forms) >= cfg.Canon.Dpd.UnknownWarnPerMuid {
			sev = atOrAbove
		}
		ds = append(ds, model.Discrepancy{
			Check:    model.CheckDpdUnknownForms,
			Severity: sev,
			Muid:     muid,
			Detail: fmt.Sprintf("unknown forms: %d (threshold %d): %s",
				len(forms), cfg.Canon.Dpd.UnknownWarnPerMuid,
				capJoin(forms, cfg.Canon.Triangulate.DetailSampleMax)),
		})
	}
	return ds, nil
}

type dpdFormState struct {
	plain    bool
	resolved bool
}

func loadDpdKnownForms(path string) (map[string]bool, error) {
	if path == "" {
		return nil, nil
	}
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("open dpd forms %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()

	states := map[string]*dpdFormState{}
	state := func(key string) *dpdFormState {
		st, ok := states[key]
		if !ok {
			st = &dpdFormState{}
			states[key] = st
		}
		return st
	}
	type splitRec struct {
		key    string
		splits []string
	}
	var splitRecs []splitRec
	r := bufio.NewReader(f)
	for {
		line, rerr := r.ReadString('\n')
		if line != "" {
			var rec model.DpdForm
			if jerr := json.Unmarshal([]byte(strings.TrimRight(line, "\r\n")), &rec); jerr != nil {
				return nil, fmt.Errorf("bad dpd form line: %w", jerr)
			}
			if key := norm.FoldPali(rec.Form); key != "" {
				st := state(key)
				if len(rec.Splits) == 0 || len(rec.Headwords) > 0 {
					st.plain = true
				} else {
					splitRecs = append(splitRecs, splitRec{key: key, splits: rec.Splits})
				}
			}
		}
		if rerr != nil {
			if errors.Is(rerr, io.EOF) {
				break
			}
			return nil, fmt.Errorf("read dpd forms: %w", rerr)
		}
	}
	for _, sr := range splitRecs {
		if dpdSplitsResolve(sr.splits, states) {
			state(sr.key).resolved = true
		}
	}
	keys := make([]string, 0, len(states))
	for k := range states {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	known := make(map[string]bool, len(keys))
	for _, k := range keys {
		if states[k].plain || states[k].resolved {
			known[k] = true
		}
	}
	return known, nil
}

func dpdSplitsResolve(splits []string, states map[string]*dpdFormState) bool {
	for _, sp := range splits {
		for _, part := range strings.Split(sp, " + ") {
			key := norm.FoldPali(strings.TrimSpace(part))
			if key == "" {
				return false
			}
			if _, ok := states[key]; !ok {
				return false
			}
		}
	}
	return true
}

func streamUnknownForms(parsedDir string, known map[string]bool, minTokenLength int) (map[string]map[string]bool, error) {
	if parsedDir == "" {
		return nil, nil
	}
	path := filepath.Join(parsedDir, string(model.StreamRoot)+".ndjson")
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open root stream: %w", err)
	}
	defer func() { _ = f.Close() }()
	out := map[string]map[string]bool{}
	r := bufio.NewReader(f)
	for {
		line, rerr := r.ReadString('\n')
		if line != "" {
			var rec model.SegmentRecord
			if jerr := json.Unmarshal([]byte(strings.TrimRight(line, "\r\n")), &rec); jerr != nil {
				return nil, fmt.Errorf("bad root record: %w", jerr)
			}
			if rec.Stream != model.StreamRoot {
				continue
			}
			for _, tok := range norm.TokenizePali(rec.Text, minTokenLength) {
				if known[tok] {
					continue
				}
				if out[rec.Muid] == nil {
					out[rec.Muid] = map[string]bool{}
				}
				out[rec.Muid][tok] = true
			}
		}
		if rerr != nil {
			if errors.Is(rerr, io.EOF) {
				return out, nil
			}
			return nil, fmt.Errorf("read root stream: %w", rerr)
		}
	}
}

func sortedFormMuids(m map[string]map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func runD002(ctx context.Context, cfg *config.Config, st Store, env Env) ([]model.Discrepancy, error) {
	degraded := []model.Discrepancy{{
		Check:    model.CheckDpdDegraded,
		Severity: model.Severity(cfg.Canon.SeverityFor(string(model.CheckDpdDegraded))),
		Detail:   "dpd database absent; dpd checks degraded (D001 skipped)",
	}}
	missing, err := env.dpdAbsent()
	if err != nil {
		return nil, fmt.Errorf("d002: %w", err)
	}
	if missing {
		return degraded, nil
	}
	return nil, nil
}

func (e Env) dpdAbsent() (bool, error) {
	if e.DpdDBPath == "" {
		return true, nil
	}
	fi, err := os.Stat(e.DpdDBPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return true, nil
		}
		return false, fmt.Errorf("stat dpd db: %w", err)
	}
	if fi.IsDir() {
		return false, fmt.Errorf("dpd db %s is not a file", e.DpdDBPath)
	}
	if e.DpdFormsFile == "" {
		return true, nil
	}
	ffi, err := os.Stat(e.DpdFormsFile)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return true, nil
		}
		return false, fmt.Errorf("stat dpd forms: %w", err)
	}
	if ffi.IsDir() {
		return false, fmt.Errorf("dpd forms %s is not a file", e.DpdFormsFile)
	}
	return ffi.Size() == 0, nil
}
