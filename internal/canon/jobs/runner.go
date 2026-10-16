package jobs

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"sync"
	"time"

	"github.com/lavantien/quotes-manager/internal/cli"
	"github.com/lavantien/quotes-manager/internal/config"
)

type State string

const (
	StateQueued   State = "queued"
	StateRunning  State = "running"
	StateDone     State = "done"
	StateError    State = "error"
	StateCanceled State = "canceled"
)

var ErrBusy = errors.New("jobs: another job is queued or running")

var ErrNotCancelable = errors.New("jobs: job already finished")

type StageRunner func(ctx context.Context, out io.Writer) error

type JobLogLine struct {
	Seq  int64     `json:"seq"`
	Text string    `json:"text"`
	At   time.Time `json:"at"`
}

type JobSnapshot struct {
	ID       int64  `json:"id"`
	Stage    string `json:"stage"`
	State    State  `json:"state"`
	Started  string `json:"started"`
	Finished string `json:"finished"`
	Error    string `json:"error"`
}

type Job struct {
	id    int64
	stage string

	mu       sync.Mutex
	state    State
	started  time.Time
	finished time.Time
	errText  string
	cancel   context.CancelFunc

	ring *ringLog
}

func (j *Job) ID() int64 { return j.id }

func (j *Job) Stage() string { return j.stage }

func (j *Job) State() State {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.state
}

func (j *Job) Snapshot() JobSnapshot {
	j.mu.Lock()
	defer j.mu.Unlock()
	s := JobSnapshot{ID: j.id, Stage: j.stage, State: j.state, Error: j.errText}
	if !j.started.IsZero() {
		s.Started = j.started.UTC().Format(time.RFC3339)
	}
	if !j.finished.IsZero() {
		s.Finished = j.finished.UTC().Format(time.RFC3339)
	}
	return s
}

func (j *Job) begin() {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.state = StateRunning
	j.started = time.Now()
}

func (j *Job) finish(state State, err error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.state = state
	j.finished = time.Now()
	if err != nil {
		j.errText = err.Error()
	}
}

type Option func(*Runner)

func OnFinish(fn func(stage string, err error)) Option {
	return func(r *Runner) { r.onFinish = fn }
}

type Runner struct {
	mu       sync.Mutex
	nextID   int64
	jobs     []*Job
	active   *Job
	onFinish func(stage string, err error)
}

func NewRunner(opts ...Option) *Runner {
	r := &Runner{}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

func (r *Runner) Start(cfg *config.Config, stage string) (*Job, error) {
	if !slices.Contains(cli.Stages(), stage) {
		return nil, fmt.Errorf("jobs: %q is not a pipeline stage", stage)
	}
	return r.StartFunc(cfg, stage, func(ctx context.Context, out io.Writer) error {
		return cli.RunStage(ctx, cfg, stage, nil, out)
	})
}

func (r *Runner) StartFunc(cfg *config.Config, stage string, run StageRunner) (*Job, error) {
	if run == nil {
		return nil, errors.New("jobs: nil stage runner")
	}
	if cfg == nil || cfg.Server.JobLogLines < 1 {
		return nil, errors.New("jobs: config must set server.job_log_lines >= 1")
	}

	r.mu.Lock()
	if r.active != nil {
		r.mu.Unlock()
		return nil, ErrBusy
	}
	r.nextID++
	ctx, cancel := context.WithCancel(context.Background())
	j := &Job{
		id:     r.nextID,
		stage:  stage,
		state:  StateQueued,
		cancel: cancel,
		ring:   newRingLog(cfg.Server.JobLogLines),
	}
	r.jobs = append(r.jobs, j)
	r.active = j
	r.mu.Unlock()

	go r.execute(j, run, ctx, cancel)
	return j, nil
}

func (r *Runner) execute(j *Job, run StageRunner, ctx context.Context, cancel context.CancelFunc) {
	j.begin()
	lw := &lineWriter{ring: j.ring}
	err := invokeStage(ctx, run, lw)
	lw.flush()
	cancel()

	state := StateDone
	if err != nil {
		state = StateError
		if errors.Is(err, context.Canceled) {
			state = StateCanceled
		}
	}
	j.finish(state, err)

	r.mu.Lock()
	if r.active == j {
		r.active = nil
	}
	r.mu.Unlock()

	if r.onFinish != nil {
		r.onFinish(j.stage, err)
	}
}

func invokeStage(ctx context.Context, run StageRunner, out io.Writer) (err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("jobs: stage panicked: %v", p)
		}
	}()
	return run(ctx, out)
}

func (r *Runner) Get(id int64) (*Job, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, j := range r.jobs {
		if j.id == id {
			return j, true
		}
	}
	return nil, false
}

func (r *Runner) List() []*Job {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.jobs)
}

func (r *Runner) LogsAfter(id, seq int64) []JobLogLine {
	j, ok := r.Get(id)
	if !ok {
		return nil
	}
	return j.ring.snapshot(seq)
}

func (r *Runner) Cancel(id int64) error {
	j, ok := r.Get(id)
	if !ok {
		return fmt.Errorf("jobs: no job %d", id)
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.state != StateQueued && j.state != StateRunning {
		return ErrNotCancelable
	}
	j.cancel()
	return nil
}

type ringLog struct {
	mu    sync.Mutex
	lines []JobLogLine
	head  int
	n     int
	seq   int64
}

func newRingLog(capacity int) *ringLog {
	return &ringLog{lines: make([]JobLogLine, capacity)}
}

func (r *ringLog) append(text string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.seq++
	line := JobLogLine{Seq: r.seq, Text: text, At: time.Now()}
	if r.n < len(r.lines) {
		r.lines[(r.head+r.n)%len(r.lines)] = line
		r.n++
		return
	}
	r.lines[r.head] = line
	r.head = (r.head + 1) % len(r.lines)
}

func (r *ringLog) snapshot(after int64) []JobLogLine {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]JobLogLine, 0, r.n)
	for i := 0; i < r.n; i++ {
		line := r.lines[(r.head+i)%len(r.lines)]
		if line.Seq > after {
			out = append(out, line)
		}
	}
	return out
}

type lineWriter struct {
	ring    *ringLog
	partial []byte
}

func (w *lineWriter) Write(p []byte) (int, error) {
	w.partial = append(w.partial, p...)
	for {
		i := bytes.IndexByte(w.partial, '\n')
		if i < 0 {
			break
		}
		w.ring.append(trimCR(w.partial[:i]))
		rest := w.partial[i+1:]
		w.partial = append(w.partial[:0], rest...)
	}
	return len(p), nil
}

func (w *lineWriter) flush() {
	if len(w.partial) > 0 {
		w.ring.append(trimCR(w.partial))
		w.partial = w.partial[:0]
	}
}

func trimCR(b []byte) string { return string(bytes.TrimSuffix(b, []byte("\r"))) }
