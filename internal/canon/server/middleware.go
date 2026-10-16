package server

import (
	"net/http"
	"net/url"
	"runtime/debug"
	"strings"
	"time"
)

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	if r.status == 0 {
		r.status = code
	}
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.WriteHeader(http.StatusOK)
	}
	return r.ResponseWriter.Write(b)
}

type headerRecorder struct {
	http.ResponseWriter
	wrote bool
}

func (r *headerRecorder) WriteHeader(code int) {
	r.inject()
	r.ResponseWriter.WriteHeader(code)
}

func (r *headerRecorder) Write(b []byte) (int, error) {
	r.inject()
	return r.ResponseWriter.Write(b)
}

func (r *headerRecorder) inject() {
	if r.wrote {
		return
	}
	r.wrote = true
	h := r.Header()
	h.Set("X-Content-Type-Options", "nosniff")
	if strings.HasPrefix(h.Get("Content-Type"), "text/html") {
		h.Set("Content-Security-Policy", "default-src 'self'")
	}
}

func (s *Server) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(&headerRecorder{ResponseWriter: w}, r)
	})
}

func (s *Server) recoverPanic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				s.log.Error("panic serving request",
					"method", r.Method, "path", r.URL.Path,
					"panic", v, "stack", string(debug.Stack()))
				if strings.HasPrefix(r.URL.Path, "/api/") {
					writeErr(w, http.StatusInternalServerError, codeInternal, "internal error")
				} else {
					w.Header().Set("Content-Type", "text/plain; charset=utf-8")
					w.WriteHeader(http.StatusInternalServerError)
					_, _ = w.Write([]byte("internal error\n"))
				}
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func (s *Server) checkOrigin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			if origin := r.Header.Get("Origin"); origin != "" {
				u, err := url.Parse(origin)
				if err != nil || u.Host != r.Host {
					writeErr(w, http.StatusForbidden, codeForbidden,
						"cross-origin POST rejected")
					return
				}
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w}
		next.ServeHTTP(rec, r)
		status := rec.status
		if status == 0 {
			status = http.StatusOK
		}
		s.log.Info("request",
			"method", r.Method, "path", r.URL.Path,
			"status", status, "duration_ms", time.Since(start).Milliseconds())
	})
}

func (s *Server) apiNotFound(w http.ResponseWriter, _ *http.Request) {
	writeErr(w, http.StatusNotFound, codeNotFound, "no such api route")
}
