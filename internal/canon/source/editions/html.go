package editions

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"golang.org/x/net/html"
	"golang.org/x/text/unicode/norm"
)

const (
	langDir      = "en"
	texZipSuffix = "-tex.zip"
	lastRunFile  = "last_run_date"
	htmlDir      = "html"
)

var dateRe = regexp.MustCompile(`[0-9]{4}-[0-9]{2}-[0-9]{2}`)

type HTMLInfo struct {
	Author     string
	Collection string
	FileName   string
	BuildDate  string
	Path       string
}

func FindHTML(repoRoot string) ([]HTMLInfo, error) {
	enRoot := filepath.Join(repoRoot, langDir)
	var out []HTMLInfo
	err := filepath.WalkDir(enRoot, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.Type().IsRegular() {
			return nil
		}
		rel, err := filepath.Rel(enRoot, p)
		if err != nil {
			return err
		}
		segs := strings.Split(filepath.ToSlash(rel), "/")
		if len(segs) != 4 || segs[2] != htmlDir || !strings.HasSuffix(segs[3], ".html") {
			return nil
		}
		date, err := StemDate(strings.TrimSuffix(segs[3], ".html"))
		if err != nil {
			return fmt.Errorf("html %s: %w", rel, err)
		}
		out = append(out, HTMLInfo{
			Author:     segs[0],
			Collection: segs[1],
			FileName:   segs[3],
			BuildDate:  date,
			Path:       p,
		})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk %s: %w", enRoot, err)
	}
	slices.SortFunc(out, func(a, b HTMLInfo) int { return strings.Compare(a.Path, b.Path) })
	return out, nil
}

var segIDRE = regexp.MustCompile(`^[a-z0-9][a-z0-9.-]*:[^:{}\s]+$`)

var voidElements = map[string]bool{
	"area": true, "base": true, "br": true, "col": true, "embed": true,
	"hr": true, "img": true, "input": true, "link": true, "meta": true,
	"param": true, "source": true, "track": true, "wbr": true,
}

func isNoteref(tok html.Token) bool {
	for _, a := range tok.Attr {
		if a.Key == "role" && a.Val == "doc-noteref" {
			return true
		}
		if a.Key == "epub:type" && a.Val == "noteref" {
			return true
		}
	}
	return false
}

