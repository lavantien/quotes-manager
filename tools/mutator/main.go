package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var flips = map[string]string{
	"==": "!=", "!=": "==",
	"<": ">=", ">=": "<",
	">": "<=", "<=": ">",
	"&&": "||", "||": "&&",
	"+": "-", "-": "+",
}

const maxParallelSlots = 8

type mutant struct {
	file   string
	offset int
	oldOp  string
	newOp  string
	line   int
}

func main() {
	pkgs := flag.String("packages", "", "comma-separated package dirs, e.g. internal/store")
	tags := flag.String("tags", "", "build tags forwarded to go test")
	parallel := flag.Int("parallel", 0, "concurrent mutant slots; 0 = min(8, GOMAXPROCS), capped at 8")
	flag.Parse()
	if *pkgs == "" {
		fmt.Fprintln(os.Stderr, "-packages required")
		os.Exit(2)
	}
	if err := capSelfMemory(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	dirs := strings.Split(*pkgs, ",")
	if err := run(dirs, *tags, *parallel); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func clampParallel(requested, mutantCount, procs int) int {
	w := requested
	if w <= 0 {
		w = procs
	}
	if w > maxParallelSlots {
		w = maxParallelSlots
	}
	if w < 1 {
		w = 1
	}
	if mutantCount > 0 && w > mutantCount {
		w = mutantCount
	}
	return w
}

func run(pkgDirs []string, tags string, parallel int) error {
	mutants, err := discover(pkgDirs)
	if err != nil {
		return err
	}
	if len(mutants) == 0 {
		return fmt.Errorf("no mutation points found in %v", pkgDirs)
	}
	workers := clampParallel(parallel, len(mutants), runtime.GOMAXPROCS(0))

	shadows := make([]string, workers)
	for i := range shadows {
		shadows[i], err = copyModule()
		if err != nil {
			cleanupShadows(shadows[:i])
			return err
		}
	}
	defer cleanupShadows(shadows)

	results := make([]testOutcome, len(mutants))
	jobs := make(chan int, len(mutants))
	for i := range mutants {
		jobs <- i
	}
	close(jobs)

	var done atomic.Int64
	var wg sync.WaitGroup
	for _, shadow := range shadows {
		wg.Add(1)
		go func(shadow string) {
			defer wg.Done()
			for i := range jobs {
				results[i] = runMutant(shadow, mutants[i], tags)
				if n := done.Add(1); n%10 == 0 {
					fmt.Printf("progress %d/%d\n", n, len(mutants))
				}
			}
		}(shadow)
	}
	wg.Wait()

	killed, invalid, survived := 0, 0, 0
	for i, m := range mutants {
		switch results[i] {
		case outcomeKilled:
			killed++
		case outcomeInvalid:
			invalid++
		default:
			survived++
			fmt.Printf("SURVIVED %s:%d %s -> %s\n", m.file, m.line, m.oldOp, m.newOp)
		}
	}
	fmt.Printf("slots=%d mutants=%d killed=%d invalid=%d survived=%d\n",
		workers, len(mutants), killed, invalid, survived)
	if survived > 0 {
		return fmt.Errorf("%d mutant(s) survived", survived)
	}
	return nil
}

func runMutant(shadow string, m mutant, tags string) testOutcome {
	dst := filepath.Join(shadow, m.file)
	orig, err := os.ReadFile(dst)
	if err != nil {
		return outcomeInvalid
	}
	mutated := replaceAt(orig, m.offset, m.oldOp, m.newOp)
	if mutated == nil {
		return outcomeInvalid
	}
	if err := os.WriteFile(dst, mutated, 0o644); err != nil {
		return outcomeInvalid
	}
	outcome := testPkg(shadow, pkgFor(m.file), tags)
	_ = os.WriteFile(dst, orig, 0o644)
	return outcome
}

func cleanupShadows(shadows []string) {
	for _, s := range shadows {
		if s != "" {
			_ = os.RemoveAll(s)
		}
	}
}

func discover(pkgDirs []string) ([]mutant, error) {
	var out []mutant
	for _, dir := range pkgDirs {
		dir = strings.TrimSpace(dir)
		if dir == "" {
			continue
		}
		err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			ms, err := discoverFile(path)
			if err != nil {
				return err
			}
			out = append(out, ms...)
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

func discoverFile(path string) ([]mutant, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, src, 0)
	if err != nil {
		return nil, err
	}
	var out []mutant
	ast.Inspect(f, func(n ast.Node) bool {
		be, ok := n.(*ast.BinaryExpr)
		if !ok {
			return true
		}
		newOp, ok := flips[be.Op.String()]
		if !ok {
			return true
		}
		pos := fset.Position(be.OpPos)
		out = append(out, mutant{
			file:   filepath.ToSlash(path),
			offset: fset.File(be.OpPos).Offset(be.OpPos),
			oldOp:  be.Op.String(),
			newOp:  newOp,
			line:   pos.Line,
		})
		return true
	})
	return out, nil
}

func replaceAt(b []byte, offset int, oldOp, newOp string) []byte {
	if offset < 0 || offset+len(oldOp) > len(b) || string(b[offset:offset+len(oldOp)]) != oldOp {
		return nil
	}
	out := make([]byte, 0, len(b)+len(newOp)-len(oldOp))
	out = append(out, b[:offset]...)
	out = append(out, newOp...)
	out = append(out, b[offset+len(oldOp):]...)
	return out
}

type testOutcome int

const (
	outcomeKilled testOutcome = iota
	outcomeSurvived
	outcomeInvalid
)

const testTimeout = 3 * time.Minute

func testPkg(moduleRoot, pkg, tags string) testOutcome {
	return testPkgTimeout(moduleRoot, pkg, tags, testTimeout)
}

func testPkgTimeout(moduleRoot, pkg, tags string, timeout time.Duration) testOutcome {
	args := []string{"test", "-run", "^Test"}
	if tags != "" {
		args = append(args, "-tags", tags)
	}
	args = append(args, pkg)
	ctx, cancelCtx := context.WithTimeout(context.Background(), timeout)
	defer cancelCtx()
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = moduleRoot
	cmd.Env = os.Environ()
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	release, err := attachKillTree(cmd)
	if err != nil {
		return outcomeInvalid
	}
	cmd.Cancel = release
	cmd.WaitDelay = 10 * time.Second
	err = cmd.Run()
	if err != nil {
		if strings.Contains(buf.String(), "[build failed]") {
			return outcomeInvalid
		}
		return outcomeKilled
	}
	return outcomeSurvived
}

func pkgFor(relFile string) string {
	dir := filepath.ToSlash(filepath.Dir(relFile))
	if dir == "." {
		return "."
	}
	return "github.com/lavantien/quotes-manager/" + dir
}

func copyModule() (string, error) {
	tmp, err := os.MkdirTemp("", "mutate-")
	if err != nil {
		return "", err
	}
	skip := map[string]bool{"data": true, "dist": true, ".git": true, "docs": true, "playground": true}
	err = filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if d.IsDir() {
			if skip[name] && path != "." {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasPrefix(name, ".") {
			return nil
		}
		rel, err := filepath.Rel(".", path)
		if err != nil {
			return err
		}
		dst := filepath.Join(tmp, rel)
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		return copyFile(path, dst)
	})
	if err != nil {
		_ = os.RemoveAll(tmp)
		return "", err
	}
	return tmp, nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, in)
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	return err
}
