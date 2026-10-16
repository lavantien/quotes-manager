package server

import (
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lavantien/quotes-manager/internal/canon/graph"
)

func storeUsable(t *testing.T, label string, s *graph.Store) bool {
	t.Helper()
	err := s.DB().PingContext(t.Context())
	if err == nil {
		return true
	}
	if strings.Contains(err.Error(), "closed") {
		return false
	}
	t.Fatalf("%s ping = %v, want usable or closed", label, err)
	return false
}

func TestDBStateStablePoolWhileUnchanged(t *testing.T) {
	srv, _ := newFixtureServer(t)
	ctx := t.Context()

	first, err := srv.db.Store(ctx)
	if err != nil {
		t.Fatal(err)
	}
	second, err := srv.db.Store(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatal("unchanged pointer must reuse the resolved pool")
	}
	if !storeUsable(t, "first pool", first) {
		t.Fatal("first pool must stay open while the pointer is unchanged")
	}
}

func TestDBStateSameMtimeDifferentHashSwaps(t *testing.T) {
	srv, cfg := newFixtureServer(t)
	ctx := t.Context()

	first, err := srv.db.Store(ctx)
	if err != nil {
		t.Fatal(err)
	}
	pinModTime, _, err := pointerStamp(cfg.Canon.DBPointerFile())
	if err != nil {
		t.Fatal(err)
	}
	fixtureDB(t, cfg, "run2", "Mūlapariyāya v2")
	if err := os.Chtimes(cfg.Canon.DBPointerFile(), pinModTime, pinModTime); err != nil {
		t.Fatal(err)
	}
	gotModTime, _, err := pointerStamp(cfg.Canon.DBPointerFile())
	if err != nil {
		t.Fatal(err)
	}
	if !gotModTime.Equal(pinModTime) {
		t.Fatalf("filesystem did not preserve the pinned mtime: %v vs %v", gotModTime, pinModTime)
	}

	runID, err := srv.db.Run(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if runID != "run2" {
		t.Fatalf("run after same-mtime content swap = %q, want run2", runID)
	}
	if storeUsable(t, "superseded pool", first) {
		t.Fatal("the superseded pool must be closed by the swap")
	}
}

func TestDBStateSameHashDifferentMtimeSwaps(t *testing.T) {
	srv, cfg := newFixtureServer(t)
	ctx := t.Context()

	first, err := srv.db.Store(ctx)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(cfg.Canon.DBPointerFile())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfg.Canon.DBPointerFile(), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	bumped := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(cfg.Canon.DBPointerFile(), bumped, bumped); err != nil {
		t.Fatal(err)
	}

	second, err := srv.db.Store(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !storeUsable(t, "re-resolved pool", second) {
		t.Fatal("the re-resolved pool must be usable")
	}
	if storeUsable(t, "superseded pool", first) {
		t.Fatal("identical content with a changed mtime must still swap and close the old pool")
	}
}

func TestDBStateRefreshReplacesPool(t *testing.T) {
	srv, _ := newFixtureServer(t)
	ctx := t.Context()

	first, err := srv.db.Store(ctx)
	if err != nil {
		t.Fatal(err)
	}
	srv.db.Refresh()
	if storeUsable(t, "pre-refresh pool", first) {
		t.Fatal("Refresh must replace the pool even when the pointer is unchanged")
	}
	second, err := srv.db.Store(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !storeUsable(t, "post-refresh pool", second) {
		t.Fatal("the post-refresh pool must be usable")
	}
}

func TestDBStateReopensAfterClose(t *testing.T) {
	cfg := newFixtureConfig(t)
	d := newDBState(cfg)
	fixtureDB(t, cfg, "run1", "Mūlapariyāya")

	if _, err := d.Store(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	store, err := d.Store(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if store == nil {
		t.Fatal("Store after Close must reopen, not return a nil pool")
	}
	if _, err := store.Stats(t.Context()); err != nil {
		t.Fatalf("reopened pool stats: %v", err)
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestDBStateConcurrentResolveKeepsEveryPool(t *testing.T) {
	srv, cfg := newFixtureServer(t)
	ctx := t.Context()

	if _, err := srv.db.Store(ctx); err != nil {
		t.Fatal(err)
	}
	fixtureDB(t, cfg, "run2", "Mūlapariyāya v2")

	const n = 16
	stores := make([]*graph.Store, n)
	errs := make([]error, n)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			stores[i], errs[i] = srv.db.Store(ctx)
		}()
	}
	close(start)
	wg.Wait()

	for i := range n {
		if errs[i] != nil {
			t.Fatalf("resolver %d: %v", i, errs[i])
		}
		if stores[i] == nil {
			t.Fatalf("resolver %d returned a nil pool", i)
		}
		if !storeUsable(t, "raced pool", stores[i]) {
			t.Fatalf("resolver %d holds a pool another resolver closed", i)
		}
	}
}

func TestDuckStateStablePoolWhileUnchanged(t *testing.T) {
	srv, cfg := newFixtureServer(t)
	ctx := t.Context()

	writeSidecarTable(t, cfg, "t1")
	db1, err := srv.duck.State(ctx)
	if err != nil {
		t.Fatal(err)
	}
	db2, err := srv.duck.State(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if db1 != db2 {
		t.Fatal("unchanged sidecar must reuse the open pool")
	}
	if err := db1.PingContext(ctx); err != nil {
		t.Fatalf("first sidecar pool = %v, want open", err)
	}
}

func TestDuckStateReopensOnMtimeBump(t *testing.T) {
	srv, cfg := newFixtureServer(t)
	ctx := t.Context()

	writeSidecarTable(t, cfg, "t1")
	db1, err := srv.duck.State(ctx)
	if err != nil {
		t.Fatal(err)
	}
	future := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(cfg.Canon.DuckDBFile(), future, future); err != nil {
		t.Fatal(err)
	}
	db2, err := srv.duck.State(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := db2.PingContext(ctx); err != nil {
		t.Fatalf("reopened sidecar pool = %v", err)
	}
	if err := db1.PingContext(ctx); err == nil ||
		!strings.Contains(err.Error(), "closed") {
		t.Fatalf("pre-bump pool = %v, want closed", err)
	}
}
