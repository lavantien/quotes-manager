package editions

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
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
)

const (
	langDir      = "en"
	texZipSuffix = "-tex.zip"
	lastRunFile  = "last_run_date"
)

var dateRe = regexp.MustCompile(`[0-9]{4}-[0-9]{2}-[0-9]{2}`)

type ZipInfo struct {
	Author     string
	Collection string
	Variant    string
	ZipName    string
	BuildDate  string
	SHA256     string
	Path       string
	TexCount   int
}

func FindZips(repoRoot string) ([]ZipInfo, error) {
	enRoot := filepath.Join(repoRoot, langDir)
	var out []ZipInfo
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
		if len(segs) != 4 || !strings.HasSuffix(segs[3], texZipSuffix) {
			return nil
		}
		date, err := ParseBuildDate(segs[3])
		if err != nil {
			return fmt.Errorf("zip %s: %w", rel, err)
		}
		sum, err := fileSHA256(p)
		if err != nil {
			return err
		}
		out = append(out, ZipInfo{
			Author:     segs[0],
			Collection: segs[1],
			Variant:    segs[2],
			ZipName:    segs[3],
			BuildDate:  date,
			SHA256:     sum,
			Path:       p,
		})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk %s: %w", enRoot, err)
	}
	slices.SortFunc(out, func(a, b ZipInfo) int { return strings.Compare(a.Path, b.Path) })
	return out, nil
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

func Extract(repoRoot, outRoot string) ([]ZipInfo, error) {
	if outRoot == "" {
		return nil, fmt.Errorf("extract: empty outRoot would write into the working directory")
	}
	zips, err := FindZips(repoRoot)
	if err != nil {
		return nil, err
	}
	for i := range zips {
		z := &zips[i]
		dest := filepath.Join(outRoot, z.Author, z.Collection, z.Variant, strings.TrimSuffix(z.ZipName, ".zip"))
		z.TexCount, err = extractZipFile(z.Path, dest)
		if err != nil {
			return nil, err
		}
	}
	return zips, nil
}

func extractZipFile(zipPath, destRoot string) (int, error) {
	f, err := os.Open(zipPath)
	if err != nil {
		return 0, err
	}
	defer func() { _ = f.Close() }()
	fi, err := f.Stat()
	if err != nil {
		return 0, err
	}
	zr, err := zip.NewReader(f, fi.Size())
	if err != nil {
		return 0, fmt.Errorf("open zip %s: %w", zipPath, err)
	}
	return extractReader(zr, destRoot, zipPath)
}

func extractReader(zr *zip.Reader, destRoot, srcName string) (int, error) {
	n := 0
	for _, zf := range zr.File {
		name := zf.Name
		if zf.FileInfo().IsDir() || strings.HasSuffix(name, "/") {
			continue
		}
		if !strings.HasSuffix(name, ".tex") {
			continue
		}
		segs, err := safeEntryRel(name)
		if err != nil {
			return n, fmt.Errorf("%s: entry %q: %w", srcName, name, err)
		}
		dest := filepath.Join(append([]string{destRoot}, segs...)...)
		if !withinDir(destRoot, dest) {
			return n, fmt.Errorf("%s: entry %q escapes the output root", srcName, name)
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return n, fmt.Errorf("%s: entry %q: %w", srcName, name, err)
		}
		if err := writeZipEntry(zf, dest); err != nil {
			return n, fmt.Errorf("%s: entry %q: %w", srcName, name, err)
		}
		n++
	}
	return n, nil
}

func writeZipEntry(zf *zip.File, dest string) error {
	rc, err := zf.Open()
	if err != nil {
		return err
	}
	defer func() { _ = rc.Close() }()
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, rc); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}

func safeEntryRel(name string) ([]string, error) {
	unsafe := fmt.Errorf("unsafe zip entry path")
	if name == "" || strings.ContainsAny(name, "\\\x00:") {
		return nil, unsafe
	}
	segs := strings.Split(name, "/")
	for _, s := range segs {
		if s == "" || s == "." || s == ".." {
			return nil, unsafe
		}
	}
	return segs, nil
}

