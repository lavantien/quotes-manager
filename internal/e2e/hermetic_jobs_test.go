package e2e

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/lavantien/quotes-manager/internal/canon/jobs"
)

const pipelineTimeout = 5 * time.Minute

func startJob(a *mergedApp, stage string) (int64, error) {
	f, err := a.fetch(http.MethodPost, "/api/v1/jobs", "application/json", `{"stage":"`+stage+`"}`, "")
	if err != nil {
		return 0, err
	}
	if f.status != http.StatusAccepted {
		return 0, fmt.Errorf("POST /api/v1/jobs %s = %d: %s", stage, f.status, f.body)
	}
	var snap jobs.JobSnapshot
	if err := json.Unmarshal([]byte(f.body), &snap); err != nil {
		return 0, fmt.Errorf("start %s body: %w (%s)", stage, err, f.body)
	}
	return snap.ID, nil
}

func jobDone(snap jobs.JobSnapshot) bool {
	return snap.State != jobs.StateQueued && snap.State != jobs.StateRunning
}

func waitJob(a *mergedApp, id int64, stage string) error {
	deadline := time.Now().Add(pipelineTimeout)
	for {
		f, err := a.fetch(http.MethodGet, fmt.Sprintf("/api/v1/jobs/%d", id), "", "", "")
		if err != nil {
			return err
		}
		if f.status != http.StatusOK {
			return fmt.Errorf("GET job %d (%s) = %d: %s", id, stage, f.status, f.body)
		}
		var snap jobs.JobSnapshot
		if err := json.Unmarshal([]byte(f.body), &snap); err != nil {
			return fmt.Errorf("job %d body: %w (%s)", id, err, f.body)
		}
		if jobDone(snap) {
			if snap.State == jobs.StateDone {
				return nil
			}
			return fmt.Errorf("job %d (%s) reached %s: %s", id, stage, snap.State, snap.Error)
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("job %d (%s) stuck after %s", id, stage, pipelineTimeout)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func runCanonPipeline(a *mergedApp) error {
	for _, stage := range []string{"sync", "parse", "load", "triangulate", "export"} {
		id, err := startJob(a, stage)
		if err != nil {
			return err
		}
		if err := waitJob(a, id, stage); err != nil {
			return err
		}
	}
	return nil
}
