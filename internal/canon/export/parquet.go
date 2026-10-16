package export

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lavantien/quotes-manager/internal/config"
)

func writeParquet(ctx context.Context, ddb *sql.DB, cfg *config.Config) error {
	if err := os.MkdirAll(cfg.Canon.ParquetDir(), 0o755); err != nil {
		return fmt.Errorf("export parquet dir: %w", err)
	}
	for _, t := range tableLoads {
		target := filepath.Join(cfg.Canon.ParquetDir(), t.name+".parquet")
		if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("export parquet %s: %w", t.name, err)
		}
		path := strings.ReplaceAll(filepath.ToSlash(target), "'", "''")
		q := fmt.Sprintf(`COPY (SELECT * FROM %s) TO '%s' (FORMAT PARQUET)`, t.name, path)
		if _, err := ddb.ExecContext(ctx, q); err != nil {
			return fmt.Errorf("export parquet %s: %w", t.name, err)
		}
	}
	return nil
}