func withinDir(root, p string) bool {
	rel, err := filepath.Rel(root, p)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

func fileSHA256(p string) (string, error) {
	b, err := os.ReadFile(p)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

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

func DiscoveryReport(zips []ZipInfo, htmls []HTMLInfo, texRoot, outPath string) error {
	var b strings.Builder
	fmt.Fprintf(&b, "# editions tex discovery report\n\nzips: %d\nhtml editions: %d\n\n", len(zips), len(htmls))
	b.WriteString("## zips\n\n")
	b.WriteString("| author | collection | variant | build date | tex files | sha256 |\n")
	b.WriteString("| --- | --- | --- | --- | --- | --- |\n")
	sorted := slices.Clone(zips)
	slices.SortFunc(sorted, func(a, b ZipInfo) int {
		return strings.Compare(a.Author+"/"+a.Collection+"/"+a.Variant+"/"+a.ZipName,
			b.Author+"/"+b.Collection+"/"+b.Variant+"/"+b.ZipName)
	})
	for _, z := range sorted {
		fmt.Fprintf(&b, "| %s | %s | %s | %s | %d | %s |\n",
			cell(z.Author), cell(z.Collection), cell(z.Variant), cell(z.BuildDate), z.TexCount, z.SHA256)
	}
	b.WriteString("\n## html editions\n\n")
	b.WriteString("| author | collection | build date | file |\n")
	b.WriteString("| --- | --- | --- | --- |\n")
	sortedHTML := slices.Clone(htmls)
	slices.SortFunc(sortedHTML, func(a, b HTMLInfo) int {
		return strings.Compare(a.Author+"/"+a.Collection+"/"+a.FileName,
			b.Author+"/"+b.Collection+"/"+b.FileName)
	})
	for _, h := range sortedHTML {
		fmt.Fprintf(&b, "| %s | %s | %s | %s |\n",
			cell(h.Author), cell(h.Collection), cell(h.BuildDate), cell(h.FileName))
	}
	b.WriteString("\n## extracted tex tree\n\n")
	writeTree(&b, texRoot)
	b.WriteString("\n## sample tex files\n\n")
	writeSamples(&b, texRoot)
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(outPath, []byte(b.String()), 0o644)
}

func cell(v string) string {
	return strings.NewReplacer("|", `\|`, "\r\n", " ", "\n", " ", "\r", " ").Replace(v)
}

func writeTree(b *strings.Builder, texRoot string) {
	if _, err := os.Stat(texRoot); err != nil {
		b.WriteString("- (none)\n")
		return
	}
	_ = filepath.WalkDir(texRoot, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(texRoot, p)
		if err != nil {
			return err
		}
		depth := strings.Count(filepath.ToSlash(rel), "/")
		name := filepath.Base(p)
		if d.IsDir() {
			name += "/"
		}
		fmt.Fprintf(b, "%s- %s\n", strings.Repeat("  ", depth), name)
		return nil
	})
}

func writeSamples(b *strings.Builder, texRoot string) {
	var files []string
	_ = filepath.WalkDir(texRoot, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(p, ".tex") {
			files = append(files, p)
		}
		return nil
	})
	if len(files) == 0 {
		b.WriteString("(none)\n")
		return
	}
	slices.Sort(files)
	for _, f := range files[:min(3, len(files))] {
		rel, err := filepath.Rel(texRoot, f)
		if err != nil {
			continue
		}
		body, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		fmt.Fprintf(b, "### %s\n\n", filepath.ToSlash(rel))
		fence := fenceFor(body)
		fmt.Fprintf(b, "%stex\n%s\n%s\n\n", fence, firstLines(body, 40), fence)
	}
}

func firstLines(body []byte, n int) string {
	trimmed := strings.TrimSuffix(string(body), "\n")
	lines := strings.Split(trimmed, "\n")
	if len(lines) > n {
		lines = lines[:n]
	}
	return strings.Join(lines, "\n")
}

func fenceFor(body []byte) string {
	longest, run := 0, 0
	for _, c := range body {
		if c == '`' {
			run++
			longest = max(longest, run)
		} else {
			run = 0
		}
	}
	return strings.Repeat("`", max(3, longest+1))
}
