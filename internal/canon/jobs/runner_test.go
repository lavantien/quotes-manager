package jobs

import (
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lavantien/quotes-manager/internal/config"
	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

type hookResult struct {
	stage string
	err   error
}

func newTestRunner() (*Runner, chan hookResult) {
	hooks := make(chan hookResult, 16)
	return NewRunner(OnFinish(func(stage string, err error) {
		hooks <- hookResult{stage: stage, err: err}
	})), hooks
}

func waitHook(t *testing.T, hooks chan hookResult) hookResult {
	t.Helper()
	select {
	case h := <-hooks:
		return h
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for the finish hook")
		return hookResult{}
	}
}

func assertNoExtraHook(t *testing.T, hooks chan hookResult) {
	t.Helper()
	select {
	case h := <-hooks:
		t.Fatalf("finish hook called more than once: %+v", h)
	default:
	}
}

func testCfg(lines int) *config.Config {
	cfg := config.Defaults()
	cfg.Server.JobLogLines = lines
	return cfg
}

func waitState(t *testing.T, j *Job, want State) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for j.State() != want {
		if time.Now().After(deadline) {
			t.Fatalf("job %d stuck in %s, want %s", j.ID(), j.State(), want)
		}
		time.Sleep(time.Millisecond)
	}
}

func instantStage(lines ...string) StageRunner {
	return func(ctx context.Context, out io.Writer) error {
		for _, l := range lines {
			_, _ = fmt.Fprintln(out, l)
		}
		return nil
	}
}

var errStage = errors.New("stage failed")

func failingStage() StageRunner {
	return func(ctx context.Context, out io.Writer) error { return errStage }
}

func blockedStage(started chan<- struct{}, release <-chan struct{}) StageRunner {
	return func(ctx context.Context, out io.Writer) error {
		close(started)
		<-release
		_, _ = fmt.Fprintln(out, "unblocked")
		return nil
	}
}

func ctxAwareStage(started chan<- struct{}) StageRunner {
	return func(ctx context.Context, out io.Writer) error {
		close(started)
		<-ctx.Done()
		return ctx.Err()
	}
}

func TestStartRejectsNonStage(t *testing.T) {
	r, hooks := newTestRunner()
	cfg := testCfg(8)
	for _, name := range []string{"", "bogus", "version", "query", "serve"} {
		if _, err := r.Start(cfg, name); err == nil {
			t.Fatalf("Start(%q) accepted a non-stage", name)
		}
	}
	if got := r.List(); len(got) != 0 {
		t.Fatalf("rejected starts must not create jobs, got %d", len(got))
	}
	assertNoExtraHook(t, hooks)
}

func TestStartFuncRejectsBadArguments(t *testing.T) {
	r, _ := newTestRunner()
	if _, err := r.StartFunc(nil, "sync", instantStage()); err == nil {
		t.Fatal("nil config must be rejected")
	}
	cfg := testCfg(4)
	cfg.Server.JobLogLines = 0
	if _, err := r.StartFunc(cfg, "sync", instantStage()); err == nil {
		t.Fatal("job_log_lines < 1 must be rejected")
	}
	if _, err := r.StartFunc(testCfg(4), "sync", nil); err == nil {
		t.Fatal("nil runner must be rejected")
	}
	if got := r.List(); len(got) != 0 {
		t.Fatalf("rejected starts must not create jobs, got %d", len(got))
	}
}

