package server

import (
	"encoding/json"
	"errors"
	"net/http"
)

const (
	codeInvalidParam = "invalid_param"
	codeDBNotReady   = "db_not_ready"
	codeNotFound     = "not_found"
	codeRunNotFound  = "run_not_found"
	codeForbidden    = "origin_rejected"
	codeInternal     = "internal_error"
)

var ErrNotReady = errors.New("database not ready")

type statusError struct {
	status  int
	code    string
	message string
}

func (e *statusError) Error() string { return e.message }

func errInvalidParam(message string) *statusError {
	return &statusError{status: http.StatusBadRequest, code: codeInvalidParam, message: message}
}

func errNotFound(code, message string) *statusError {
	return &statusError{status: http.StatusNotFound, code: code, message: message}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

type errorEnvelope struct {
	Error apiError `json:"error"`
}

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeErr(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errorEnvelope{Error: apiError{Code: code, Message: message}})
}

func writeError(w http.ResponseWriter, err error) {
	var se *statusError
	switch {
	case errors.As(err, &se):
		writeErr(w, se.status, se.code, se.message)
	case errors.Is(err, ErrNotReady):
		writeErr(w, http.StatusServiceUnavailable, codeDBNotReady,
			"no published database yet; run the load stage first")
	default:
		writeErr(w, http.StatusInternalServerError, codeInternal, "internal error")
	}
}
