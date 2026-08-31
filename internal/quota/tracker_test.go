package quota

import (
	"context"
	"testing"
	"time"

	"github.com/maazrashid/AgentsUsage/internal/parser"
)

type fakeUsageSource struct{ stats parser.Stats }

func (f *fakeUsageSource) Snapshot() parser.Stats        { return f.stats }
func (f *fakeUsageSource) Refresh(context.Context) error { return nil }
func (f *fakeUsageSource) LastError() error              { return nil }
func (f *fakeUsageSource) LastRefresh() time.Time        { return time.Time{} }

type fakeClaudeFetcher struct {
	snapshot *parser.QuotaSnapshot
	err      error
}

func (f fakeClaudeFetcher) Fetch(context.Context) (*parser.QuotaSnapshot, error) {
	return f.snapshot, f.err
}

func TestTrackerMergesClaudeAndCodexQuota(t *testing.T) {
	reset := time.Now().Add(time.Hour)
	base := &fakeUsageSource{stats: parser.Stats{Quotas: []parser.QuotaSnapshot{{
		Provider: parser.ProviderCodex, ObservedAt: time.Now(), Source: "local-log",
		Windows: []parser.QuotaWindow{{Kind: "session", UsedPercent: 20, ResetsAt: &reset}},
	}}}}
	claude := &parser.QuotaSnapshot{
		Provider: parser.ProviderClaude, ObservedAt: time.Now(), Source: "oauth",
		Windows: []parser.QuotaWindow{{Kind: "weekly", UsedPercent: 40, ResetsAt: &reset}},
	}
	tracker := NewTracker(base, fakeClaudeFetcher{snapshot: claude}, nil, time.Minute)
	tracker.refreshQuotas(context.Background())
	stats := tracker.Snapshot()
	if len(stats.Quotas) != 2 || stats.Quotas[0].Provider != parser.ProviderClaude || stats.Quotas[1].Provider != parser.ProviderCodex {
		t.Fatalf("unexpected merged quotas: %+v", stats.Quotas)
	}
}
