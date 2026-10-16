package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/lavantien/quotes-manager/internal/canon/jobs"
	"github.com/lavantien/quotes-manager/internal/cli"
)

const (
	codeStageInvalid  = "stage_invalid"
	codeArgsInvalid   = "args_invalid"
	codeJobBusy       = "job_busy"
	codeJobNotFound   = "job_not_found"
	codeNotCancelable = "not_cancelable"
)

var duckRebuildStages = []string{"export", "all"}

const jobBodyMaxBytes = 4 << 10

const maxJobArgs = 32

func (s *Server) registerJobsRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/jobs", s.apiJobStart)
	mux.HandleFunc("GET /api/v1/jobs", s.apiJobsList)
	mux.HandleFunc("GET /api/v1/jobs/{id}", s.apiJobGet)
	mux.HandleFunc("GET /api/v1/jobs/{id}/logs", s.apiJobLogs)
	mux.HandleFunc("POST /api/v1/jobs/{id}/cancel", s.apiJobCancel)
}

func (s *Server) startJob(stage string, args []string) (*jobs.Job, error) {
	if !slices.Contains(cli.Stages(), stage) {
		return nil, &statusError{
			status:  http.StatusBadRequest,
			code:    codeStageInvalid,
			message: "stage must be one of: " + strings.Join(cli.Stages(), ", "),
		}
	}
	if err := s.validateJobArgs(stage, args); err != nil {
		return nil, err
	}
	return s.jobs.StartFunc(s.cfg, stage, func(ctx context.Context, out io.Writer) error {
		return s.execStage(ctx, stage, args, out)
	})
}

func (s *Server) validateJobArgs(stage string, args []string) error {
	if stage != "report" {
		return nil
	}
	root, err := filepath.Abs(s.cfg.Canon.Paths.ReportsDir)
	if err != nil {
		return &statusError{
			status:  http.StatusBadRequest,
			code:    codeArgsInvalid,
			message: "report --out cannot be validated",
		}
	}
	for i := 0; i < len(args); i++ {
		var val string
		switch {
		case args[i] == "--out":
			if i+1 == len(args) {
				return &statusError{
					status:  http.StatusBadRequest,
					code:    codeArgsInvalid,
					message: "report --out needs a value",
				}
			}
			val = args[i+1]
			i++
		case strings.HasPrefix(args[i], "--out="):
			val = strings.TrimPrefix(args[i], "--out=")
		default:
			continue
		}
		if !underDir(root, val) {
			return &statusError{
				status:  http.StatusBadRequest,
				code:    codeArgsInvalid,
				message: "report --out must stay under the reports dir",
			}
		}
	}
	return nil
}

func underDir(root, dir string) bool {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(root, abs)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func (s *Server) setStageExec(fn func(ctx context.Context, stage string, args []string, out io.Writer) error) {
	s.execMu.Lock()
	s.stageExec = fn
	s.execMu.Unlock()
}

func (s *Server) execStage(ctx context.Context, stage string, args []string, out io.Writer) error {
	s.execMu.RLock()
	fn := s.stageExec
	s.execMu.RUnlock()
	return fn(ctx, stage, args, out)
}

func (s *Server) defaultStageExec(ctx context.Context, stage string, args []string, out io.Writer) error {
	if slices.Contains(duckRebuildStages, stage) {
		if err := s.duck.Quiesce(); err != nil {
			s.duck.Reopen()
			return fmt.Errorf("quiesce duckdb for %s: %w", stage, err)
		}
	}
	return cli.RunStage(ctx, s.cfg, stage, args, out)
}

func jobPathID(r *http.Request) (int64, error) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		return 0, errNotFound(codeJobNotFound, "job id must be an integer")
	}
	return id, nil
}

func (s *Server) apiJobStart(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Stage string   `json:"stage"`
		Args  []string `json:"args"`
	}
	dec := json.NewDecoder(io.LimitReader(r.Body, jobBodyMaxBytes))
	if err := dec.Decode(&body); err != nil || dec.More() {
		writeErr(w, http.StatusBadRequest, codeInvalidParam,
			`body must be one json object: {"stage": "sync", "args": ["--source", "bilara"]}`)
		return
	}
	if len(body.Args) > maxJobArgs {
		writeErr(w, http.StatusBadRequest, codeArgsInvalid,
			fmt.Sprintf("args must hold at most %d entries", maxJobArgs))
		return
	}
	j, err := s.startJob(body.Stage, body.Args)
	if err != nil {
		if errors.Is(err, jobs.ErrBusy) {
			writeErr(w, http.StatusConflict, codeJobBusy, "another job is queued or running")
			return
		}
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, j.Snapshot())
}

func (s *Server) apiJobsList(w http.ResponseWriter, _ *http.Request) {
	all := s.jobs.List()
	out := make([]jobs.JobSnapshot, 0, len(all))
	for _, j := range all {
		out = append(out, j.Snapshot())
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) apiJobGet(w http.ResponseWriter, r *http.Request) {
	id, err := jobPathID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	j, ok := s.jobs.Get(id)
	if !ok {
		writeError(w, errNotFound(codeJobNotFound, fmt.Sprintf("job %d not found", id)))
		return
	}
	writeJSON(w, http.StatusOK, j.Snapshot())
}

func (s *Server) apiJobLogs(w http.ResponseWriter, r *http.Request) {
	id, err := jobPathID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	after, err := queryInt64(r, "after", 0)
	if err != nil {
		writeError(w, err)
		return
	}
	if _, ok := s.jobs.Get(id); !ok {
		writeError(w, errNotFound(codeJobNotFound, fmt.Sprintf("job %d not found", id)))
		return
	}
	lines := s.jobs.LogsAfter(id, after)
	if lines == nil {
		lines = []jobs.JobLogLine{}
	}
	writeJSON(w, http.StatusOK, lines)
}

func (s *Server) apiJobCancel(w http.ResponseWriter, r *http.Request) {
	id, err := jobPathID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	j, ok := s.jobs.Get(id)
	if !ok {
		writeError(w, errNotFound(codeJobNotFound, fmt.Sprintf("job %d not found", id)))
		return
	}
	if err := s.jobs.Cancel(id); err != nil {
		if errors.Is(err, jobs.ErrNotCancelable) {
			writeErr(w, http.StatusConflict, codeNotCancelable,
				fmt.Sprintf("job %d already finished", id))
			return
		}
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, j.Snapshot())
}
