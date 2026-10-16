package triangulate

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/lavantien/quotes-manager/internal/canon/model"
	"github.com/lavantien/quotes-manager/internal/canon/norm"
	"github.com/lavantien/quotes-manager/internal/config"
)

var collectionPrefix = map[string]string{
	"an":        "an",
	"dhp":       "dhp",
	"dn":        "dn",
	"iti":       "iti",
	"mn":        "mn",
	"sn":        "sn",
	"snp":       "snp",
	"thag":      "thag",
	"thig":      "thig",
	"ud":        "ud",
	"pli-tv-vi": "pli-tv-",
}

func assignCollection(muid string, cols []string) (string, bool) {
	best := ""
	bestLen := -1
	for _, c := range cols {
		p, known := collectionPrefix[c]
		if !known || !strings.HasPrefix(muid, p) || len(p) <= bestLen {
			continue
		}
		best, bestLen = c, len(p)
	}
	return best, best != ""
}

func editionKey(author, collection string) string {
	return author + "/" + collection
}

var dateLayouts = []string{
	"2006-01-02",
	time.RFC3339,
	"2006-01-02T15:04:05",
	"2006-01-02 15:04:05",
}

func parseDateTolerant(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	for _, l := range dateLayouts {
		if t, err := time.Parse(l, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unparsable date %q", s)
}

func escapeLike(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	return strings.ReplaceAll(s, `_`, `\_`)
}

func readDirNames(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read tex parsed dir: %w", err)
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".txt") {
			continue
		}
		out = append(out, strings.TrimSuffix(e.Name(), ".txt"))
	}
	sort.Strings(out)
	return out, nil
}

func runE101(ctx context.Context, cfg *config.Config, st Store, env Env) ([]model.Discrepancy, error) {
	htmlSegs, err := loadHTMLSegments(env.HtmlSegmentsFile)
	if err != nil {
		return nil, fmt.Errorf("e101: %w", err)
	}
	if len(htmlSegs) > 0 {
		return e101HTML(ctx, cfg, st, htmlSegs)
	}

	absent, err := env.texParsedAbsent()
	if err != nil {
		return nil, fmt.Errorf("e101: %w", err)
	}
	if absent {
		detail := "html stream absent and tex parsed dir not configured; editions checks degraded (tex fallback)"
		if env.TexParsedDir != "" {
			detail = "html stream absent and tex parsed dir missing or empty: " + env.TexParsedDir + "; editions checks degraded (tex fallback)"
		}
		return []model.Discrepancy{{
			Check:    model.CheckEditionsCoverage,
			Severity: directionSeverity(cfg, "E101.degraded", model.Info),
			Detail:   detail,
		}}, nil
	}
	texMuids, err := readDirNames(env.TexParsedDir)
	if err != nil {
		return nil, fmt.Errorf("e101: %w", err)
	}
	texSet := map[string]bool{}
	for _, m := range texMuids {
		texSet[m] = true
	}

	sev := model.Severity(cfg.Canon.SeverityFor(string(model.CheckEditionsCoverage)))
	var ds []model.Discrepancy
	bilaraAll := map[string]bool{}
	for _, author := range sortedAuthors(cfg) {
		files, err := authorFileMuids(ctx, st.DB(), author)
		if err != nil {
			return nil, fmt.Errorf("e101: %w", err)
		}
		for _, fileMuid := range files {
			bilaraAll[fileMuid] = true
			col, ok := assignCollection(fileMuid, cfg.Canon.Editions.Authors[author])
			if !ok {
				continue
			}
			if !texSet[fileMuid] {
				ds = append(ds, model.Discrepancy{
					Check:    model.CheckEditionsCoverage,
					Severity: sev,
					Muid:     fileMuid,
					Detail:   "no tex for " + col + ": " + fileMuid,
				})
			}
		}
	}
	for _, muid := range texMuids {
		if !bilaraAll[muid] {
			ds = append(ds, model.Discrepancy{
				Check:    model.CheckEditionsCoverage,
				Severity: sev,
				Muid:     muid,
				Detail:   "tex muid without bilara text: " + muid,
			})
		}
	}
	return ds, nil
}

