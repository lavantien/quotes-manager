package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"slices"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/lavantien/quotes-manager/internal/canon/jobs"
	"github.com/lavantien/quotes-manager/internal/config"
)

type Server struct {
	cfg  *config.Config
	db   *dbState
	duck *duckState
	jobs *jobs.Runner
	log  *slog.Logger

	execMu    sync.RWMutex
	stageExec func(ctx context.Context, stage string, args []string, out io.Writer) error

	closing atomic.Bool
	hooks   *hookTracker
}

func New(cfg *config.Config) (*Server, error) {
	if cfg == nil {
		return nil, fmt.Errorf("server: config required")
	}
	s := &Server{
		cfg:  cfg,
		db:   newDBState(cfg),
		duck: newDuckState(cfg),
		log:  slog.Default(),
	}
	s.setStageExec(s.defaultStageExec)
	s.hooks = newHookTracker()
	s.jobs = jobs.NewRunner(jobs.OnFinish(func(stage string, _ error) {
		s.hooks.begin()
		defer s.hooks.end()
		if s.closing.Load() {
			return
		}
		s.db.Refresh()
		if slices.Contains(duckRebuildStages, stage) {
			s.duck.Reopen()
		}
	}))
	return s, nil
}

type hookTracker struct {
	mu   sync.Mutex
	n    int
	idle chan struct{}
}

func newHookTracker() *hookTracker {
	idle := make(chan struct{})
	close(idle)
	return &hookTracker{idle: idle}
}

func (h *hookTracker) begin() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.n == 0 {
		h.idle = make(chan struct{})
	}
	h.n++
}

func (h *hookTracker) end() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.n--
	if h.n == 0 {
		close(h.idle)
	}
}

func (h *hookTracker) wait(timeout time.Duration) {
	h.mu.Lock()
	ch := h.idle
	h.mu.Unlock()
	select {
	case <-ch:
	case <-time.After(timeout):
	}
}

func (s *Server) RegisterRoutes(m *http.ServeMux) {
	s.registerReadRoutes(m)
	s.registerGraphRoutes(m)
	s.registerJobsRoutes(m)
	s.registerAnalyticsRoutes(m)
	s.registerReportRoutes(m)
	s.registerPageRoutes(m)
	m.HandleFunc("/api/", s.apiNotFound)
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	s.RegisterRoutes(mux)
	mux.HandleFunc("/", s.pageNotFound)
	var h http.Handler = mux
	h = s.recoverPanic(h)
	h = s.securityHeaders(h)
	h = s.checkOrigin(h)
	h = s.logRequests(h)
	return h
}

func (s *Server) Serve(ctx context.Context) error {
	srv := &http.Server{
		Addr:         s.cfg.Server.Addr,
		Handler:      s.Routes(),
		ReadTimeout:  time.Duration(s.cfg.Server.ReadTimeoutSeconds) * time.Second,
		WriteTimeout: time.Duration(s.cfg.Server.WriteTimeoutSeconds) * time.Second,
		IdleTimeout:  time.Duration(s.cfg.Server.IdleTimeoutSeconds) * time.Second,
	}
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("server: %w", err)
	case <-ctx.Done():
	}

	budget := time.Duration(s.cfg.Server.ShutdownTimeoutSeconds) * time.Second
	deadline := time.Now().Add(budget)
	s.cancelActiveJobs(budget)

	shutdownCtx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("server shutdown: %w", err)
	}
	if err := <-errCh; err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("server: %w", err)
	}
	return s.Close()
}

const jobDrainInterval = 25 * time.Millisecond

func (s *Server) cancelActiveJobs(budget time.Duration) {
	for _, j := range s.jobs.List() {
		_ = s.jobs.Cancel(j.ID())
	}
	deadline := time.Now().Add(budget)
	for {
		active := false
		for _, j := range s.jobs.List() {
			switch j.State() {
			case jobs.StateQueued, jobs.StateRunning:
				active = true
			}
		}
		if !active || time.Now().After(deadline) {
			return
		}
		time.Sleep(jobDrainInterval)
	}
}

func (s *Server) Close() error {
	s.closing.Store(true)
	s.hooks.wait(time.Duration(s.cfg.Server.ShutdownTimeoutSeconds) * time.Second)
	dbErr := s.db.Close()
	duckErr := s.duck.Close()
	return errors.Join(dbErr, duckErr)
}
