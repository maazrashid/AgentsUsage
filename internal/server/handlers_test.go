package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/maazrashid/AgentsUsage/internal/parser"
)

type fakeSource struct{ stats parser.Stats }

func (f fakeSource) Snapshot() parser.Stats { return f.stats }
func (fakeSource) LastError() error         { return nil }
func (fakeSource) LastRefresh() time.Time   { return time.Date(2026, 8, 29, 10, 0, 0, 0, time.UTC) }

func TestStatsEndpoint(t *testing.T) {
	s := &Server{startedAt: time.Now()}
	handler := s.Handler(fakeSource{stats: parser.Stats{AllTime: parser.Totals{ProcessedTokens: 42}}})
	request := httptest.NewRequest(http.MethodGet, "/api/stats", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
	var body parser.Stats
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.AllTime.ProcessedTokens != 42 {
		t.Fatalf("body = %+v", body)
	}
	if response.Header().Get("Content-Security-Policy") == "" {
		t.Fatal("missing CSP")
	}
}

func TestDashboardFallback(t *testing.T) {
	s := &Server{startedAt: time.Now()}
	request := httptest.NewRequest(http.MethodGet, "/not-a-real-route", nil)
	response := httptest.NewRecorder()
	s.Handler(fakeSource{}).ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
	if contentType := response.Header().Get("Content-Type"); contentType != "text/html; charset=utf-8" {
		t.Fatalf("content type = %q", contentType)
	}
}
