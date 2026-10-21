package cli

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

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

type suttaRow struct {
	Muid          string `json:"muid"`
	Display       string `json:"display"`
	Link          string `json:"link"`
	Length        int    `json:"length"`
	GroupID       int64  `json:"group_id,omitempty"`
	GroupName     string `json:"group_name,omitempty"`
	GroupPosition int    `json:"group_position,omitempty"`
}

func runQuerySuttas(cfg *config.Config, form, dbOverride string, collection int64) error {
	format, err := resolveQueryFormat(form, formatQM)
	if err != nil {
		return err
	}
	dbPath := cfg.Quotes.DBPath
	if dbOverride != "" {
		dbPath = dbOverride
	}
	if _, err := os.Stat(dbPath); err != nil {
		return fmt.Errorf("suttas: database %s does not exist: %w", dbPath, err)
	}
	st, err := store.Open(dbPath)
	if err != nil {
		return fmt.Errorf("suttas: %w", err)
	}
	defer func() { _ = st.Close() }()

	if collection == 0 {
		suttas, err := st.ListSuttas()
		if err != nil {
			return err
		}
		rows := make([]suttaRow, 0, len(suttas))
		for _, s := range suttas {
			rows = append(rows, suttaRow{Muid: s.Muid, Display: s.Display, Link: s.Link, Length: s.Length})
		}
		return renderSuttas(format, rows)
	}
	if _, err := st.GetCollection(collection); err != nil {
		return fmt.Errorf("suttas: collection %d: %w", collection, err)
	}
	groups, err := st.ListCollectionGroups(collection)
	if err != nil {
		return err
	}
	items, err := st.CollectionSuttaItems(collection)
	if err != nil {
		return err
	}
	byGroup := make(map[int64][]store.SuttaItem, len(groups))
	var ungrouped []store.SuttaItem
	for _, it := range items {
		if it.GroupID == 0 {
			ungrouped = append(ungrouped, it)
		} else {
			byGroup[it.GroupID] = append(byGroup[it.GroupID], it)
		}
	}
	rows := make([]suttaRow, 0, len(items))
	appendBlock := func(block []store.SuttaItem, gid int64, gname string, gpos int) {
		store.SortShortestFirst(block)
		for _, it := range block {
			rows = append(rows, suttaRow{
				Muid: it.Muid, Display: it.Display, Link: it.Link, Length: it.Length,
				GroupID: gid, GroupName: gname, GroupPosition: gpos,
			})
		}
	}
	for gi, g := range groups {
		appendBlock(byGroup[g.ID], g.ID, g.Name, gi+1)
	}
	appendBlock(ungrouped, 0, "", 0)
	return renderSuttas(format, rows)
}

func renderSuttas(format string, rows []suttaRow) error {
	if format == formatJSON {
		return printJSON(rows)
	}
	var b strings.Builder
	for i, r := range rows {
		if i > 0 && rows[i-1].GroupID != r.GroupID {
			b.WriteString("\n")
		}
		b.WriteString(r.Display)
		b.WriteString(" (")
		b.WriteString(r.Link)
		b.WriteString(")")
		if r.Length > 0 {
			b.WriteString(" ")
			b.WriteString(strconv.Itoa(r.Length))
		}
		b.WriteString("\n")
	}
	_, err := fmt.Fprint(stdout, b.String())
	return err
}
