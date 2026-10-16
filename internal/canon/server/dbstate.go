package server

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/lavantien/quotes-manager/internal/canon/graph"
	"github.com/lavantien/quotes-manager/internal/config"
)

type dbState struct {
	cfg *config.Config

	mu      sync.RWMutex
	store   *graph.Store
	runID   string
	modTime time.Time
	hash    [8]byte
}

func newDBState(cfg *config.Config) *dbState {
	return &dbState{cfg: cfg}
}

func pointerStamp(path string) (time.Time, [8]byte, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return time.Time{}, [8]byte{}, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}, [8]byte{}, err
	}
	sum := sha256.Sum256(raw)
	var hash [8]byte
	copy(hash[:], sum[:8])
	return info.ModTime(), hash, nil
}

func (d *dbState) Store(ctx context.Context) (*graph.Store, error) {
	store, _, err := d.resolve(ctx, false)
	return store, err
}

func (d *dbState) Run(ctx context.Context) (string, error) {
	_, runID, err := d.resolve(ctx, false)
	return runID, err
}

func (d *dbState) Refresh() {
	_, _, _ = d.resolve(context.Background(), true)
}

func (d *dbState) resolve(ctx context.Context, force bool) (*graph.Store, string, error) {
	if err := ctx.Err(); err != nil {
		return nil, "", err
	}
	modTime, hash, err := pointerStamp(d.cfg.Canon.DBPointerFile())
	if err != nil {
		return nil, "", fmt.Errorf("%w: %v", ErrNotReady, err)
	}

	d.mu.RLock()
	unchanged := d.store != nil && !force && d.modTime.Equal(modTime) && d.hash == hash
	store, runID := d.store, d.runID
	d.mu.RUnlock()
	if unchanged {
		return store, runID, nil
	}

	runID, dbPath, err := graph.CurrentDB(d.cfg)
	if err != nil {
		return nil, "", fmt.Errorf("%w: %v", ErrNotReady, err)
	}
	next, err := graph.OpenRO(dbPath)
	if err != nil {
		return nil, "", fmt.Errorf("open published db %s: %w", dbPath, err)
	}
	pool := next.DB()
	pool.SetMaxOpenConns(4)
	pool.SetConnMaxIdleTime(30 * time.Second)

	d.mu.Lock()
	if !force && d.store != nil && d.store != next && d.modTime.Equal(modTime) && d.hash == hash {
		winner, winnerRun := d.store, d.runID
		d.mu.Unlock()
		_ = next.Close()
		return winner, winnerRun, nil
	}
	previous := d.store
	d.store, d.runID, d.modTime, d.hash = next, runID, modTime, hash
	d.mu.Unlock()

	if previous != nil {
		_ = previous.Close()
	}
	return next, runID, nil
}

func (d *dbState) Close() error {
	d.mu.Lock()
	previous := d.store
	d.store, d.runID = nil, ""
	d.mu.Unlock()
	if previous != nil {
		return previous.Close()
	}
	return nil
}
