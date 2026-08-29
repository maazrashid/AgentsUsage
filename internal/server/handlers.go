package server

import (
	"encoding/json"
	"io/fs"
	"net/http"
	"strings"
	"time"

	"github.com/maazrashid/AgentsUsage/internal/parser"
	appweb "github.com/maazrashid/AgentsUsage/web"
)

type DataSource interface {
	Snapshot() parser.Stats
	LastError() error
	LastRefresh() time.Time
}

func (s *Server) Handler(source DataSource) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/status", func(response http.ResponseWriter, request *http.Request) {
		lastError := ""
		if err := source.LastError(); err != nil {
			lastError = err.Error()
		}
		writeJSON(response, http.StatusOK, map[string]any{
			"active":        true,
			"uptimeSeconds": int64(time.Since(s.startedAt).Seconds()),
			"lastRefresh":   source.LastRefresh(),
			"lastError":     lastError,
		})
	})
	mux.HandleFunc("GET /api/stats", func(response http.ResponseWriter, request *http.Request) {
		writeJSON(response, http.StatusOK, source.Snapshot())
	})

	static := http.FileServer(http.FS(appweb.Static))
	mux.HandleFunc("GET /", func(response http.ResponseWriter, request *http.Request) {
		path := strings.TrimPrefix(request.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}
		if _, err := fs.Stat(appweb.Static, path); err != nil {
			path = "index.html"
		}
		if path == "index.html" {
			request.URL.Path = "/"
			response.Header().Set("Cache-Control", "no-cache")
		} else {
			request.URL.Path = "/" + path
		}
		static.ServeHTTP(response, request)
	})
	return securityHeaders(mux)
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("X-Content-Type-Options", "nosniff")
		response.Header().Set("Referrer-Policy", "no-referrer")
		response.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'")
		next.ServeHTTP(response, request)
	})
}

func writeJSON(response http.ResponseWriter, status int, value any) {
	response.Header().Set("Content-Type", "application/json; charset=utf-8")
	response.Header().Set("Cache-Control", "no-store")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(value)
}
