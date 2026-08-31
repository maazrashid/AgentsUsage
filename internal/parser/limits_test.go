package parser

import (
	"testing"
	"time"
)

func TestLiveQuotaSnapshotsPrefersLiveSourceOverNewerLog(t *testing.T) {
	now := time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)
	reset := now.Add(time.Hour)
	live := &QuotaSnapshot{
		Provider: ProviderCodex, ObservedAt: now.Add(-time.Minute), Source: "cli", Confidence: "live",
		Windows: []QuotaWindow{{Kind: "session", UsedPercent: 12, ResetsAt: &reset}},
	}
	local := &QuotaSnapshot{
		Provider: ProviderCodex, ObservedAt: now, Source: "local-log", Confidence: "last-observed",
		Windows: []QuotaWindow{{Kind: "session", UsedPercent: 80, ResetsAt: &reset}},
	}

	got := LiveQuotaSnapshots([]*QuotaSnapshot{local, live}, now)
	if len(got) != 1 || got[0].Source != "cli" || got[0].Windows[0].UsedPercent != 12 {
		t.Fatalf("unexpected selected quota: %+v", got)
	}
}

func TestLiveQuotaSnapshotsDropsExpiredWindows(t *testing.T) {
	now := time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)
	expired := now.Add(-time.Second)
	future := now.Add(time.Hour)
	snapshot := &QuotaSnapshot{
		Provider: ProviderClaude, ObservedAt: now, Source: "oauth", Confidence: "exact",
		Windows: []QuotaWindow{
			{Kind: "session", UsedPercent: 99, ResetsAt: &expired},
			{Kind: "weekly", UsedPercent: 40, ResetsAt: &future},
		},
	}

	got := LiveQuotaSnapshots([]*QuotaSnapshot{snapshot}, now)
	if len(got) != 1 || len(got[0].Windows) != 1 || got[0].Windows[0].Kind != "weekly" {
		t.Fatalf("unexpected live windows: %+v", got)
	}
}