func TestBusyWhileRunning(t *testing.T) {
	r, hooks := newTestRunner()
	cfg := testCfg(16)
	started, release := make(chan struct{}), make(chan struct{})
	j, err := r.StartFunc(cfg, "sync", blockedStage(started, release))
	if err != nil {
		t.Fatalf("first start: %v", err)
	}
	<-started
	if _, err := r.StartFunc(cfg, "prune", instantStage()); !errors.Is(err, ErrBusy) {
		t.Fatalf("start while running: want ErrBusy, got %v", err)
	}
	if snap := j.Snapshot(); snap.State != StateRunning || snap.Finished != "" {
		t.Fatalf("running snapshot: %+v", snap)
	}
	close(release)
	h := waitHook(t, hooks)
	if h.stage != "sync" || h.err != nil {
		t.Fatalf("hook: %+v", h)
	}
	waitState(t, j, StateDone)
	assertNoExtraHook(t, hooks)

	j2, err := r.StartFunc(cfg, "report", instantStage())
	if err != nil {
		t.Fatalf("start after finish: %v", err)
	}
	waitHook(t, hooks)
	waitState(t, j2, StateDone)
}

type startResult struct {
	job *Job
	err error
}

func TestConcurrentStartSingleWinner(t *testing.T) {
	r, hooks := newTestRunner()
	cfg := testCfg(8)
	release := make(chan struct{})
	results := make(chan startResult, 16)
	for i := 0; i < 16; i++ {
		go func() {
			j, err := r.StartFunc(cfg, "sync", func(ctx context.Context, out io.Writer) error {
				<-release
				return nil
			})
			results <- startResult{job: j, err: err}
		}()
	}
	wins, busy, winner := 0, 0, (*Job)(nil)
	for i := 0; i < 16; i++ {
		g := <-results
		switch {
		case g.err == nil:
			wins++
			winner = g.job
		case errors.Is(g.err, ErrBusy):
			busy++
		default:
			t.Fatalf("unexpected start error: %v", g.err)
		}
	}
	if wins != 1 || busy != 15 {
		t.Fatalf("want 1 win and 15 busy, got %d and %d", wins, busy)
	}
	close(release)
	h := waitHook(t, hooks)
	if h.err != nil {
		t.Fatalf("winner hook err: %v", h.err)
	}
	waitState(t, winner, StateDone)
}

func TestDoneLifecycle(t *testing.T) {
	r, hooks := newTestRunner()
	j, err := r.StartFunc(testCfg(32), "report", instantStage("a", "b"))
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	h := waitHook(t, hooks)
	if h.stage != "report" || h.err != nil {
		t.Fatalf("hook: %+v", h)
	}
	waitState(t, j, StateDone)
	assertNoExtraHook(t, hooks)

	snap := j.Snapshot()
	if snap.ID != 1 || snap.Stage != "report" || snap.State != StateDone || snap.Error != "" {
		t.Fatalf("snapshot: %+v", snap)
	}
	st, err := time.Parse(time.RFC3339, snap.Started)
	if err != nil {
		t.Fatalf("started not RFC3339: %v", err)
	}
	ft, err := time.Parse(time.RFC3339, snap.Finished)
	if err != nil {
		t.Fatalf("finished not RFC3339: %v", err)
	}
	if ft.Before(st) {
		t.Fatalf("finished %s before started %s", snap.Finished, snap.Started)
	}
	lines := r.LogsAfter(j.ID(), 0)
	if len(lines) != 2 || lines[0].Text != "a" || lines[1].Text != "b" {
		t.Fatalf("logs: %+v", lines)
	}
	if lines[0].Seq != 1 || lines[1].Seq != 2 || lines[0].At.IsZero() {
		t.Fatalf("log seqs/timestamps: %+v", lines)
	}
}

func TestErrorLifecycle(t *testing.T) {
	r, hooks := newTestRunner()
	j, err := r.StartFunc(testCfg(8), "export", failingStage())
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	h := waitHook(t, hooks)
	if !errors.Is(h.err, errStage) {
		t.Fatalf("hook err: %v", h.err)
	}
	waitState(t, j, StateError)
	assertNoExtraHook(t, hooks)
	if snap := j.Snapshot(); snap.State != StateError || snap.Error != errStage.Error() {
		t.Fatalf("snapshot: %+v", snap)
	}
}

