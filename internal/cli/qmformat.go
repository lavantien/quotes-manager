package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/lavantien/quotes-manager/internal/canon/graph"
	"github.com/lavantien/quotes-manager/internal/config"
	"github.com/lavantien/quotes-manager/internal/quote"
	"github.com/lavantien/quotes-manager/internal/search"
	"github.com/lavantien/quotes-manager/internal/store"
)

const (
	formatJSON = "json"
	formatQM   = "qm"
)

func resolveQueryFormat(form, def string) (string, error) {
	if form == "" {
		return def, nil
	}
	switch form {
	case formatJSON, formatQM:
		return form, nil
	}
	return "", fmt.Errorf("format %q must be %s or %s", form, formatJSON, formatQM)
}

func quoteFromMuidPairs(pairs []graph.Pair) (*quote.Quote, error) {
	if len(pairs) == 0 {
		return nil, nil
	}
	id, err := quote.DisplaySuttaID(pairs[0].Muid)
	if err != nil {
		return nil, err
	}
	passages := make([]string, len(pairs))
	for i, p := range pairs {
		if p.En != "" {
			passages[i] = p.En
		} else {
			passages[i] = p.Pali
		}
	}
	return quote.New(id, id, passages), nil
}

func quotesFromHits(pairs []graph.Pair) ([]*quote.Quote, error) {
	out := make([]*quote.Quote, 0, len(pairs))
	for _, p := range pairs {
		q, err := quoteFromMuidPairs([]graph.Pair{p})
		if err != nil {
			return nil, err
		}
		out = append(out, q)
	}
	return out, nil
}

func quoteFromRow(row store.Quote) *quote.Quote {
	return quote.New(row.SuttaID, row.Citation, strings.Split(row.BodyText, "\n"))
}

func rowsToQuotes(rows []store.Quote) []*quote.Quote {
	out := make([]*quote.Quote, len(rows))
	for i, r := range rows {
		out[i] = quoteFromRow(r)
	}
	return out
}

func printQM(qs []*quote.Quote) error {
	if len(qs) == 0 {
		return nil
	}
	_, err := fmt.Fprint(stdout, quote.RenderExportFile(qs))
	return err
}

func runQueryQuotes(cfg *config.Config, text, form string, limit int, dbOverride string) error {
	format, err := resolveQueryFormat(form, formatQM)
	if err != nil {
		return err
	}
	dbPath := cfg.Quotes.DBPath
	if dbOverride != "" {
		dbPath = dbOverride
	}
	if _, err := os.Stat(dbPath); err != nil {
		return fmt.Errorf("quotes: database %s does not exist: %w", dbPath, err)
	}
	st, err := store.Open(dbPath)
	if err != nil {
		return fmt.Errorf("quotes: %w", err)
	}
	defer func() { _ = st.Close() }()
	rows, err := st.List()
	if err != nil {
		return err
	}
	rows = search.Filter(rows, search.Parse(text))
	if limit > 0 && len(rows) > limit {
		rows = rows[:limit]
	}
	if format == formatJSON {
		return printJSON(rows)
	}
	return printQM(rowsToQuotes(rows))
}
