package quota

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"
)

func TestParseCodexRateLimits(t *testing.T) {
	observedAt := time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)
	result := json.RawMessage(`{"rateLimits":{"limitName":"Codex","primary":{"usedPercent":80,"windowDurationMins":300,"resetsAt":1788016102},"secondary":{"usedPercent":69,"windowDurationMins":10080,"resetsAt":1788455666}}}`)
	snapshot, err := parseCodexRateLimits(result, observedAt)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Source != "cli" || snapshot.Confidence != "live" || len(snapshot.Windows) != 2 {
		t.Fatalf("unexpected snapshot: %+v", snapshot)
	}
	if snapshot.Windows[0].Kind != "session" || snapshot.Windows[0].UsedPercent != 80 {
		t.Fatalf("unexpected primary window: %+v", snapshot.Windows[0])
	}
	if snapshot.Windows[1].Kind != "weekly" || snapshot.Windows[1].UsedPercent != 69 {
		t.Fatalf("unexpected secondary window: %+v", snapshot.Windows[1])
	}
}

func TestInstalledCodexClient(t *testing.T) {
	if os.Getenv("AGENTSUSAGE_TEST_CODEX_CLI") == "" {
		t.Skip("set AGENTSUSAGE_TEST_CODEX_CLI=1 to query the installed Codex CLI")
	}
	client := NewCodexClient()
	snapshot, err := client.Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Provider != "codex" || len(snapshot.Windows) == 0 {
		t.Fatalf("unexpected installed Codex snapshot: %+v", snapshot)
	}
}