func TestPanicBecomesError(t *testing.T) {
	r, hooks := newTestRunner()
	j, err := r.StartFunc(testCfg(8), "sync", func(ctx context.Context, out io.Writer) error {
		panic("boom")
	})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	h := waitHook(t, hooks)
	if h.err == nil {
		t.Fatal("panic must reach the hook as an error")
	}
	waitState(t, j, StateError)
	assertNoExtraHook(t, hooks)
	if snap := j.Snapshot(); !strings.Contains(snap.Error, "boom") {
		t.Fatalf("panic text missing from snapshot: %+v", snap)
	}
}

func TestCancelCtxAwareStage(t *testing.T) {
	r, hooks := newTestRunner()
	started := make(chan struct{})
	j, err := r.StartFunc(testCfg(8), "load", ctxAwareStage(started))
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	<-started
	if err := r.Cancel(j.ID()); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	h := waitHook(t, hooks)
	if !errors.Is(h.err, context.Canceled) {
		t.Fatalf("hook err: %v", h.err)
	}
	waitState(t, j, StateCanceled)
	if snap := j.Snapshot(); snap.State != StateCanceled || snap.Error != context.Canceled.Error() {
		t.Fatalf("snapshot: %+v", snap)
	}
}

func TestCancelCtxIgnoringStageFinishesDone(t *testing.T) {
	r, hooks := newTestRunner()
	started, goOn := make(chan struct{}), make(chan struct{})
	j, err := r.StartFunc(testCfg(8), "sync", func(ctx context.Context, out io.Writer) error {
		close(started)
		<-goOn
		_, _ = fmt.Fprintln(out, "late")
		return nil
	})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	<-started
	if err := r.Cancel(j.ID()); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	close(goOn)
	h := waitHook(t, hooks)
	if h.err != nil {
		t.Fatalf("hook err: %v", h.err)
	}
	waitState(t, j, StateDone)
	if lines := r.LogsAfter(j.ID(), 0); len(lines) != 1 || lines[0].Text != "late" {
		t.Fatalf("logs after cancel: %+v", lines)
	}
}

func TestCancelFinishedAndMissing(t *testing.T) {
	r, hooks := newTestRunner()
	j, err := r.StartFunc(testCfg(8), "report", instantStage())
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	waitHook(t, hooks)
	waitState(t, j, StateDone)
	if err := r.Cancel(j.ID()); !errors.Is(err, ErrNotCancelable) {
		t.Fatalf("cancel finished: want ErrNotCancelable, got %v", err)
	}
	if err := r.Cancel(12345); err == nil {
		t.Fatal("cancel missing job must error")
	}
	if got := r.LogsAfter(12345, 0); got != nil {
		t.Fatalf("logs of missing job: %+v", got)
	}
}

func TestRingEvictionKeepsSeqMonotonic(t *testing.T) {
	r, hooks := newTestRunner()
	j, err := r.StartFunc(testCfg(3), "export", instantStage("l1", "l2", "l3", "l4", "l5"))
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	waitHook(t, hooks)
	waitState(t, j, StateDone)

	lines := r.LogsAfter(j.ID(), 0)
	if len(lines) != 3 {
		t.Fatalf("ring must hold exactly the capacity, got %d", len(lines))
	}
	want := []string{"l3", "l4", "l5"}
	for i, l := range lines {
		if l.Text != want[i] || l.Seq != int64(i+3) {
			t.Fatalf("line %d: %+v, want %s seq %d", i, l, want[i], i+3)
		}
	}
	if got := r.LogsAfter(j.ID(), 3); len(got) != 2 || got[0].Seq != 4 || got[1].Seq != 5 {
		t.Fatalf("logs after seq 3: %+v", got)
	}
	if got := r.LogsAfter(j.ID(), 5); len(got) != 0 {
		t.Fatalf("logs after last seq: %+v", got)
	}
}