func tokenAttr(tok html.Token, key string) string {
	for _, a := range tok.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func splitSegID(id string) (muid, segkey string, ok bool) {
	if !segIDRE.MatchString(id) {
		return "", "", false
	}
	muid, segkey, _ = strings.Cut(id, ":")
	return muid, segkey, true
}

func ParseHTMLSegments(r io.Reader, yield func(muid, segkey, text string) error) error {
	z := html.NewTokenizer(r)
	depth := 0
	skipDepth := 0
	var curMuid, curSeg string
	var b strings.Builder
	flush := func() error {
		if curMuid == "" {
			return nil
		}
		text := strings.Join(strings.Fields(b.String()), " ")
		if err := yield(curMuid, curSeg, norm.NFC.String(text)); err != nil {
			return err
		}
		curMuid = ""
		b.Reset()
		return nil
	}
	for {
		tt := z.Next()
		switch tt {
		case html.ErrorToken:
			if err := z.Err(); err == io.EOF {
				return flush()
			}
			return z.Err()
		case html.StartTagToken:
			tok := z.Token()
			switch {
			case skipDepth > 0:
				if !voidElements[tok.Data] {
					skipDepth++
				}
			case depth > 0:
				if isNoteref(tok) {
					skipDepth = 1
					continue
				}
				if tok.Data == "br" {
					b.WriteByte(' ')
					continue
				}
				if !voidElements[tok.Data] {
					depth++
				}
			default:
				if muid, seg, ok := splitSegID(tokenAttr(tok, "id")); ok && !voidElements[tok.Data] {
					if err := flush(); err != nil {
						return err
					}
					curMuid, curSeg = muid, seg
					depth = 1
				}
			}
		case html.SelfClosingTagToken:
			tok := z.Token()
			if skipDepth == 0 && depth == 0 {
				if muid, seg, ok := splitSegID(tokenAttr(tok, "id")); ok && !voidElements[tok.Data] {
					if err := yield(muid, seg, ""); err != nil {
						return err
					}
				}
			}
		case html.EndTagToken:
			switch {
			case skipDepth > 0:
				skipDepth--
			case depth > 0:
				depth--
				if depth == 0 {
					if err := flush(); err != nil {
						return err
					}
				}
			}
		case html.TextToken:
			if depth > 0 && skipDepth == 0 {
				b.Write(z.Text())
			}
		}
	}
}

func ParseBuildDate(zipName string) (string, error) {
	if !strings.HasSuffix(zipName, texZipSuffix) {
		return "", fmt.Errorf("zip name %q lacks %s suffix", zipName, texZipSuffix)
	}
	return StemDate(strings.TrimSuffix(zipName, texZipSuffix))
}

func StemDate(stem string) (string, error) {
	date := ""
	for _, loc := range dateRe.FindAllStringIndex(stem, -1) {
		before, after := loc[0]-1, loc[1]
		if before >= 0 && isDigit(stem[before]) {
			continue
		}
		if after < len(stem) && isDigit(stem[after]) {
			continue
		}
		date = stem[loc[0]:loc[1]]
	}
	if date == "" {
		return "", fmt.Errorf("name %q carries no build date", stem)
	}
	if _, err := time.Parse("2006-01-02", date); err != nil {
		return "", fmt.Errorf("name %q: bad date %q", stem, date)
	}
	return date, nil
}

func isDigit(b byte) bool { return b >= '0' && b <= '9' }

func ReadLastRunDate(repoRoot string) (string, error) {
	b, err := os.ReadFile(filepath.Join(repoRoot, lastRunFile))
	if err != nil {
		return "", fmt.Errorf("read %s: %w", lastRunFile, err)
	}
	return strings.TrimSpace(strings.TrimPrefix(string(b), string(rune(0xFEFF)))), nil
}

func LatestCommit(repoDir, pathFilter string) (string, error) {
	git, err := exec.LookPath("git")
	if err != nil {
		return "", fmt.Errorf("git not found: %w", err)
	}
	args := []string{"-C", repoDir, "log", "-1", "--format=%cI"}
	if pathFilter != "" {
		args = append(args, "--", pathFilter)
	}
	out, err := exec.Command(git, args...).Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("git log: %v: %s", err, strings.TrimSpace(string(ee.Stderr)))
		}
		return "", fmt.Errorf("git log: %w", err)
	}
	s := strings.TrimSpace(string(out))
	if s == "" {
		return "", fmt.Errorf("git log: no commit touches %q", pathFilter)
	}
	return s, nil
}

func DiscoveryReport(htmls []HTMLInfo, outPath string) error {
	var b strings.Builder
	fmt.Fprintf(&b, "# editions html discovery report\n\nhtml editions: %d\n\n", len(htmls))
	b.WriteString("## html editions\n\n")
	b.WriteString("| author | collection | build date | file |\n")
	b.WriteString("| --- | --- | --- | --- |\n")
	sorted := slices.Clone(htmls)
	slices.SortFunc(sorted, func(a, b HTMLInfo) int {
		return strings.Compare(a.Author+"/"+a.Collection+"/"+a.FileName,
			b.Author+"/"+b.Collection+"/"+b.FileName)
	})
	for _, h := range sorted {
		fmt.Fprintf(&b, "| %s | %s | %s | %s |\n",
			cell(h.Author), cell(h.Collection), cell(h.BuildDate), cell(h.FileName))
	}
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(outPath, []byte(b.String()), 0o644)
}

func cell(v string) string {
	return strings.NewReplacer("|", `\|`, "\r\n", " ", "\n", " ", "\r", " ").Replace(v)
}