func e101HTML(ctx context.Context, cfg *config.Config, st Store, htmlSegs map[string]map[string][]model.HTMLSegment) ([]model.Discrepancy, error) {
	sev := model.Severity(cfg.Canon.SeverityFor(string(model.CheckEditionsCoverage)))
	var ds []model.Discrepancy
	for _, author := range sortedAuthors(cfg) {
		texts, err := authorTextMuids(ctx, st.DB(), author)
		if err != nil {
			return nil, fmt.Errorf("e101: %w", err)
		}
		textSet := map[string]bool{}
		for _, m := range texts {
			textSet[m] = true
		}
		for _, muid := range texts {
			col, ok := assignCollection(muid, cfg.Canon.Editions.Authors[author])
			if !ok {
				continue
			}
			if len(htmlSegs[author][muid]) == 0 {
				ds = append(ds, model.Discrepancy{
					Check:    model.CheckEditionsCoverage,
					Severity: sev,
					Muid:     muid,
					Detail:   "no html segment for " + col + ": " + muid,
				})
			}
		}
		for _, muid := range sortedStrings(mapKeys(htmlSegs[author])) {
			if !textSet[muid] {
				ds = append(ds, model.Discrepancy{
					Check:    model.CheckEditionsCoverage,
					Severity: sev,
					Muid:     muid,
					Detail:   "html muid without bilara text: " + muid,
				})
			}
		}
	}
	return ds, nil
}

func mapKeys(m map[string][]model.HTMLSegment) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func sortedAuthors(cfg *config.Config) []string {
	out := make([]string, 0, len(cfg.Canon.Editions.Authors))
	for a := range cfg.Canon.Editions.Authors {
		out = append(out, a)
	}
	sort.Strings(out)
	return out
}

