package report

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/lavantien/quotes-manager/internal/canon/model"
)

const samplesPerBucket = 10

const defaultDetailSampleMax = 200

type CoverageRow struct {
	Division string `json:"division"`
	Texts    int    `json:"texts"`
	Covered  int    `json:"covered"`
}

type Input struct {
	Date              string              `json:"date"`
	Provenance        map[string]string   `json:"provenance"`
	Counts            map[string]int64    `json:"counts"`
	Coverage          []CoverageRow       `json:"coverage"`
	Discrepancies     []model.Discrepancy `json:"discrepancies"`
	DiscrepancyCounts map[string]int      `json:"discrepancy_counts"`
	DetailSampleMax   int                 `json:"detail_sample_max"`
}

func RenderMarkdown(in Input) string {
	max := detailMax(in)
	var b strings.Builder
	fmt.Fprintf(&b, "# triangulation report\n\ndate: %s\n", orUnknown(in.Date))

	b.WriteString("\n## provenance\n\n")
	writeSortedPairs(&b, in.Provenance)

	b.WriteString("\n## counts\n\n")
	if countKeys := sortedKeys(in.Counts); len(countKeys) == 0 {
		b.WriteString("- none\n")
	} else {
		for _, k := range countKeys {
			fmt.Fprintf(&b, "- %s: %d\n", k, in.Counts[k])
		}
	}

	b.WriteString("\n## coverage\n\n")
	rows := sortedCoverage(in.Coverage)
	if len(rows) == 0 {
		b.WriteString("none\n")
	} else {
		b.WriteString("| division | texts | covered |\n| --- | --- | --- |\n")
		for _, r := range rows {
			fmt.Fprintf(&b, "| %s | %d | %d |\n", escapePipe(r.Division), r.Texts, r.Covered)
		}
	}

	b.WriteString("\n## discrepancies\n\n")
	keys := sortedKeys(in.DiscrepancyCounts)
	if len(keys) == 0 {
		b.WriteString("none\n")
	} else {
		b.WriteString("| check | count |\n| --- | --- |\n")
		for _, k := range keys {
			fmt.Fprintf(&b, "| %s | %d |\n", k, in.DiscrepancyCounts[k])
		}
	}

	b.WriteString("\n### samples\n\n")
	discs := sortedDiscrepancies(in.Discrepancies)
	if len(discs) == 0 {
		b.WriteString("none\n")
	} else {
		writeSamples(&b, discs, max)
	}

	b.WriteString("\n## staleness\n\n")
	stale := 0
	for k, n := range in.DiscrepancyCounts {
		if strings.HasPrefix(k, string(model.CheckEditionsStaleness)+"/") &&
			!strings.HasSuffix(k, "/"+string(model.Info)) {
			stale += n
		}
	}
	if stale > 0 {
		fmt.Fprintf(&b, "stale editions: %d\n", stale)
	} else {
		b.WriteString("staleness-free\n")
	}
	return b.String()
}

func RenderJSON(in Input) ([]byte, error) {
	out := Input{
		Date:              in.Date,
		Provenance:        in.Provenance,
		Counts:            in.Counts,
		Coverage:          sortedCoverage(in.Coverage),
		Discrepancies:     sortedDiscrepancies(in.Discrepancies),
		DiscrepancyCounts: in.DiscrepancyCounts,
		DetailSampleMax:   detailMax(in),
	}
	b, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("render json: %w", err)
	}
	return append(b, '\n'), nil
}

func detailMax(in Input) int {
	if in.DetailSampleMax <= 0 {
		return defaultDetailSampleMax
	}
	return in.DetailSampleMax
}

func orUnknown(s string) string {
	if s == "" {
		return "unknown"
	}
	return s
}

func writeSortedPairs(b *strings.Builder, m map[string]string) {
	keys := sortedKeys(m)
	if len(keys) == 0 {
		b.WriteString("- none\n")
		return
	}
	for _, k := range keys {
		fmt.Fprintf(b, "- %s: %s\n", k, m[k])
	}
}

func escapePipe(s string) string {
	return strings.ReplaceAll(s, "|", `\|`)
}

func writeSamples(b *strings.Builder, discs []model.Discrepancy, max int) {
	buckets := map[string][]model.Discrepancy{}
	for _, d := range discs {
		k := fmt.Sprintf("%s/%s", d.Check, d.Severity)
		buckets[k] = append(buckets[k], d)
	}
	for i, k := range sortedKeys(buckets) {
		if i > 0 {
			b.WriteString("\n")
		}
		fmt.Fprintf(b, "#### %s\n\n", k)
		for _, d := range buckets[k][:min(len(buckets[k]), samplesPerBucket)] {
			if d.Muid != "" {
				fmt.Fprintf(b, "- %s: %s\n", d.Muid, truncate(d.Detail, max))
			} else {
				fmt.Fprintf(b, "- %s\n", truncate(d.Detail, max))
			}
		}
		if rest := len(buckets[k]) - samplesPerBucket; rest > 0 {
			fmt.Fprintf(b, "- (+%d more)\n", rest)
		}
	}
}

func truncate(s string, max int) string {
	const marker = " [...]"
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	if max <= len(marker) {
		return string(r[:max])
	}
	return string(r[:max-len(marker)]) + marker
}

func sortedCoverage(rows []CoverageRow) []CoverageRow {
	out := append([]CoverageRow(nil), rows...)
	sort.Slice(out, func(i, j int) bool { return out[i].Division < out[j].Division })
	return out
}

func sortedDiscrepancies(ds []model.Discrepancy) []model.Discrepancy {
	out := append([]model.Discrepancy(nil), ds...)
	sort.Slice(out, func(i, j int) bool {
		if out[i].Check != out[j].Check {
			return out[i].Check < out[j].Check
		}
		if out[i].Severity != out[j].Severity {
			return out[i].Severity < out[j].Severity
		}
		if out[i].Muid != out[j].Muid {
			return out[i].Muid < out[j].Muid
		}
		return out[i].Detail < out[j].Detail
	})
	return out
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
