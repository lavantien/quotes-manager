package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/lavantien/quotes-manager/internal/canon/pipeline"
	"github.com/lavantien/quotes-manager/internal/canon/source/dpd"
	"github.com/lavantien/quotes-manager/internal/canon/source/editions"
	"github.com/lavantien/quotes-manager/internal/canon/sync"
	"github.com/lavantien/quotes-manager/internal/config"
)

func init() {
	Register(Command{
		Name:  "sync",
		Usage: "update and prune the mirrors: --source bilara|editions|dpd|all (default all)",
		Run:   runSync,
	})
}

func mirrorSource(name string) ([]string, error) {
	switch name {
	case "all":
		return []string{"bilara", "editions", "dpd"}, nil
	case "bilara", "editions", "dpd":
		return []string{name}, nil
	default:
		return nil, fmt.Errorf("unknown --source %q: want bilara, editions, dpd, or all", name)
	}
}

func runSync(ctx context.Context, cfg *config.Config, args []string) error {
	fs := flag.NewFlagSet("sync", flag.ContinueOnError)
	source := fs.String("source", "all", "bilara|editions|dpd|all")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("sync takes no positional arguments: %q", fs.Arg(0))
	}
	kinds, err := mirrorSource(*source)
	if err != nil {
		return err
	}
	for _, kind := range kinds {
		if kind == "dpd" {
			if err := syncDpd(ctx, cfg); err != nil {
				return fmt.Errorf("sync dpd: %w", err)
			}
			continue
		}
		m, err := sync.New(cfg, kind)
		if err != nil {
			return err
		}
		if err := m.Update(ctx); err != nil {
			return fmt.Errorf("sync %s: %w", kind, err)
		}
		removed, err := m.Prune()
		if err != nil {
			return fmt.Errorf("prune %s: %w", kind, err)
		}
		if Verbose(ctx) {
			printLine("sync %s: pruned %d files\n", kind, removed)
		}
	}
	return writeSyncManifest(ctx, cfg, kinds)
}

func nowUTC() string { return time.Now().UTC().Format(time.RFC3339) }

func syncDpd(ctx context.Context, cfg *config.Config) error {
	if err := os.MkdirAll(cfg.Canon.DpdDir(), 0o755); err != nil {
		return err
	}
	pv, err := dpd.Fetch(ctx, dpd.FetchOpts{
		URL:          cfg.Canon.Dpd.URL,
		PinnedSHA256: cfg.Canon.Dpd.SHA256,
		MaxBytes:     cfg.Canon.Dpd.MaxTarballBytes,
		Timeout:      time.Duration(cfg.Canon.Dpd.TimeoutSeconds) * time.Second,
	}, cfg.Canon.DpdTarball())
	if err != nil {
		return err
	}
	if err := dpd.Extract(cfg.Canon.DpdTarball(), cfg.Canon.DpdDB(), cfg.Canon.Dpd.MaxDBBytes); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(pipeline.DpdProvenance{
		Tag: pv.Tag, SHA256: pv.SHA256, Size: pv.Size,
		Fetched: pv.Fetched.Format(time.RFC3339),
	}, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(cfg.Canon.DpdProvenanceFile(), append(raw, '\n'), 0o644); err != nil {
		return fmt.Errorf("dpd provenance: %w", err)
	}
	return nil
}

func writeSyncManifest(ctx context.Context, cfg *config.Config, kinds []string) error {
	synced := map[string]bool{}
	for _, k := range kinds {
		synced[k] = true
	}
	m, _ := pipeline.ReadManifest(cfg)
	m.Updated = nowUTC()
	if synced["bilara"] || dirExists(cfg.Canon.BilaraDir()) {
		bm, err := sync.New(cfg, "bilara")
		if err != nil {
			return err
		}
		if m.Bilara.Commit, err = bm.Commit(); err != nil {
			return fmt.Errorf("bilara commit: %w", err)
		}
		if m.Bilara.Date, err = editions.LatestCommit(cfg.Canon.BilaraDir(), ""); err != nil {
			return fmt.Errorf("bilara commit date: %w", err)
		}
	}
	if synced["editions"] || dirExists(cfg.Canon.EditionsDir()) {
		em, err := sync.New(cfg, "editions")
		if err != nil {
			return err
		}
		if m.Editions.Commit, err = em.Commit(); err != nil {
			return fmt.Errorf("editions commit: %w", err)
		}
		htmls, err := editions.FindHTML(cfg.Canon.EditionsDir())
		if err != nil {
			return fmt.Errorf("editions html: %w", err)
		}
		m.Htmls = nil
		for _, h := range htmls {
			m.Htmls = append(m.Htmls, pipeline.HtmlEdition{
				Author:     h.Author,
				Collection: h.Collection,
				FileName:   h.FileName,
				BuildDate:  h.BuildDate,
			})
		}
	}
	if synced["dpd"] || fileExists(cfg.Canon.DpdProvenanceFile()) {
		pv, err := readDpdProvenance(cfg)
		if err != nil {
			return err
		}
		m.Dpd = pv
	}
	if err := pipeline.WriteManifest(cfg, m); err != nil {
		return err
	}
	if Verbose(ctx) {
		printLine("manifest: %s with %d htmls\n", cfg.Canon.ManifestFile(), len(m.Htmls))
	}
	return nil
}

func readDpdProvenance(cfg *config.Config) (pipeline.DpdProvenance, error) {
	raw, err := os.ReadFile(cfg.Canon.DpdProvenanceFile())
	if err != nil {
		return pipeline.DpdProvenance{}, fmt.Errorf("dpd provenance: %w", err)
	}
	var pv pipeline.DpdProvenance
	if err := json.Unmarshal(raw, &pv); err != nil {
		return pipeline.DpdProvenance{}, fmt.Errorf("dpd provenance %s: %w", cfg.Canon.DpdProvenanceFile(), err)
	}
	return pv, nil
}

func dirExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func fileExists(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && !fi.IsDir()
}
