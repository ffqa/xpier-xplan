// Package server exposes the xplan HTTP API and serves the embedded UI.
package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"path"
	"runtime/debug"
	"strings"
	"time"

	"github.com/ffqa/xplan/internal/app"
	"github.com/ffqa/xplan/internal/console"
	"github.com/ffqa/xplan/internal/project"
	"github.com/ffqa/xplan/internal/review"
	"github.com/ffqa/xplan/internal/scanner"
)

// maxBodyBytes caps JSON request bodies.
const maxBodyBytes = 1 << 20

// Server holds the dependencies of the HTTP handlers.
type Server struct {
	app     *app.App
	console *console.Console
	static  fs.FS
	logger  *log.Logger
}

// New builds the full handler chain. static may be nil (API only).
func New(application *app.App, static fs.FS) http.Handler {
	srv := &Server{app: application, console: &console.Console{App: application}, static: static, logger: application.Logger}
	mux := http.NewServeMux()
	srv.routes(mux)
	return srv.recoverPanics(srv.guard(srv.logRequests(mux)))
}

func (s *Server) routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/meta", s.handleMeta)
	mux.HandleFunc("GET /api/config", s.handleGetConfig)
	mux.HandleFunc("PATCH /api/config", s.handlePatchConfig)
	mux.HandleFunc("GET /api/overview", s.handleOverview)
	mux.HandleFunc("GET /api/inbox", s.handleInbox)
	mux.HandleFunc("GET /api/reviews/active", s.handleActiveReviews)
	mux.HandleFunc("POST /api/discover", s.handleDiscover)
	mux.HandleFunc("POST /api/projects", s.handleAddProject)

	mux.HandleFunc("GET /api/projects/{slug}", s.handleGetProject)
	mux.HandleFunc("PATCH /api/projects/{slug}", s.handlePatchProject)
	mux.HandleFunc("DELETE /api/projects/{slug}", s.handleDeleteProject)
	mux.HandleFunc("GET /api/projects/{slug}/brief", s.handleBrief)
	mux.HandleFunc("POST /api/projects/{slug}/scan", s.handleScan)
	mux.HandleFunc("POST /api/projects/{slug}/visit", s.handleVisit)

	mux.HandleFunc("GET /api/projects/{slug}/docs", s.handleDocs)
	mux.HandleFunc("GET /api/projects/{slug}/file", s.handleFile)
	mux.HandleFunc("GET /api/projects/{slug}/tree", s.handleTree)

	mux.HandleFunc("GET /api/projects/{slug}/intent", s.handleGetIntent)
	mux.HandleFunc("PUT /api/projects/{slug}/intent", s.handlePutIntent)
	mux.HandleFunc("GET /api/projects/{slug}/adrs", s.handleListADRs)
	mux.HandleFunc("POST /api/projects/{slug}/adrs", s.handleCreateADR)
	mux.HandleFunc("PUT /api/projects/{slug}/adrs/{id}", s.handleUpdateADR)
	mux.HandleFunc("GET /api/projects/{slug}/journal", s.handleListJournal)
	mux.HandleFunc("POST /api/projects/{slug}/journal", s.handleAddJournal)

	mux.HandleFunc("GET /api/projects/{slug}/reviews", s.handleListReviews)
	mux.HandleFunc("POST /api/projects/{slug}/reviews", s.handleStartReview)
	mux.HandleFunc("GET /api/projects/{slug}/reviews/{id}", s.handleGetReview)
	mux.HandleFunc("POST /api/projects/{slug}/reviews/{id}/cancel", s.handleCancelReview)

	mux.HandleFunc("GET /api/projects/{slug}/console", s.handleConsoleHistory)
	mux.HandleFunc("POST /api/projects/{slug}/console", s.handleConsoleRun)

	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "unknown endpoint"})
	})
	mux.HandleFunc("/", s.handleStatic)
}

// guard rejects requests that could come from another website: the Host
// header must be loopback (DNS rebinding) and state-changing requests must
// carry X-Xplan: 1, which a cross-site form cannot set.
func (s *Server) guard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !loopbackHost(r.Host) {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "xplan only answers on localhost"})
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead && r.Header.Get("X-Xplan") != "1" {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "missing X-Xplan header"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func loopbackHost(hostHeader string) bool {
	host := hostHeader
	if splitHost, _, err := net.SplitHostPort(hostHeader); err == nil {
		host = splitHost
	}
	host = strings.Trim(host, "[]")
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (s *Server) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/") {
			next.ServeHTTP(w, r)
			return
		}
		started := time.Now()
		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(recorder, r)
		if recorder.status >= 400 || r.Method != http.MethodGet {
			s.logger.Printf("http %s %s -> %d (%s)", r.Method, r.URL.Path, recorder.status, time.Since(started).Round(time.Millisecond))
		}
	})
}

func (s *Server) recoverPanics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				s.logger.Printf("panic in %s %s: %v\n%s", r.Method, r.URL.Path, recovered, debug.Stack())
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func (s *Server) handleStatic(w http.ResponseWriter, r *http.Request) {
	if s.static == nil {
		http.Error(w, "UI not built. Run `make web`.", http.StatusNotFound)
		return
	}
	cleaned := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
	if cleaned == "" {
		cleaned = "index.html"
	}
	if info, err := fs.Stat(s.static, cleaned); err != nil || info.IsDir() {
		cleaned = "index.html" // SPA fallback
	}
	content, err := fs.ReadFile(s.static, cleaned)
	if err != nil {
		http.Error(w, "UI not built. Run `make web`.", http.StatusNotFound)
		return
	}
	if strings.HasPrefix(cleaned, "assets/") {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		w.Header().Set("Cache-Control", "no-cache")
	}
	http.ServeContent(w, r, cleaned, time.Time{}, strings.NewReader(string(content)))
}

// writeJSON encodes v with the given status.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("encode response: %v", err)
	}
}

// writeError maps domain errors to HTTP status codes.
func (s *Server) writeError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, project.ErrNotFound), errors.Is(err, review.ErrJobNotFound), errors.Is(err, os.ErrNotExist):
		status = http.StatusNotFound
	case errors.Is(err, project.ErrInvalid):
		status = http.StatusBadRequest
	case errors.Is(err, review.ErrAlreadyActive):
		status = http.StatusConflict
	case errors.Is(err, scanner.ErrOutsideRoot), errors.Is(err, scanner.ErrSensitive):
		status = http.StatusForbidden
	case errors.Is(err, scanner.ErrNotText):
		status = http.StatusUnsupportedMediaType
	}
	if status == http.StatusInternalServerError {
		s.logger.Printf("internal error: %v", err)
	}
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

// decodeBody parses a JSON body into target.
func decodeBody(r *http.Request, target any) error {
	decoder := json.NewDecoder(http.MaxBytesReader(nil, r.Body, maxBodyBytes))
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("%w: invalid JSON body: %v", project.ErrInvalid, err)
	}
	return nil
}
