package cli

import (
	"context"
	"database/sql"
	"fmt"
	"maps"
	"slices"

	"github.com/lavantien/quotes-manager/internal/canon/graph"
	"github.com/lavantien/quotes-manager/internal/canon/source/editions"
	"github.com/lavantien/quotes-manager/internal/canon/triangulate"
	"github.com/lavantien/quotes-manager/internal/config"
)

func init() {
	Register(Command{Name: "triangulate", Usage: "run all triangulation checks against the current database", Run: runTriangulate})
}

func triangulateEnv(cfg *config.Config) (triangulate.Env, error) {
	env := triangulate.Env{
		ParsedDir:        cfg.Canon.ParsedDir(),
		HtmlSegmentsFile: cfg.Canon.HtmlSegmentsFile(),
		DpdDBPath:        cfg.Canon.DpdDB(),
		DpdFormsFile:     cfg.Canon.DpdFormsFile(),
		BilaraNewerThan: func(author string) (string, bool) {
			d, err := editions.LatestCommit(cfg.Canon.BilaraDir(), "translation/en/"+author)
			return d, err == nil
		},
	}
	if htmls, err := editions.FindHTML(cfg.Canon.EditionsDir()); err == nil {
		for _, h := range htmls {
			env.EditionsHTML = append(env.EditionsHTML, triangulate.HTMLRef{
				Author:     h.Author,
				Collection: h.Collection,
				BuildDate:  h.BuildDate,
				FileName:   h.FileName,
			})
		}
	}
	if d, err := editions.ReadLastRunDate(cfg.Canon.EditionsDir()); err == nil {
		env.LastRunDate = d
	}
	return env, nil
}

func runTriangulate(ctx context.Context, cfg *config.Config, args []string) error {
	if err := noArgs("triangulate", args); err != nil {
		return err
	}
	_, dbPath, err := graph.CurrentDB(cfg)
	if err != nil {
		return fmt.Errorf("triangulate: %w", err)
	}
	store, err := graph.Open(dbPath)
	if err != nil {
		return fmt.Errorf("triangulate: %w", err)
	}
	defer func() { _ = store.Close() }()

	var runID sql.NullInt64
	if err := store.DB().QueryRowContext(ctx, `SELECT MAX(id) FROM run`).Scan(&runID); err != nil {
		return fmt.Errorf("triangulate: latest run: %w", err)
	}
	if !runID.Valid {
		return fmt.Errorf("triangulate: current database has no run row; run load first")
	}
	env, err := triangulateEnv(cfg)
	if err != nil {
		return fmt.Errorf("triangulate: %w", err)
	}
	env.RunID = runID.Int64

	sum, err := triangulate.Run(ctx, cfg, store, env)
	if err != nil {
		return fmt.Errorf("triangulate: %w", err)
	}
	for _, k := range slices.Sorted(maps.Keys(sum.Counts)) {
		printLine("%s: %d\n", k, sum.Counts[k])
	}
	printLine("total: %d\n", sum.Total)
	if sum.HasErrors {
		return fmt.Errorf("triangulation reported errors: %d findings, see report", sum.Total)
	}
	return nil
}
