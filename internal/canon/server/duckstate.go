package server

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/lavantien/quotes-manager/internal/config"

	_ "github.com/duckdb/duckdb-go/v2"
)

type duckState struct {
	cfg *config.Config

	mu         sync.Mutex
	db         *sql.DB
	modTime    time.Time
	rebuilding bool
}

const (
	quiesceDrainTimeout = 5 * time.Second
	quiesceDrainPoll    = 100 * time.Millisecond
)

func newDuckState(cfg *config.Config) *duckState {
	return &duckState{cfg: cfg}
}

func (d *duckState) dsn() string {
	return fmt.Sprintf("%s?access_mode=read_only&threads=%d&memory_limit=%dGB",
		d.cfg.Canon.DuckDBFile(), d.cfg.Server.AnalyticsThreads, d.cfg.Server.AnalyticsMemoryGB)
}

func (d *duckState) State(ctx context.Context) (*sql.DB, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	info, err := os.Stat(d.cfg.Canon.DuckDBFile())
	if err != nil {
		return nil, fmt.Errorf("%w: duckdb sidecar absent", ErrNotReady)
	}
	d.mu.Lock()
	if d.rebuilding {
		d.mu.Unlock()
		return nil, fmt.Errorf("%w: duckdb sidecar rebuilding", ErrNotReady)
	}
	db := d.db
	d.mu.Unlock()
	if db != nil && d.modTimeEqual(info.ModTime()) {
		return db, nil
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	if d.rebuilding {
		return nil, fmt.Errorf("%w: duckdb sidecar rebuilding", ErrNotReady)
	}
	if err := d.closeLocked(); err != nil {
		return nil, err
	}
	newDB, err := sql.Open("duckdb", d.dsn())
	if err != nil {
		return nil, d.wrapOpenErr(err)
	}
	newDB.SetMaxOpenConns(2)
	if err := newDB.PingContext(ctx); err != nil {
		_ = newDB.Close()
		return nil, d.wrapOpenErr(err)
	}
	d.db, d.modTime = newDB, d.lastModTime()
	return newDB, nil
}

func (d *duckState) wrapOpenErr(err error) error {
	if strings.Contains(err.Error(), "different configuration") {
		return fmt.Errorf("%w: duckdb sidecar rebuilding: %w", ErrNotReady, err)
	}
	return fmt.Errorf("open duckdb ro: %w", err)
}

func (d *duckState) modTimeEqual(mod time.Time) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.db != nil && d.modTime.Equal(mod)
}

func (d *duckState) lastModTime() time.Time {
	info, err := os.Stat(d.cfg.Canon.DuckDBFile())
	if err != nil {
		return time.Time{}
	}
	return info.ModTime()
}

func (d *duckState) Quiesce() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.rebuilding = true
	pool := d.db
	if err := d.closeLocked(); err != nil {
		return err
	}
	if pool == nil {
		return nil
	}
	deadline := time.Now().Add(quiesceDrainTimeout)
	d.mu.Unlock()
	for pool.Stats().InUse > 0 && time.Now().Before(deadline) {
		time.Sleep(quiesceDrainPoll)
	}
	d.mu.Lock()
	if n := pool.Stats().InUse; n > 0 {
		return fmt.Errorf("duckdb sidecar still has %d in-flight analytics readers", n)
	}
	return nil
}

func (d *duckState) Reopen() {
	d.mu.Lock()
	d.modTime = time.Time{}
	d.rebuilding = false
	d.mu.Unlock()
}

func (d *duckState) closeLocked() error {
	if d.db == nil {
		return nil
	}
	err := d.db.Close()
	d.db = nil
	return err
}

func (d *duckState) Close() error {
	return d.Quiesce()
}
