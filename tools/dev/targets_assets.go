package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

func assetCommands() []command {
	return []command{
		{"vendor-assets", "verify the vendored htmx and cytoscape pins in web/static, -refresh moves them", runVendorAssets},
		{"diagrams", "render every docs/diagrams/*.puml to a committed PNG beside it", runDiagrams},
	}
}

func runVendorAssets(args []string) error {
	fs := newFlagSet("vendor-assets")
	refresh := fs.Bool("refresh", false, "check the npm registry, move pins, and download")
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	r, err := newDevRunner()
	if err != nil {
		return err
	}
	if *refresh {
		for _, a := range vendoredAssets() {
			if err := refreshAsset(r, a); err != nil {
				return err
			}
		}
	}
	for _, a := range vendoredAssets() {
		path := filepath.Join("web", "static", a.File)
		sum, err := sha256File(path)
		if err != nil {
			return err
		}
		if sum != a.SHA256 {
			return fmt.Errorf("vendor-assets: %s sha mismatch for %s: have %s want %s, run vendor-assets -refresh", a.Pkg, path, sum, a.SHA256)
		}
		fmt.Printf("vendor-assets: %s %s ok\n", a.Pkg, a.Version)
	}
	return nil
}

func refreshAsset(r *devRunner, a vendoredAsset) error {
	newest, err := newestStableTag(a.Pkg)
	if err != nil {
		return err
	}
	if newest == "" || newest == a.Version || compareVersions(newest, a.Version) <= 0 {
		return nil
	}
	url := unpkgBase + "/" + a.Pkg + "@" + newest + "/dist/" + a.File
	body, err := download(url)
	if err != nil {
		return err
	}
	if len(body) == 0 || body[0] == '<' {
		return fmt.Errorf("vendor-assets: %s %s returned html, refusing to move the pin", a.Pkg, url)
	}
	path := filepath.Join("web", "static", a.File)
	if err := os.WriteFile(path, body, 0o644); err != nil {
		return err
	}
	sum := sha256Bytes(body)
	if err := rewritePins(r, map[string]string{a.VerConst: newest, a.SHAConst: sum}); err != nil {
		return err
	}
	fmt.Printf("vendor-assets: %s pin moved %s -> %s (sha256 %s)\n", a.Pkg, a.Version, newest, sum)
	return nil
}

func newestStableTag(pkg string) (string, error) {
	body, err := download(npmRegistry + "/-/package/" + pkg + "/dist-tags")
	if err != nil {
		return "", fmt.Errorf("vendor-assets: %s: registry unreachable: %w", pkg, err)
	}
	var tags map[string]string
	if err := json.Unmarshal(body, &tags); err != nil {
		return "", err
	}
	newest := ""
	for _, v := range tags {
		if !isStableVersion(v) {
			continue
		}
		if newest == "" || compareVersions(v, newest) > 0 {
			newest = v
		}
	}
	return newest, nil
}

func isStableVersion(v string) bool {
	if v == "" || v[0] < '0' || v[0] > '9' {
		return false
	}
	return !strings.ContainsRune(v, '-')
}

func compareVersions(a, b string) int {
	pa := strings.Split(a, ".")
	pb := strings.Split(b, ".")
	n := len(pa)
	if len(pb) > n {
		n = len(pb)
	}
	for i := 0; i < n; i++ {
		sa, sb := "0", "0"
		if i < len(pa) {
			sa = pa[i]
		}
		if i < len(pb) {
			sb = pb[i]
		}
		na, ea := strconv.Atoi(sa)
		nb, eb := strconv.Atoi(sb)
		if ea != nil || eb != nil {
			if c := strings.Compare(sa, sb); c != 0 {
				return c
			}
			continue
		}
		if na != nb {
			if na < nb {
				return -1
			}
			return 1
		}
	}
	return 0
}

func rewritePins(r *devRunner, consts map[string]string) error {
	path := filepath.Join("tools", "dev", "pins.go")
	src, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	text := string(src)
	for name, value := range consts {
		re := regexp.MustCompile(`(` + name + `\s+= ")[^"]*(")`)
		if !re.MatchString(text) {
			return fmt.Errorf("vendor-assets: pin rewrite failed for %s", name)
		}
		text = re.ReplaceAllString(text, "${1}"+value+"${2}")
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		return err
	}
	return r.exec("gofumpt-pins", nil, "gofumpt", "-w", path)
}

func download(url string) ([]byte, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: status %s", url, resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 64<<20))
}

func sha256File(path string) (string, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return sha256Bytes(body), nil
}

func sha256Bytes(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func runDiagrams(args []string) error {
	if err := parseFlags(newFlagSet("diagrams"), args); err != nil {
		return err
	}
	r, err := newDevRunner()
	if err != nil {
		return err
	}
	jar := plantumlJarPath
	sum, err := sha256File(jar)
	if err != nil || sum != plantumlSHA256 {
		if err := os.MkdirAll(filepath.Dir(jar), 0o755); err != nil {
			return err
		}
		body, derr := download(plantumlURL)
		if derr != nil {
			return fmt.Errorf("diagrams: jar missing or corrupt and download failed: %w", derr)
		}
		if err := os.WriteFile(jar, body, 0o644); err != nil {
			return err
		}
	}
	if _, err := r.execLook("java"); err != nil {
		fmt.Fprintln(os.Stderr, "diagrams: java not found; docker fallback: docker run --rm -v $(pwd):/w -w /w/docs/diagrams plantuml/plantuml:"+plantumlVersion+" -tpng '*.puml'")
		return err
	}
	pumls, err := filepath.Glob(filepath.Join("docs", "diagrams", "*.puml"))
	if err != nil {
		return err
	}
	for _, src := range pumls {
		if err := r.exec("diagrams "+filepath.Base(src), nil, "java", "-jar", jar, "-failfast2", "-overwrite", "-tpng", src); err != nil {
			return err
		}
		png := strings.TrimSuffix(src, ".puml") + ".png"
		if _, err := os.Stat(png); err != nil {
			return fmt.Errorf("diagrams: render failed for %s", src)
		}
	}
	return nil
}
