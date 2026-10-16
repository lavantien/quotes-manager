package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/lavantien/quotes-manager/internal/config"
	"github.com/lavantien/quotes-manager/internal/store"
)

const (
	apiCodeInvalidParam = "invalid_param"
	apiCodeNotFound     = "not_found"
	apiCodeInternal     = "internal_error"
)

type apiErrorEnvelope struct {
	Error apiErrorBody `json:"error"`
}

type apiErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeAPIJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeAPIErr(w http.ResponseWriter, status int, code, message string) {
	writeAPIJSON(w, status, apiErrorEnvelope{Error: apiErrorBody{Code: code, Message: message}})
}

type apiQuotesList struct {
	Rows  []store.Quote `json:"rows"`
	Total int           `json:"total"`
}

func (s *Server) apiQuotesListHandler(w http.ResponseWriter, r *http.Request) {
	limit, err := s.apiLimit(r)
	if err != nil {
		writeAPIErr(w, http.StatusBadRequest, apiCodeInvalidParam, "limit must be an integer")
		return
	}
	offset, err := apiOffset(r)
	if err != nil {
		writeAPIErr(w, http.StatusBadRequest, apiCodeInvalidParam, "offset must be an integer >= 0")
		return
	}
	qs, err := s.store.List()
	if err != nil {
		writeAPIErr(w, http.StatusInternalServerError, apiCodeInternal, "internal error")
		return
	}
	total := len(qs)
	start := min(offset, total)
	end := min(start+limit, total)
	rows := make([]store.Quote, 0, end-start)
	rows = append(rows, qs[start:end]...)
	writeAPIJSON(w, http.StatusOK, apiQuotesList{Rows: rows, Total: total})
}

func (s *Server) apiQuoteGetHandler(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeAPIErr(w, http.StatusNotFound, apiCodeNotFound, "no such quote")
		return
	}
	q, err := s.store.Get(id)
	if errors.Is(err, store.ErrNotFound) {
		writeAPIErr(w, http.StatusNotFound, apiCodeNotFound, "no such quote")
		return
	}
	if err != nil {
		writeAPIErr(w, http.StatusInternalServerError, apiCodeInternal, "internal error")
		return
	}
	writeAPIJSON(w, http.StatusOK, q)
}

func (s *Server) apiLimit(r *http.Request) (int, error) {
	raw := r.URL.Query().Get("limit")
	if raw == "" {
		return s.defaultLimit, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, err
	}
	if n <= 0 {
		return s.defaultLimit, nil
	}
	return min(n, s.maxLimit), nil
}

func apiOffset(r *http.Request) (int, error) {
	raw := r.URL.Query().Get("offset")
	if raw == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 0 {
		return 0, errors.New("offset must be an integer >= 0")
	}
	return n, nil
}

type Option func(*Server)

func WithLimits(defaultLimit, maxLimit int) Option {
	return func(s *Server) {
		s.defaultLimit = defaultLimit
		s.maxLimit = maxLimit
	}
}

func limitsFromConfig() (int, int) {
	srv := config.Defaults().Server
	return srv.DefaultLimit, srv.MaxLimit
}
