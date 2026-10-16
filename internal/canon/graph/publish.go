package graph

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/lavantien/quotes-manager/internal/config"
)

type dbPointer struct {
	Current string `json:"current"`
	Path    string `json:"path"`
	Updated string `json:"updated"`
}

func dbSidecars(path string) []string {
	base := filepath.Base(path)
	return []string{base, base + "-wal", base + "-shm", base + "-journal"}
}

func removeStaleDBs(dir string, keep map[string]bool) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		name := e.Name()
		if keep[name] || !strings.HasPrefix(name, "canon-") {
			continue
		}
		if strings.HasSuffix(name, ".db") ||
			strings.HasSuffix(name, ".db-wal") ||
			strings.HasSuffix(name, ".db-shm") ||
			strings.HasSuffix(name, ".db-journal") {
			if err := os.Remove(filepath.Join(dir, name)); err != nil {
				return fmt.Errorf("remove %s: %w", name, err)
			}
		}
	}
	return nil
}

func PublishDB(cfg *config.Config, runID, dbPath string) error {
	if !utf8.ValidString(runID) || !utf8.ValidString(dbPath) {
		return fmt.Errorf("publish db: run id and path must be valid UTF-8")
	}
	dir := cfg.Canon.Paths.DBDir
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("publish db: %w", err)
	}

	keep := map[string]bool{}
	for _, n := range dbSidecars(dbPath) {
		keep[n] = true
	}
	superseded := ""
	if _, curPath, err := CurrentDB(cfg); err == nil {
		if curPath != dbPath {
			superseded = curPath
			for _, n := range dbSidecars(curPath) {
				keep[n] = true
			}
		}
	}
	if err := removeStaleDBs(dir, keep); err != nil {
		return fmt.Errorf("publish db cleanup: %w", err)
	}

	raw, err := json.Marshal(dbPointer{
		Current: runID,
		Path:    dbPath,
		Updated: time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".pointer-*.tmp")
	if err != nil {
		return fmt.Errorf("publish db: %w", err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	if _, err := tmp.Write(raw); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("publish db: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("publish db: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("publish db: %w", err)
	}
	if err := os.Rename(tmpName, cfg.Canon.DBPointerFile()); err != nil {
		return fmt.Errorf("publish db: %w", err)
	}

	if superseded != "" {
		keepNew := map[string]bool{}
		for _, n := range dbSidecars(dbPath) {
			keepNew[n] = true
		}
		if err := removeStaleDBsWithRetry(dir, keepNew); err != nil {
			return fmt.Errorf("%w: pointer now references %s, but retiring the previous database failed: %w", ErrRetirementLag, dbPath, err)
		}
	}
	return nil
}

var ErrRetirementLag = errors.New("publish committed, superseded database retires later")

const (
	retireAttempts = 5
	retireDelay    = 500 * time.Millisecond
)

func removeStaleDBsWithRetry(dir string, keep map[string]bool) error {
	var err error
	for range retireAttempts {
		if err = removeStaleDBs(dir, keep); err == nil {
			return nil
		}
		time.Sleep(retireDelay)
	}
	return err
}

func CurrentDB(cfg *config.Config) (runID, path string, err error) {
	raw, err := os.ReadFile(cfg.Canon.DBPointerFile())
	if err != nil {
		return "", "", fmt.Errorf("read pointer: %w", err)
	}
	var p dbPointer
	if err := json.Unmarshal(raw, &p); err != nil {
		return "", "", fmt.Errorf("parse pointer %s: %w", cfg.Canon.DBPointerFile(), err)
	}
	return p.Current, p.Path, nil
}
