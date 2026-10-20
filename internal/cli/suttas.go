package cli

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/lavantien/quotes-manager/internal/canon/graph"
	"github.com/lavantien/quotes-manager/internal/config"
	"github.com/lavantien/quotes-manager/internal/store"
)

func init() {
	Register(Command{Name: "suttas", Usage: "sutta registry maintenance: --lengths fills lengths from the published canon db", Run: runSuttas})
}

func runSuttas(ctx context.Context, cfg *config.Config, args []string) error {
	fs := flag.NewFlagSet("suttas", flag.ContinueOnError)
	lengths := fs.Bool("lengths", false, "fill suttas.length from the published canon db")
	db := fs.String("db", "", "quotes database path override")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("suttas: unknown arguments %v", fs.Args())
	}
	if !*lengths {
		printLine("usage: qm suttas --lengths [--db file]\n")
		return nil
	}
	dbPath := cfg.Quotes.DBPath
	if *db != "" {
		dbPath = *db
	}
	if _, err := os.Stat(dbPath); err != nil {
		return fmt.Errorf("suttas: database %s does not exist: %w", dbPath, err)
	}
	st, err := store.Open(dbPath)
	if err != nil {
		return fmt.Errorf("suttas: %w", err)
	}
	defer func() { _ = st.Close() }()

	_, graphPath, err := graph.CurrentDB(cfg)
	if err != nil {
		return fmt.Errorf("suttas: no published canon db: %w", err)
	}
	gs, err := graph.Open(graphPath)
	if err != nil {
		return fmt.Errorf("suttas: %w", err)
	}
	defer func() { _ = gs.Close() }()

	registry, err := st.ListSuttas()
	if err != nil {
		return err
	}
	counts := make(map[string]int, len(registry))
	for _, s := range registry {
		pairs, err := gs.Segments(ctx, s.Muid, true, 0, 0)
		if err != nil {
			return fmt.Errorf("suttas: muid %s: %w", s.Muid, err)
		}
		if len(pairs) > 0 {
			counts[s.Muid] = len(pairs)
		}
	}
	if err := st.SetSuttaLengths(counts); err != nil {
		return fmt.Errorf("suttas: %w", err)
	}
	printLine("suttas: %d lengths updated of %d from %s\n", len(counts), len(registry), graphPath)
	return nil
}
