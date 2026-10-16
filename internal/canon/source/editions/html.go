package editions

import (
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/text/unicode/norm"
)

const htmlDir = "html"

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
