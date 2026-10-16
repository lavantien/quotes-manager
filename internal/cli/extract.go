package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/lavantien/quotes-manager/internal/config"
	"github.com/lavantien/quotes-manager/internal/quote"
)

func init() {
	Register(Command{Name: "extract", Usage: "dumps -> seed sql + shortest-first export", Run: runExtract})
}

func runExtract(_ context.Context, cfg *config.Config, args []string) error {
	if err := noArgs("extract", args); err != nil {
		return err
	}
	count, err := extractGenerate(cfg.Quotes.DumpsDir, cfg.Quotes.SeedFile, cfg.Quotes.ExportFile)
	if err != nil {
		return fmt.Errorf("extract: %w", err)
	}
	printLine("extracted %d unique quotes\n", count)
	printLine("  -> %s\n", cfg.Quotes.SeedFile)
	printLine("  -> %s\n", cfg.Quotes.ExportFile)
	return nil
}

func extractGenerate(dumpsDir, seedPath, exportPath string) (int, error) {
	files, err := loadDumps(dumpsDir)
	if err != nil {
		return 0, err
	}
	quotes := quote.Parse(files)
	quotes = quote.Dedup(quotes)
	quote.SortByCharCount(quotes)

	for _, dir := range []string{filepath.Dir(seedPath), filepath.Dir(exportPath)} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return 0, err
		}
	}
	if err := os.WriteFile(seedPath, []byte(quote.RenderSeedSQL(quotes)), 0o644); err != nil {
		return 0, err
	}
	if err := os.WriteFile(exportPath, []byte(quote.RenderExportFile(quotes)), 0o644); err != nil {
		return 0, err
	}
	return len(quotes), nil
}

func loadDumps(dir string) ([]quote.File, error) {
	matches, err := filepath.Glob(filepath.Join(dir, "*.txt"))
	if err != nil {
		return nil, err
	}
	sort.Strings(matches)
	files := make([]quote.File, 0, len(matches))
	for _, m := range matches {
		b, err := os.ReadFile(m)
		if err != nil {
			return nil, err
		}
		files = append(files, quote.File{Name: filepath.Base(m), Content: string(b)})
	}
	return files, nil
}