func TestLineWriterChunking(t *testing.T) {
	r, hooks := newTestRunner()
	stage := func(ctx context.Context, out io.Writer) error {
		_, _ = out.Write([]byte("par"))
		_, _ = out.Write([]byte("tial\nsecond\nthi"))
		_, _ = out.Write([]byte("rd\r\n"))
		_, _ = out.Write([]byte("tail"))
		return nil
	}
	j, err := r.StartFunc(testCfg(8), "parse", stage)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	waitHook(t, hooks)
	waitState(t, j, StateDone)

	lines := r.LogsAfter(j.ID(), 0)
	want := []string{"partial", "second", "third", "tail"}
	if len(lines) != len(want) {
		t.Fatalf("lines: %+v", lines)
	}
	for i, l := range lines {
		if l.Text != want[i] {
			t.Fatalf("line %d = %q, want %q", i, l.Text, want[i])
		}
	}
}

func TestListOrderingAndSeqPerJob(t *testing.T) {
	r, hooks := newTestRunner()
	cfg := testCfg(8)
	for _, stage := range []string{"sync", "parse", "load"} {
		j, err := r.StartFunc(cfg, stage, instantStage("x"))
		if err != nil {
			t.Fatalf("start %s: %v", stage, err)
		}
		waitHook(t, hooks)
		waitState(t, j, StateDone)
	}
	list := r.List()
	if len(list) != 3 {
		t.Fatalf("list: %d jobs", len(list))
	}
	for i, j := range list {
		if j.ID() != int64(i+1) {
			t.Fatalf("list[%d].ID = %d", i, j.ID())
		}
		if s := j.Snapshot(); s.State != StateDone {
			t.Fatalf("job %d state %s", j.ID(), s.State)
		}
	}
	if _, ok := r.Get(2); !ok {
		t.Fatal("Get(2) missing")
	}
	if _, ok := r.Get(4); ok {
		t.Fatal("Get(4) must miss")
	}
	if lines := r.LogsAfter(2, 0); len(lines) != 1 || lines[0].Seq != 1 {
		t.Fatalf("seq must restart per job: %+v", lines)
	}
}

func TestHookCanInspectRunnerState(t *testing.T) {
	seen := make(chan State, 1)
	var r *Runner
	r = NewRunner(OnFinish(func(stage string, err error) {
		j, ok := r.Get(1)
		if !ok {
			t.Error("hook cannot find its own job")
		}
		_ = r.List()
		_ = r.LogsAfter(1, 0)
		seen <- j.State()
	}))
	if _, err := r.StartFunc(testCfg(4), "sync", instantStage("x")); err != nil {
		t.Fatalf("start: %v", err)
	}
	select {
	case s := <-seen:
		if s != StateDone {
			t.Fatalf("hook saw state %s, want done", s)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("hook deadlocked inspecting runner state")
	}
}

func TestParallelReadersDuringRun(t *testing.T) {
	r, hooks := newTestRunner()
	cfg := testCfg(64)
	started, release := make(chan struct{}), make(chan struct{})
	j, err := r.StartFunc(cfg, "triangulate", func(ctx context.Context, out io.Writer) error {
		close(started)
		i := 0
		for {
			select {
			case <-release:
				return nil
			default:
				_, _ = fmt.Fprintf(out, "line %d\n", i)
				i++
				if i%10 == 0 {
					time.Sleep(time.Millisecond)
				}
			}
		}
	})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	<-started
	var wg sync.WaitGroup
	deadline := time.Now().Add(200 * time.Millisecond)
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var seq int64
			for time.Now().Before(deadline) {
				for _, l := range r.LogsAfter(j.ID(), seq) {
					if l.Seq <= seq {
						t.Errorf("seq went backwards: %d after %d", l.Seq, seq)
					}
					seq = l.Seq
				}
				_ = j.Snapshot()
				_ = r.List()
			}
		}()
	}
	wg.Wait()
	close(release)
	waitHook(t, hooks)
	waitState(t, j, StateDone)
	if !slices.IsSortedFunc(r.LogsAfter(j.ID(), 0), func(a, b JobLogLine) int {
		return int(a.Seq - b.Seq)
	}) {
		t.Fatal("final logs not sorted by seq")
	}
}
