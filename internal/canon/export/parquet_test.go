package export

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestRebuildWritesParquetPerTable(t *testing.T) {
	cfg := testConfig(t)
	buildFeedsGraphDB(t, cfg)

	counts, err := Rebuild(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	ddb := openSidecar(t, cfg)

	for _, tl := range tableLoads {
		path := filepath.Join(cfg.Canon.ParquetDir(), tl.name+".parquet")
		if _, err := os.Stat(path); err != nil {
			t.Errorf("parquet for %s missing: %v", tl.name, err)
			continue
		}
		var n int64
		q := fmt.Sprintf(`SELECT COUNT(*) FROM read_parquet('%s')`,
			filepath.ToSlash(path))
		if err := ddb.QueryRow(q).Scan(&n); err != nil {
			t.Errorf("read back parquet for %s: %v", tl.name, err)
			continue
		}
		if n != counts[tl.name] {
			t.Errorf("parquet %s rows = %d, duckdb table = %d", tl.name, n, counts[tl.name])
		}
	}
}

func TestRebuildParquetOverwritesStaleFiles(t *testing.T) {
	cfg := testConfig(t)
	buildFeedsGraphDB(t, cfg)

	if _, err := Rebuild(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}
	stale := filepath.Join(cfg.Canon.ParquetDir(), "run.parquet")
	if err := os.WriteFile(stale, []byte("not parquet"), 0o644); err != nil {
		t.Fatal(err)
	}
	counts, err := Rebuild(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	ddb := openSidecar(t, cfg)
	var n int64
	q := fmt.Sprintf(`SELECT COUNT(*) FROM read_parquet('%s')`, filepath.ToSlash(stale))
	if err := ddb.QueryRow(q).Scan(&n); err != nil {
		t.Fatalf("reread stale parquet: %v", err)
	}
	if n != counts["run"] {
		t.Errorf("run parquet rows = %d; want %d", n, counts["run"])
	}
}

func TestRebuildSkipsParquetWhenDisabled(t *testing.T) {
	cfg := testConfig(t)
	cfg.Canon.Export.Parquet = false
	buildFeedsGraphDB(t, cfg)

	if _, err := Rebuild(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(cfg.Canon.ParquetDir()); !os.IsNotExist(err) {
		t.Errorf("parquet dir state after disabled rebuild: %v", err)
	}
	if _, err := os.Stat(cfg.Canon.DuckDBFile()); err != nil {
		t.Errorf("duckdb file missing after disabled rebuild: %v", err)
	}
}
