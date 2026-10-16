package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/lavantien/quotes-manager/internal/canon/jobs"
)

func FuzzJobsStage(f *testing.F) {
	tooMany, err := json.Marshal(make([]string, maxJobArgs+1))
	if err != nil {
		f.Fatal(err)
	}
	for _, seed := range []struct {
		body string
		args []byte
	}{
		{`{"stage":"export"}`, nil},
		{`{"stage":"sync"}`, nil},
		{`{"stage":"all"}`, nil},
		{`{"stage":"nope"}`, nil},
		{`{"stage":""}`, nil},
		{`{"stage":"expo"}`, nil},
		{`{"stage":"export "}`, nil},
		{`{"stage":"EXPORT"}`, nil},
		{`{}`, nil},
		{``, nil},
		{`{`, nil},
		{`["sync"]`, nil},
		{`{"stage":123}`, nil},
		{`null`, nil},
		{"\x00\x01\x02", nil},
		{`export`, []byte(`["--source","bilara"]`)},
		{`prune`, []byte(`["--nope"]`)},
		{`sync`, tooMany},
		{`export`, []byte(`null`)},
		{`export`, []byte(`"--x"`)},
		{`export`, []byte(`x`)},
	} {
		f.Add(seed.body, seed.args)
	}
	srv, _ := newFixtureServer(f)
	withFakeExec(f, srv, func(context.Context, io.Writer) error { return nil })
	h := srv.Routes()
	f.Fuzz(func(t *testing.T, body string, args []byte) {
		if hasNUL(body) || bytes.ContainsRune(args, 0) {
			return
		}
		payload := body
		if len(args) > 0 {
			stage, err := json.Marshal(body)
			if err != nil {
				t.Fatalf("marshal stage %q: %v", body, err)
			}
			payload = `{"stage":` + string(stage) + `,"args":` + string(args) + `}`
		}
		rec := postBody(t, h, "/api/v1/jobs", payload)
		switch rec.Code {
		case http.StatusAccepted:
			var snap jobs.JobSnapshot
			if err := json.Unmarshal(rec.Body.Bytes(), &snap); err != nil {
				t.Fatalf("body %q args %s 202 = %s", body, args, rec.Body.String())
			}
		case http.StatusBadRequest:
			var env errorEnvelope
			if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil ||
				(env.Error.Code != codeStageInvalid && env.Error.Code != codeInvalidParam &&
					env.Error.Code != codeArgsInvalid) {
				t.Fatalf("body %q args %s 400 = %s", body, args, rec.Body.String())
			}
		case http.StatusConflict:
			if !strings.Contains(rec.Body.String(), codeJobBusy) {
				t.Fatalf("body %q args %s 409 = %s", body, args, rec.Body.String())
			}
		default:
			t.Fatalf("body %q args %s = %d: %s", body, args, rec.Code, rec.Body.String())
		}
	})
}
