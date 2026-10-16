package triangulate

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/lavantien/quotes-manager/internal/canon/model"
	"github.com/lavantien/quotes-manager/internal/config"
)

type Store interface {
	DB() *sql.DB
	InsertDiscrepancies(ctx context.Context, runID int64, ds []model.Discrepancy) error
	SetCoversEdge(ctx context.Context, muid, editionKey, dataJSON string) error
}

type HTMLRef struct {
	Author     string
	Collection string
	BuildDate  string
	FileName   string
}

type Env struct {
	RunID            int64
	ParsedDir        string
	HtmlSegmentsFile string
	EditionsHTML     []HTMLRef
	BilaraNewerThan  func(author string) (string, bool)
	LastRunDate      string
	DpdDBPath        string
	DpdFormsFile     string
}

type Check struct {
	ID  model.CheckID
	Run func(ctx context.Context, cfg *config.Config, st Store, env Env) ([]model.Discrepancy, error)
}

func All() []Check {
	return []Check{
		{ID: model.CheckKeysetRootTranslation, Run: runB001},
		{ID: model.CheckKeysetRootHTML, Run: runB002},
		{ID: model.CheckKeysetReference, Run: runB003},
		{ID: model.CheckDuplicateKeys, Run: runB004},
		{ID: model.CheckEmptyValue, Run: runB005},
		{ID: model.CheckNFCChanged, Run: runB006},
		{ID: model.CheckPrefixMismatch, Run: runB007},
		{ID: model.CheckEditionsCoverage, Run: runE101},
		{ID: model.CheckEditionsContent, Run: runE102},
		{ID: model.CheckEditionsStaleness, Run: runE103},
		{ID: model.CheckEditionsLastRun, Run: runE104},
		{ID: model.CheckDpdUnknownForms, Run: runD001},
		{ID: model.CheckDpdDegraded, Run: runD002},
	}
}

type Summary struct {
	Counts    map[string]int
	HasErrors bool
	Total     int
}

func Run(ctx context.Context, cfg *config.Config, st Store, env Env) (Summary, error) {
	if cfg == nil {
		cfg = config.Defaults()
	}
	if st == nil {
		return Summary{}, fmt.Errorf("triangulate: nil store")
	}
	if _, err := st.DB().ExecContext(ctx, `DELETE FROM discrepancy WHERE run_id = ?`, env.RunID); err != nil {
		return Summary{}, fmt.Errorf("triangulate clear run %d: %w", env.RunID, err)
	}
	sum := Summary{Counts: map[string]int{}}
	var all []model.Discrepancy
	for _, c := range All() {
		ds, err := c.Run(ctx, cfg, st, env)
		if err != nil {
			ds = []model.Discrepancy{{
				Check:    c.ID,
				Severity: directionSeverity(cfg, string(c.ID)+".run", model.Error),
				Detail:   fmt.Sprintf("check run failed: %v", err),
			}}
		}
		for _, d := range ds {
			sum.Counts[fmt.Sprintf("%s/%s", d.Check, d.Severity)]++
			if d.Severity == model.Error {
				sum.HasErrors = true
			}
		}
		all = append(all, ds...)
	}
	if err := st.InsertDiscrepancies(ctx, env.RunID, all); err != nil {
		return Summary{}, fmt.Errorf("triangulate insert discrepancies: %w", err)
	}
	sum.Total = len(all)
	return sum, nil
}

func directionSeverity(cfg *config.Config, key string, def model.Severity) model.Severity {
	if _, ok := cfg.Canon.Triangulate.Severity[key]; ok {
		return model.Severity(cfg.Canon.SeverityFor(key))
	}
	return def
}

func severityRank(s model.Severity) int {
	switch s {
	case model.Info:
		return 0
	case model.Warn:
		return 1
	default:
		return 2
	}
}

func capJoin(items []string, max int) string {
	joined := strings.Join(items, ", ")
	if len(joined) <= max {
		return joined
	}
	var kept []string
	used := 0
	for _, it := range items {
		cost := len(it)
		if len(kept) > 0 {
			cost += 2
		}
		if used+cost > max {
			break
		}
		kept = append(kept, it)
		used += cost
	}
	s := strings.Join(kept, ", ")
	if rest := len(items) - len(kept); rest > 0 {
		s += fmt.Sprintf(" (+%d more)", rest)
	}
	return s
}
