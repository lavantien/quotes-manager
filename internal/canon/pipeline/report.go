package pipeline

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/lavantien/quotes-manager/internal/canon/graph"
	"github.com/lavantien/quotes-manager/internal/canon/model"
	"github.com/lavantien/quotes-manager/internal/canon/report"
	"github.com/lavantien/quotes-manager/internal/config"
)

const reportDateLayout = "2006-01-02"

func Report(ctx context.Context, cfg *config.Config, outDir string) error {
	runID, dbPath, err := graph.CurrentDB(cfg)
	if err != nil {
		return fmt.Errorf("report: %w", err)
	}
	store, err := graph.Open(dbPath)
	if err != nil {
		return fmt.Errorf("report: %w", err)
	}
	defer func() { _ = store.Close() }()

	date := time.Now().UTC().Format(reportDateLayout)
	in := report.Input{
		Date:              date,
		Provenance:        map[string]string{},
		Counts:            map[string]int64{},
		DiscrepancyCounts: map[string]int{},
		DetailSampleMax:   cfg.Canon.Triangulate.DetailSampleMax,
	}
	if man, err := ReadManifest(cfg); err == nil {
		in.Provenance["bilara_commit"] = man.Bilara.Commit
		in.Provenance["bilara_date"] = man.Bilara.Date
		in.Provenance["editions_commit"] = man.Editions.Commit
		in.Provenance["dpd_tag"] = man.Dpd.Tag
		in.Provenance["dpd_sha256"] = man.Dpd.SHA256
		in.Provenance["dpd_fetched"] = man.Dpd.Fetched
		in.Provenance["synced"] = man.Updated
	}
	in.Provenance["run"] = runID
	in.Provenance["db"] = filepath.Base(dbPath)

	counts, err := store.Stats(ctx)
	if err != nil {
		return fmt.Errorf("report stats: %w", err)
	}
	in.Counts = counts

	coverage, err := coverageRows(ctx, store)
	if err != nil {
		return fmt.Errorf("report coverage: %w", err)
	}
	in.Coverage = coverage

	runRow, discs, err := readDiscrepancies(ctx, store)
	if err != nil {
		return fmt.Errorf("report discrepancies: %w", err)
	}
	in.Discrepancies = discs
	for _, d := range discs {
		in.DiscrepancyCounts[string(d.Check)+"/"+string(d.Severity)]++
	}
	in.Provenance["triangulate_run"] = fmt.Sprintf("%d", runRow)

	if outDir == "" {
		outDir = cfg.Canon.ReportsDirFor(date)
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("report: %w", err)
	}
	md := report.RenderMarkdown(in)
	if err := os.WriteFile(filepath.Join(outDir, "report.md"), []byte(md), 0o644); err != nil {
		return fmt.Errorf("report: %w", err)
	}
	raw, err := report.RenderJSON(in)
	if err != nil {
		return fmt.Errorf("report: %w", err)
	}
	if err := os.WriteFile(filepath.Join(outDir, "report.json"), raw, 0o644); err != nil {
		return fmt.Errorf("report: %w", err)
	}
	return nil
}

func coverageRows(ctx context.Context, store *graph.Store) ([]report.CoverageRow, error) {
	rows, err := store.DB().QueryContext(ctx, `
		SELECT dv.key, COUNT(DISTINCT t.id), COUNT(DISTINCT
			CASE WHEN EXISTS (
				SELECT 1 FROM edge c WHERE c.src = t.id AND c.type = 'COVERED_BY'
			) THEN t.id END)
		FROM node dv
		JOIN edge tch ON tch.dst = dv.id AND tch.type = 'CHILD_OF'
		JOIN node t ON t.id = tch.src AND t.type = 'text'
		WHERE dv.type = 'division'
		GROUP BY dv.key
		ORDER BY dv.key`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []report.CoverageRow
	for rows.Next() {
		var r report.CoverageRow
		if err := rows.Scan(&r.Division, &r.Texts, &r.Covered); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func readDiscrepancies(ctx context.Context, store *graph.Store) (int64, []model.Discrepancy, error) {
	var runID int64
	err := store.DB().QueryRowContext(ctx,
		`SELECT id FROM run ORDER BY id DESC LIMIT 1`).Scan(&runID)
	if err != nil {
		return 0, nil, fmt.Errorf("latest run: %w", err)
	}
	rows, err := store.DB().QueryContext(ctx, `
		SELECT "check", severity, muid, node_id, detail
		FROM discrepancy WHERE run_id = ? ORDER BY id`, runID)
	if err != nil {
		return runID, nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []model.Discrepancy
	for rows.Next() {
		var d model.Discrepancy
		var muid *string
		var nodeID *int64
		if err := rows.Scan(&d.Check, &d.Severity, &muid, &nodeID, &d.Detail); err != nil {
			return runID, nil, err
		}
		if muid != nil {
			d.Muid = *muid
		}
		if nodeID != nil {
			d.NodeID = *nodeID
		}
		out = append(out, d)
	}
	return runID, out, rows.Err()
}
