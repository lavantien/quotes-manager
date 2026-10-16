package server

import (
	"net/http"
	"strconv"
	"strings"
)

func (s *Server) queryLimit(r *http.Request) (int, error) {
	raw := r.URL.Query().Get("limit")
	if raw == "" {
		return s.cfg.Server.DefaultLimit, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, errInvalidParam("limit must be an integer")
	}
	if n <= 0 {
		return s.cfg.Server.DefaultLimit, nil
	}
	return min(n, s.cfg.Server.MaxLimit), nil
}

func queryOffset(r *http.Request) (int, error) {
	raw := r.URL.Query().Get("offset")
	if raw == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 0 {
		return 0, errInvalidParam("offset must be an integer >= 0")
	}
	return n, nil
}

func queryEnum(r *http.Request, name, def string, allowed ...string) (string, error) {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return def, nil
	}
	for _, a := range allowed {
		if raw == a {
			return raw, nil
		}
	}
	return "", errInvalidParam(name + " must be one of: " + strings.Join(allowed, ", "))
}

func queryFlag(r *http.Request, name string) (bool, error) {
	values, ok := r.URL.Query()[name]
	if !ok {
		return false, nil
	}
	raw := values[len(values)-1]
	if raw == "" {
		return true, nil
	}
	b, err := strconv.ParseBool(raw)
	if err != nil {
		return false, errInvalidParam(name + " must be a boolean")
	}
	return b, nil
}

func queryInt64(r *http.Request, name string, def int64) (int64, error) {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return def, nil
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, errInvalidParam(name + " must be an integer")
	}
	return n, nil
}

func requireParam(r *http.Request, name string) (string, error) {
	v := r.URL.Query().Get(name)
	if v == "" {
		return "", errInvalidParam(name + " is required")
	}
	return v, nil
}
