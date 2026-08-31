package tray

import (
	"testing"

	"github.com/maazrashid/AgentsUsage/internal/parser"
)

func TestCompactTokens(t *testing.T) {
	tests := map[int64]string{0: "0", 999: "999", 1_250: "1.2K", 2_500_000: "2.5M", 3_100_000_000: "3.1B"}
	for input, expected := range tests {
		if got := compactTokens(input); got != expected {
			t.Fatalf("compactTokens(%d) = %q, want %q", input, got, expected)
		}
	}
}

func TestFormatQuotaSummary(t *testing.T) {
	snapshots := []parser.QuotaSnapshot{{
		Provider: parser.ProviderClaude,
		Windows: []parser.QuotaWindow{
			{Kind: "weekly_scoped", UsedPercent: 91},
			{Kind: "session", UsedPercent: 55.6},
			{Kind: "weekly", UsedPercent: 44.5},
		},
	}}
	if got := formatQuotaSummary(parser.ProviderClaude, snapshots); got != "Claude: 5h 56% · wk 44%" {
		t.Fatalf("formatQuotaSummary() = %q", got)
	}
	if got := formatQuotaSummary(parser.ProviderCodex, snapshots); got != "Codex: usage limits unavailable" {
		t.Fatalf("missing provider = %q", got)
	}
}
