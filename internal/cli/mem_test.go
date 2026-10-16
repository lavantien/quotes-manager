package cli

import (
	"runtime"
	"testing"
)

func TestPipelineMemoryBounded(t *testing.T) {
	gc()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)
	goroutinesBefore := runtime.NumGoroutine()

	tmp := t.TempDir()
	buildHermeticBilara(t, tmp+"/bilara-src")
	buildHermeticEditions(t, tmp+"/editions-src")
	cfgPath := writeHermeticConfig(t, tmp)

	for _, pass := range []int{0, 1} {
		if pass == 0 {
			if code, out := runCLI(t, cfgPath, "sync"); code != 0 {
				t.Fatalf("sync exit %d: %s", code, out)
			}
		}
		for _, cmd := range []string{"parse", "load", "export", "report"} {
			if code, out := runCLI(t, cfgPath, cmd); code != 0 {
				t.Fatalf("pass %d %s exit %d: %s", pass, cmd, code, out)
			}
		}
		if code, _ := runCLI(t, cfgPath, "triangulate"); code != 1 {
			t.Fatalf("pass %d triangulate exit %d, want 1", pass, code)
		}
	}

	gc()
	var after runtime.MemStats
	runtime.ReadMemStats(&after)

	const heapSlack, sysSlack = 32 << 20, 256 << 20
	if after.HeapAlloc > before.HeapAlloc+heapSlack {
		t.Fatalf("heap did not return to baseline: before=%d after=%d (leak?)", before.HeapAlloc, after.HeapAlloc)
	}
	if after.Sys > before.Sys+sysSlack {
		t.Fatalf("runtime Sys grew beyond slack: before=%d after=%d (leak?)", before.Sys, after.Sys)
	}
	if got := runtime.NumGoroutine(); got > goroutinesBefore+2 {
		t.Fatalf("goroutines accumulated: before=%d after=%d (leak?)", goroutinesBefore, got)
	}
}

func gc() {
	runtime.GC()
	runtime.GC()
}
