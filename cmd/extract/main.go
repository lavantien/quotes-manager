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

	files, err := loadDumps("dumps")
	if err != nil {
		log.Fatalf("load dumps: %v", err)
	}

	quotes := quote.Parse(files)
	quotes = quote.Dedup(quotes)
	quote.SortByCharCount(quotes)

	for _, dir := range []string{"database", "exports"} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			log.Fatalf("mkdir %s: %v", dir, err)
		}
	}
	if err := os.WriteFile("database/seed.sql", []byte(quote.RenderSeedSQL(quotes)), 0o644); err != nil {
		log.Fatalf("write seed.sql: %v", err)
	}
	if err := os.WriteFile("exports/shortest-first.md", []byte(quote.RenderExportFile(quotes)), 0o644); err != nil {
		log.Fatalf("write export: %v", err)
	}

	fmt.Printf("extracted %d unique quotes\n", len(quotes))
	fmt.Println("  -> database/seed.sql")
	fmt.Println("  -> exports/shortest-first.md")
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
