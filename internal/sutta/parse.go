package sutta

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/lavantien/quotes-manager/internal/quote"
)

var (
	headingRe = regexp.MustCompile(`^\*\*([IVXLCDM]+)\.\s*(.+?)\*\*`)
	linkRe    = regexp.MustCompile(`\[([^\]]*)\]\(([^()\s]+)\)`)
)

func ParseFile(name, content string) (Table, error) {
	tab := Table{Name: name}
	seen := make(map[string]bool)
	for i, line := range strings.Split(content, "\n") {
		if m := headingRe.FindStringSubmatchIndex(line); m != nil {
			group := strings.TrimSpace(line[m[4]:m[5]])
			group = strings.TrimSuffix(group, ".")
			if group != "" {
				tab.Groups = append(tab.Groups, Group{Name: group})
				line = line[m[1]:]
			}
		}
		if len(tab.Groups) == 0 {
			continue
		}
		if err := appendLineSuttas(&tab, seen, line, i+1); err != nil {
			return Table{}, err
		}
	}
	return tab, nil
}

func ParseFiles(files []File) ([]Table, error) {
	tables := make([]Table, 0, len(files))
	for _, f := range files {
		tab, err := ParseFile(f.Name, f.Content)
		if err != nil {
			return nil, err
		}
		tables = append(tables, tab)
	}
	return tables, nil
}

func appendLineSuttas(tab *Table, seen map[string]bool, line string, lineNo int) error {
	for _, lm := range linkRe.FindAllStringSubmatch(line, -1) {
		ref, ok, err := parseSuttaURL(lm[2])
		if err != nil {
			return fmt.Errorf("%s line %d: %w", tab.Name, lineNo, err)
		}
		if !ok {
			continue
		}
		g := &tab.Groups[len(tab.Groups)-1]
		for _, muid := range ref.muids {
			if seen[muid] {
				continue
			}
			seen[muid] = true
			display := lm[1]
			if display == "" || ref.expanded {
				display, err = quote.DisplaySuttaID(muid)
				if err != nil {
					return fmt.Errorf("%s line %d: %w", tab.Name, lineNo, err)
				}
			}
			g.Suttas = append(g.Suttas, Sutta{Muid: muid, Display: display, Link: ref.link})
		}
	}
	return nil
}
