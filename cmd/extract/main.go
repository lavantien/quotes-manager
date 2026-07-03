// Command extract reads the sutta-quote dumps, normalizes every quote into a
// single canonical format, and writes database/seed.sql and the shortest-first
// text export. Run the sqlite3 CLI against seed.sql to populate the database.
package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"

	"github.com/lavantien/quotes-manager/internal/quote"
)

func main() {
	log.SetFlags(0)
	count, err := generate("dumps", "database/seed.sql", "exports/shortest-first.md")
	if err != nil {
		log.Fatalf("extract: %v", err)
	}
	fmt.Printf("extracted %d unique quotes\n", count)
	fmt.Println("  -> database/seed.sql")
	fmt.Println("  -> exports/shortest-first.md")
}

// generate reads the dumps, normalizes and sorts the quotes, and writes the
// seed SQL and text export. It returns the number of unique quotes written.
func generate(dumpsDir, seedPath, exportPath string) (int, error) {
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

// loadDumps reads dumps/*.txt in filename order as named files.
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