func authorFileMuids(ctx context.Context, db *sql.DB, author string) ([]string, error) {
	prefix := prefixEn + author + ":"
	rows, err := db.QueryContext(ctx, `
		SELECT DISTINCT json_extract(t.data, '$.file_muid') AS file_muid
		FROM node t
		JOIN edge ch ON ch.dst = t.id AND ch.type = 'CHILD_OF'
		JOIN node seg ON seg.id = ch.src
		WHERE t.type = 'text' AND seg.type = 'segment'
		  AND seg.key LIKE ? ESCAPE '\' AND substr(seg.key, 1, ?) = ?
		ORDER BY file_muid`, prefixEn+escapeLike(author)+":%", len(prefix), prefix)
	if err != nil {
		return nil, fmt.Errorf("author file muids query %s: %w", author, err)
	}
	defer func() { _ = rows.Close() }()
	var out []string
	for rows.Next() {
		var muid sql.NullString
		if err := rows.Scan(&muid); err != nil {
			return nil, fmt.Errorf("author file muids scan: %w", err)
		}
		if muid.Valid {
			out = append(out, muid.String)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("author file muids iterate: %w", err)
	}
	return out, nil
}

func authorTextMuids(ctx context.Context, db *sql.DB, author string) ([]string, error) {
	prefix := prefixEn + author + ":"
	rows, err := db.QueryContext(ctx, `
		SELECT DISTINCT t.key
		FROM node t
		JOIN edge ch ON ch.dst = t.id AND ch.type = 'CHILD_OF'
		JOIN node seg ON seg.id = ch.src
		WHERE t.type = 'text' AND seg.type = 'segment'
		  AND seg.key LIKE ? ESCAPE '\' AND substr(seg.key, 1, ?) = ?
		ORDER BY t.key`, prefixEn+escapeLike(author)+":%", len(prefix), prefix)
	if err != nil {
		return nil, fmt.Errorf("author text muids query %s: %w", author, err)
	}
	defer func() { _ = rows.Close() }()
	var out []string
	for rows.Next() {
		var muid string
		if err := rows.Scan(&muid); err != nil {
			return nil, fmt.Errorf("author text muids scan: %w", err)
		}
		out = append(out, muid)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("author text muids iterate: %w", err)
	}
	return out, nil
}

func fileTexts(ctx context.Context, db *sql.DB, fileMuid string) ([]string, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT t.key FROM node t
		WHERE t.type = 'text' AND json_extract(t.data, '$.file_muid') = ?
		ORDER BY json_extract(t.data, '$.file_ord'), t.key`, fileMuid)
	if err != nil {
		return nil, fmt.Errorf("file texts query %s: %w", fileMuid, err)
	}
	defer func() { _ = rows.Close() }()
	var out []string
	for rows.Next() {
		var muid string
		if err := rows.Scan(&muid); err != nil {
			return nil, fmt.Errorf("file texts scan: %w", err)
		}
		out = append(out, muid)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("file texts iterate: %w", err)
	}
	return out, nil
}

type coversData struct {
	Similarity float64 `json:"similarity"`
	Source     string  `json:"source"`
}

func runE102(ctx context.Context, cfg *config.Config, st Store, env Env) ([]model.Discrepancy, error) {
	absent, err := env.texParsedAbsent()
	if err != nil {
		return nil, fmt.Errorf("e102: %w", err)
	}
	htmlSegs, err := loadHTMLSegments(env.HtmlSegmentsFile)
	if err != nil {
		return nil, fmt.Errorf("e102: %w", err)
	}
	if len(env.EditionsZips) == 0 {
		return nil, nil
	}
	comparable := map[string]bool{}
	for _, z := range env.EditionsZips {
		if _, err := parseDateTolerant(z.BuildDate); err == nil {
			comparable[editionKey(z.Author, z.Collection)] = true
		}
	}
	if len(comparable) == 0 {
		return nil, nil
	}
	texSet := map[string]bool{}
	if !absent {
		texMuids, err := readDirNames(env.TexParsedDir)
		if err != nil {
			return nil, fmt.Errorf("e102: %w", err)
		}
		for _, m := range texMuids {
			texSet[m] = true
		}
	}

	sev := model.Severity(cfg.Canon.SeverityFor(string(model.CheckEditionsContent)))
	var ds []model.Discrepancy
	for _, author := range sortedAuthors(cfg) {
		files, err := authorFileMuids(ctx, st.DB(), author)
		if err != nil {
			return nil, fmt.Errorf("e102: %w", err)
		}
		authorHTML := htmlSegs[author]
		for _, col := range sortedStrings(cfg.Canon.Editions.Authors[author]) {
			if !comparable[editionKey(author, col)] {
				continue
			}
			for _, fileMuid := range files {
				if c, ok := assignCollection(fileMuid, cfg.Canon.Editions.Authors[author]); !ok || c != col {
					continue
				}
				texts, err := fileTexts(ctx, st.DB(), fileMuid)
				if err != nil {
					return nil, fmt.Errorf("e102: %w", err)
				}
				if hasHTMLSegments(authorHTML, texts) {
					findings, err := compareHTMLSegments(ctx, st.DB(), author, fileMuid, texts, authorHTML, cfg)
					if err != nil {
						return nil, fmt.Errorf("e102: %w", err)
					}
					if len(findings) == 0 {
						data, err := json.Marshal(coversData{Similarity: 1, Source: "html " + editionKey(author, col)})
						if err != nil {
							return nil, fmt.Errorf("e102 marshal covers: %w", err)
						}
						for _, muid := range texts {
							if err := st.SetCoversEdge(ctx, muid, editionKey(author, col), string(data)); err != nil {
								return nil, fmt.Errorf("e102 set covers %s: %w", muid, err)
							}
						}
						continue
					}
					ds = append(ds, findings...)
					continue
				}
				if !texSet[fileMuid] {
					continue
				}
				segs, err := enSegmentTexts(ctx, st.DB(), author, fileMuid)
				if err != nil {
					return nil, fmt.Errorf("e102: %w", err)
				}
				texPath := filepath.Join(env.TexParsedDir, fileMuid+".txt")
				texBytes, err := os.ReadFile(texPath)
				if err != nil {
					return nil, fmt.Errorf("e102 read tex %s: %w", texPath, err)
				}
				texFolded := norm.Fold(string(texBytes))
				enFolded := make([]string, len(segs))
				bilaraCounts := map[string]int{}
				for i, s := range segs {
					enFolded[i] = norm.Fold(s)
					addTrigramCounts(bilaraCounts, enFolded[i])
				}
				enTrim := strings.TrimSpace(strings.Join(enFolded, " "))
				if enTrim == "" && texFolded == "" {
					ds = append(ds, model.Discrepancy{
						Check:    model.CheckEditionsContent,
						Severity: sev,
						Muid:     fileMuid,
						Detail:   "empty bilara and tex text: " + editionKey(author, col),
					})
					continue
				}
				sim := countsJaccard(bilaraCounts, trigramCounts(texFolded))
				if sim >= cfg.Canon.Triangulate.SimilarityMin {
					data, err := json.Marshal(coversData{Similarity: round4(sim), Source: texPath})
					if err != nil {
						return nil, fmt.Errorf("e102 marshal covers: %w", err)
					}
					for _, muid := range texts {
						if err := st.SetCoversEdge(ctx, muid, editionKey(author, col), string(data)); err != nil {
							return nil, fmt.Errorf("e102 set covers %s: %w", muid, err)
						}
					}
					continue
				}
				ds = append(ds, model.Discrepancy{
					Check:    model.CheckEditionsContent,
					Severity: sev,
					Muid:     fileMuid,
					Detail: divergenceDetail(enTrim, texFolded, sim,
						cfg.Canon.Triangulate.SimilarityMin, cfg.Canon.Triangulate.DetailSampleMax),
				})
			}
		}
	}
	return ds, nil
}

func loadHTMLSegments(path string) (map[string]map[string][]model.HTMLSegment, error) {
	if path == "" {
		return nil, nil
	}
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("open html segments %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()
	out := map[string]map[string][]model.HTMLSegment{}
	r := bufio.NewReader(f)
	for {
		line, err := r.ReadString('\n')
		if line != "" {
			var rec model.HTMLSegment
			if jerr := json.Unmarshal([]byte(strings.TrimRight(line, "\r\n")), &rec); jerr != nil {
				return nil, fmt.Errorf("bad html segment line: %w", jerr)
			}
			if rec.Author == "" || rec.Muid == "" || rec.SegKey == "" {
				continue
			}
			if out[rec.Author] == nil {
				out[rec.Author] = map[string][]model.HTMLSegment{}
			}
			out[rec.Author][rec.Muid] = append(out[rec.Author][rec.Muid], rec)
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				return out, nil
			}
			return nil, fmt.Errorf("read html segments: %w", err)
		}
	}
}

func hasHTMLSegments(authorHTML map[string][]model.HTMLSegment, texts []string) bool {
	for _, muid := range texts {
		if len(authorHTML[muid]) > 0 {
			return true
		}
	}
	return false
}

func isHeadingSegKey(segkey string) bool {
	return strings.HasPrefix(segkey, "0.")
}

type enSegRow struct {
	SegKey string
	Text   string
	Markup string
}

func htmlMarkupRows(ctx context.Context, db *sql.DB, muid string) (map[string]string, error) {
	prefix := prefixHTML + muid + ":"
	rows, err := db.QueryContext(ctx, `
		SELECT key, json_extract(data, '$.text') FROM node
		WHERE type = 'segment' AND key LIKE ? ESCAPE '\'`, escapeLike(prefix)+"%")
	if err != nil {
		return nil, fmt.Errorf("html markup query %s: %w", muid, err)
	}
	defer func() { _ = rows.Close() }()
	out := map[string]string{}
	for rows.Next() {
		var key string
		var markup sql.NullString
		if err := rows.Scan(&key, &markup); err != nil {
			return nil, fmt.Errorf("html markup scan: %w", err)
		}
		out[strings.TrimPrefix(key, prefix)] = markup.String
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("html markup iterate: %w", err)
	}
	return out, nil
}

func isHeadingRow(segkey, markup string) bool {
	return isHeadingSegKey(segkey) || strings.Contains(markup, "<h")
}

func enSegmentRows(ctx context.Context, db *sql.DB, author, muid string) ([]enSegRow, error) {
	prefix := prefixEn + author + ":" + muid + ":"
	rows, err := db.QueryContext(ctx, `
		SELECT en.key, json_extract(en.data, '$.text')
		FROM node en
		JOIN edge che ON che.src = en.id AND che.type = 'CHILD_OF'
		JOIN node t ON t.id = che.dst
		WHERE en.type = 'segment' AND en.key LIKE ? ESCAPE '\' AND t.type = 'text' AND t.key = ?
		ORDER BY che.ord, en.key`, escapeLike(prefix)+"%", muid)
	if err != nil {
		return nil, fmt.Errorf("en segment rows query %s/%s: %w", author, muid, err)
	}
	defer func() { _ = rows.Close() }()
	var out []enSegRow
	for rows.Next() {
		var key string
		var text sql.NullString
		if err := rows.Scan(&key, &text); err != nil {
			return nil, fmt.Errorf("en segment rows scan: %w", err)
		}
		out = append(out, enSegRow{SegKey: strings.TrimPrefix(key, prefix), Text: text.String})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("en segment rows iterate: %w", err)
	}
	return out, nil
}

func compareHTMLSegments(ctx context.Context, db *sql.DB, author, fileMuid string, texts []string, authorHTML map[string][]model.HTMLSegment, cfg *config.Config) ([]model.Discrepancy, error) {
	var samples []string
	n := 0
	for _, muid := range texts {
		rows, err := enSegmentRows(ctx, db, author, muid)
		if err != nil {
			return nil, err
		}
		markup, err := htmlMarkupRows(ctx, db, muid)
		if err != nil {
			return nil, err
		}
		var brows []enSegRow
		for _, r := range rows {
			if isHeadingRow(r.SegKey, markup[r.SegKey]) || norm.Fold(r.Text) == "" {
				continue
			}
			brows = append(brows, r)
		}
		folded := make([]string, len(brows))
		for i, r := range brows {
			folded[i] = norm.Fold(r.Text)
		}
		cur := 0
		for _, h := range authorHTML[muid] {
			want := norm.Fold(h.Text)
			if want == "" || isHeadingSegKey(h.SegKey) {
				continue
			}
			start := cur
			acc := ""
			matched := false
			for cur < len(brows) {
				if acc != "" {
					acc += " "
				}
				acc += folded[cur]
				cur++
				if acc == want {
					matched = true
					break
				}
				if len(acc) > len(want) {
					break
				}
			}
			if matched {
				continue
			}
			cur = start
			n++
			samples = append(samples, htmlMismatchSample(muid, h.SegKey, acc, want,
				cfg.Canon.Triangulate.DetailSampleMax))
		}
		for _, r := range brows[cur:] {
			n++
			samples = append(samples, muid+":"+r.SegKey+" no html segment")
		}
	}
	if n == 0 {
		return nil, nil
	}
	return []model.Discrepancy{{
		Check:    model.CheckEditionsContent,
		Severity: model.Severity(cfg.Canon.SeverityFor(string(model.CheckEditionsContent))),
		Muid:     fileMuid,
		Detail:   fmt.Sprintf("html segments diverge: %d findings: %s", n, capJoin(samples, cfg.Canon.Triangulate.DetailSampleMax)),
	}}, nil
}

func htmlMismatchSample(muid, segkey, bilara, html string, maxChars int) string {
	quarter := maxChars / 4
	rb, rh := []rune(bilara), []rune(html)
	if len(rb) > quarter {
		rb = append(rb[:quarter], []rune("...")...)
	}
	if len(rh) > quarter {
		rh = append(rh[:quarter], []rune("...")...)
	}
	return fmt.Sprintf("%s:%s a=%q b=%q", muid, segkey, string(rb), string(rh))
}

func sortedStrings(in []string) []string {
	out := append([]string(nil), in...)
	sort.Strings(out)
	return out
}

func enSegmentTexts(ctx context.Context, db *sql.DB, author, fileMuid string) ([]string, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT json_extract(en.data, '$.text') AS text
		FROM node en
		JOIN edge che ON che.src = en.id AND che.type = 'CHILD_OF'
		JOIN node t ON t.id = che.dst
		WHERE en.type = 'segment' AND en.key LIKE ? ESCAPE '\'
		  AND t.type = 'text' AND json_extract(t.data, '$.file_muid') = ?
		ORDER BY json_extract(t.data, '$.file_ord'), che.ord, en.key`,
		prefixEn+escapeLike(author)+":%", fileMuid)
	if err != nil {
		return nil, fmt.Errorf("en segments query %s/%s: %w", author, fileMuid, err)
	}
	defer func() { _ = rows.Close() }()
	var out []string
	for rows.Next() {
		var text sql.NullString
		if err := rows.Scan(&text); err != nil {
			return nil, fmt.Errorf("en segments scan: %w", err)
		}
		if text.Valid {
			out = append(out, text.String)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("en segments iterate: %w", err)
	}
	return out, nil
}

func trigramJaccard(a, b string) float64 {
	return countsJaccard(trigramCounts(a), trigramCounts(b))
}

func countsJaccard(ta, tb map[string]int) float64 {
	if len(ta) == 0 && len(tb) == 0 {
		return 1
	}
	var inter, union float64
	for k, ca := range ta {
		cb := tb[k]
		inter += float64(min(ca, cb))
		union += float64(max(ca, cb))
	}
	for k, cb := range tb {
		if _, ok := ta[k]; !ok {
			union += float64(cb)
		}
	}
	if union == 0 {
		return 0
	}
	return inter / union
}

func trigramCounts(s string) map[string]int {
	rs := []rune(s)
	out := make(map[string]int, len(rs))
	addRunTrigrams(out, rs)
	return out
}

func addTrigramCounts(dst map[string]int, s string) {
	addRunTrigrams(dst, []rune(s))
}

func addRunTrigrams(dst map[string]int, rs []rune) {
	for i := 0; i+3 <= len(rs); i++ {
		dst[string(rs[i:i+3])]++
	}
}

func round4(f float64) float64 {
	return math.Round(f*10000) / 10000
}

func divergenceDetail(a, b string, sim, minimum float64, maxChars int) string {
	base := fmt.Sprintf("similarity %.4f < %.4f", sim, minimum)
	ra, rb := []rune(a), []rune(b)
	off := firstDiffRune(ra, rb)
	if off < 0 {
		return base + "; texts are equal"
	}
	half := maxChars / 2
	pre := 20
	if pre > half {
		pre = half
	}
	start := off - pre
	if start < 0 {
		start = 0
	}
	sa := string(ra[start:min(len(ra), start+half)])
	sb := string(rb[start:min(len(rb), start+half)])
	return fmt.Sprintf("%s; first divergence at rune %d: a=%q b=%q", base, off, sa, sb)
}

func firstDiffRune(a, b []rune) int {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		if a[i] != b[i] {
			return i
		}
	}
	if len(a) != len(b) {
		return n
	}
	return -1
}

func runE103(ctx context.Context, cfg *config.Config, st Store, env Env) ([]model.Discrepancy, error) {
	if env.BilaraNewerThan == nil || len(env.EditionsZips) == 0 {
		return nil, nil
	}
	sev := model.Severity(cfg.Canon.SeverityFor(string(model.CheckEditionsStaleness)))
	var ds []model.Discrepancy
	zips := newestPerEdition(env.EditionsZips)
	for _, author := range sortedAuthors(cfg) {
		latest, ok := env.BilaraNewerThan(author)
		if !ok {
			continue
		}
		lt, err := parseDateTolerant(latest)
		if err != nil {
			ds = append(ds, model.Discrepancy{
				Check:    model.CheckEditionsStaleness,
				Severity: model.Info,
				Detail:   fmt.Sprintf("bilara date unparsable for %s: %s", author, latest),
			})
			continue
		}
		for _, z := range zips {
			if z.Author != author {
				continue
			}
			bt, err := parseDateTolerant(z.BuildDate)
			if err != nil {
				ds = append(ds, model.Discrepancy{
					Check:    model.CheckEditionsStaleness,
					Severity: model.Info,
					Detail: fmt.Sprintf("zip %s build date unparsable: %s",
						editionKey(z.Author, z.Collection), z.BuildDate),
				})
				continue
			}
			if lt.After(bt) {
				name := z.ZipName
				if name == "" {
					name = editionKey(z.Author, z.Collection)
				}
				ds = append(ds, model.Discrepancy{
					Check:    model.CheckEditionsStaleness,
					Severity: sev,
					Detail: fmt.Sprintf("bilara newer than %s built %s: latest %s",
						name, z.BuildDate, latest),
				})
			}
		}
	}
	return ds, nil
}

func newestPerEdition(zips []ZipRef) []ZipRef {
	best := map[string]ZipRef{}
	for _, z := range zips {
		k := editionKey(z.Author, z.Collection)
		if cur, ok := best[k]; !ok || z.BuildDate > cur.BuildDate ||
			(z.BuildDate == cur.BuildDate && z.ZipName > cur.ZipName) {
			best[k] = z
		}
	}
	out := make([]ZipRef, 0, len(best))
	for _, k := range sortedKeys2(best) {
		out = append(out, best[k])
	}
	return out
}

func sortedKeys2(m map[string]ZipRef) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func sortedZips(in []ZipRef) []ZipRef {
	out := append([]ZipRef(nil), in...)
	sort.Slice(out, func(i, j int) bool {
		if out[i].Author != out[j].Author {
			return out[i].Author < out[j].Author
		}
		if out[i].Collection != out[j].Collection {
			return out[i].Collection < out[j].Collection
		}
		return out[i].BuildDate < out[j].BuildDate
	})
	return out
}

func runE104(ctx context.Context, cfg *config.Config, st Store, env Env) ([]model.Discrepancy, error) {
	if env.LastRunDate == "" || len(env.EditionsZips) == 0 {
		return nil, nil
	}
	lr, err := parseDateTolerant(env.LastRunDate)
	if err != nil {
		return []model.Discrepancy{{
			Check:    model.CheckEditionsLastRun,
			Severity: model.Info,
			Detail:   fmt.Sprintf("last run date unparsable: %s", env.LastRunDate),
		}}, nil
	}
	var newest time.Time
	var newestRaw string
	for _, z := range sortedZips(env.EditionsZips) {
		bt, err := parseDateTolerant(z.BuildDate)
		if err != nil {
			continue
		}
		if newestRaw == "" || bt.After(newest) {
			newest, newestRaw = bt, z.BuildDate
		}
	}
	if newestRaw == "" {
		return nil, nil
	}
	if sameDay(lr, newest) {
		return nil, nil
	}
	return []model.Discrepancy{{
		Check:    model.CheckEditionsLastRun,
		Severity: model.Severity(cfg.Canon.SeverityFor(string(model.CheckEditionsLastRun))),
		Detail:   fmt.Sprintf("last run %s != newest edition build %s", env.LastRunDate, newestRaw),
	}}, nil
}

func sameDay(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}
