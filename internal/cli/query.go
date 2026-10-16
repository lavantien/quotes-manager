package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"

	"github.com/lavantien/quotes-manager/internal/canon/graph"
	"github.com/lavantien/quotes-manager/internal/config"
)

func init() {
	Register(Command{Name: "query", Usage: "query the current database: segments|search|tree|stats|word", Run: runQuery})
}

func runQuery(ctx context.Context, cfg *config.Config, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("query needs a subcommand: segments, search, tree, stats, or word")
	}
	fs := flag.NewFlagSet("query "+args[0], flag.ContinueOnError)
	muid := fs.String("muid", "", "text muid")
	withEn := fs.Bool("with-en", false, "only segments with a translation")
	text := fs.String("text", "", "search text")
	lang := fs.String("lang", "en", "search column: pali or en")
	limit := fs.Int("limit", 0, "row limit (0 = all)")
	offset := fs.Int("offset", 0, "row offset (0 = none)")
	division := fs.String("division", "", "division key")
	types := fs.String("types", "all", "node types: segment, text, or all")
	form := fs.String("form", "", "word lookup: surface form token")
	headword := fs.String("headword", "", "word lookup: headword lemma")
	withSegments := fs.Bool("segments", false, "with --headword: list the segment refs")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unknown query arguments: %v", fs.Args())
	}
	if *offset < 0 {
		return fmt.Errorf("offset must be >= 0")
	}

	_, dbPath, err := graph.CurrentDB(cfg)
	if err != nil {
		return fmt.Errorf("query: %w", err)
	}
	store, err := graph.Open(dbPath)
	if err != nil {
		return fmt.Errorf("query: %w", err)
	}
	defer func() { _ = store.Close() }()

	switch args[0] {
	case "segments":
		if *muid == "" {
			return fmt.Errorf("segments needs --muid")
		}
		pairs, err := store.Segments(ctx, *muid, *withEn, *limit, *offset)
		if err != nil {
			return err
		}
		return printJSON(pairs)
	case "search":
		if *text == "" {
			return fmt.Errorf("search needs --text")
		}
		pairs, err := store.SearchFTS(ctx, *text, *lang, *limit, *offset)
		if err != nil {
			return err
		}
		return printJSON(pairs)
	case "tree":
		if *division == "" {
			return fmt.Errorf("tree needs --division")
		}
		nodes, err := store.Tree(ctx, *division, *types, *limit, *offset)
		if err != nil {
			return err
		}
		return printJSON(nodes)
	case "stats":
		stats, err := store.Stats(ctx)
		if err != nil {
			return err
		}
		for _, k := range sortedKeys(stats) {
			printLine("%s: %d\n", k, stats[k])
		}
		return nil
	case "word":
		switch {
		case *form != "" && *headword != "":
			return fmt.Errorf("word takes one of --form or --headword, not both")
		case *form != "":
			entries, err := store.ResolveForm(ctx, *form)
			if err != nil {
				return err
			}
			return printJSON(entries)
		case *headword != "":
			n, refs, err := store.HeadwordUsage(ctx, *headword, *withSegments, *limit, *offset)
			if err != nil {
				return err
			}
			return printJSON(struct {
				Headword string   `json:"headword"`
				Segments int64    `json:"segments"`
				Refs     []string `json:"refs,omitempty"`
			}{Headword: *headword, Segments: n, Refs: refs})
		default:
			return fmt.Errorf("word needs --form <token> or --headword <lemma>")
		}
	default:
		return fmt.Errorf("unknown query subcommand %q: want segments, search, tree, stats, or word", args[0])
	}
}

func printJSON(v any) error {
	enc := json.NewEncoder(stdout)
	return enc.Encode(v)
}
