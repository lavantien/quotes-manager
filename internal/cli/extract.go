package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/lavantien/quotes-manager/internal/config"
	"github.com/lavantien/quotes-manager/internal/quote"
	"github.com/lavantien/quotes-manager/internal/sutta"
)

func init() {
	Register(Command{Name: "extract", Usage: "dumps -> seed sql + shortest-first export", Run: runExtract})
}

func runExtract(_ context.Context, cfg *config.Config, args []string) error {
	if err := noArgs("extract", args); err != nil {
		return err
	}
	count, err := extractGenerate(cfg.Quotes.DumpsDir, cfg.Quotes.SeedFile, cfg.Quotes.ExportFile, cfg.Quotes.TablesCollection)
	if err != nil {
		return fmt.Errorf("extract: %w", err)
	}
	printLine("extracted %d unique quotes\n", count)
	printLine("  -> %s\n", cfg.Quotes.SeedFile)
	printLine("  -> %s\n", cfg.Quotes.ExportFile)
	return nil
}

func extractGenerate(dumpsDir, seedPath, exportPath, collection string) (int, error) {
	files, err := loadDumps(dumpsDir)
	if err != nil {
		return 0, err
	}
	quotes := quote.Parse(files)
	quotes = quote.Dedup(quotes)
	quote.SortByCharCount(quotes)

	tables, err := parseTables(dumpsDir)
	if err != nil {
		return 0, err
	}

	for _, dir := range []string{filepath.Dir(seedPath), filepath.Dir(exportPath)} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return 0, err
		}
	}
	var seed strings.Builder
	seed.WriteString(quote.RenderSeedSQL(quotes))
	if len(tables) > 0 {
		seed.WriteString("\n")
		seed.WriteString(sutta.RenderSeedSQL(tables, collection))
	}
	if err := os.WriteFile(seedPath, []byte(seed.String()), 0o644); err != nil {
		return 0, err
	}
	if err := os.WriteFile(exportPath, []byte(quote.RenderExportFile(quotes)), 0o644); err != nil {
		return 0, err
	}
	return len(quotes), nil
}

func parseTables(dumpsDir string) ([]sutta.Table, error) {
	matches, err := filepath.Glob(filepath.Join(dumpsDir, "*.md"))
	if err != nil {
		return nil, err
	}
	sort.Strings(matches)
	files := make([]sutta.File, 0, len(matches))
	for _, m := range matches {
		b, err := os.ReadFile(m)
		if err != nil {
			return nil, err
		}
		files = append(files, sutta.File{Name: filepath.Base(m), Content: string(b)})
	}
	if len(files) == 0 {
		return nil, nil
	}
	tables, err := sutta.ParseFiles(files)
	if err != nil {
		return nil, fmt.Errorf("table dump: %w", err)
	}
	return tables, nil
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
