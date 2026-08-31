package quota

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestClaudeClientNormalizesModernQuotaWindows(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer test-token" {
			t.Fatalf("unexpected authorization header")
		}
		_ = json.NewEncoder(response).Encode(map[string]any{
			"limits": []any{
				map[string]any{"kind": "session", "group": "session", "percent": 42, "resets_at": "2026-08-29T12:00:00Z", "is_active": false},
				map[string]any{"kind": "weekly_all", "group": "weekly", "percent": 67, "resets_at": "2026-09-01T12:00:00Z", "is_active": false},
				map[string]any{"kind": "weekly_scoped", "group": "weekly", "percent": 81, "resets_at": "2026-09-01T12:00:00Z", "is_active": true, "scope": map[string]any{"model": map[string]any{"display_name": "Opus"}}},
			},
		})
	}))
	defer server.Close()

	credentials := filepath.Join(t.TempDir(), ".credentials.json")
	if err := os.WriteFile(credentials, []byte(`{"claudeAiOauth":{"accessToken":"test-token","expiresAt":9999999999999}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 29, 10, 0, 0, 0, time.UTC)
	client := NewClaudeClient(credentials)
	client.endpoint = server.URL
	client.now = func() time.Time { return now }
	snapshot, err := client.Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Source != "oauth" || snapshot.Confidence != "exact" || len(snapshot.Windows) != 3 {
		t.Fatalf("unexpected snapshot: %+v", snapshot)
	}
	if snapshot.Windows[0].Kind != "session" || snapshot.Windows[0].WindowMinutes != 300 {
		t.Fatalf("unexpected session window: %+v", snapshot.Windows[0])
	}
	if snapshot.Windows[2].Kind != "weekly_scoped" || snapshot.Windows[2].Label != "Opus" || !snapshot.Windows[2].Active {
		t.Fatalf("unexpected scoped window: %+v", snapshot.Windows[2])
	}
}

func TestClaudeClientTreatsMissingCredentialsAsUnavailable(t *testing.T) {
	client := NewClaudeClient(filepath.Join(t.TempDir(), "missing.json"))
	if _, err := client.Fetch(context.Background()); err != ErrUnavailable {
		t.Fatalf("error = %v, want ErrUnavailable", err)
	}
}

func TestClaudeClientFallsBackToLegacyQuotaShape(t *testing.T) {
	usage := claudeUsageResponse{
		FiveHour: &claudeLimit{Utilization: floatPointer(12)},
		SevenDay: &claudeLimit{Utilization: floatPointer(34)},
	}
	windows := normalizeClaudeWindows(usage)
	if len(windows) != 2 || windows[0].Label != "5h" || windows[1].Kind != "weekly" {
		t.Fatalf("unexpected legacy windows: %+v", windows)
	}
}

func floatPointer(value float64) *float64 { return &value }
