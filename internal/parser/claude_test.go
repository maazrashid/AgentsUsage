package parser

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestClaudeParserDeduplicatesResponseSnapshots(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.jsonl")
	data := "" +
		`{"timestamp":"2026-08-29T08:00:00Z","requestId":"req-1","message":{"id":"msg-1","model":"claude-sonnet-4-6","usage":{"input_tokens":10,"output_tokens":2,"cache_read_input_tokens":20,"cache_creation_input_tokens":3}}}` + "\n" +
		`{"timestamp":"2026-08-29T08:00:01Z","requestId":"req-1","message":{"id":"msg-1","model":"claude-sonnet-4-6","usage":{"input_tokens":12,"output_tokens":5,"cache_read_input_tokens":20,"cache_creation_input_tokens":3}}}` + "\n"
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	candidates, diagnostics := parseClaudeFile(context.Background(), path)
	if diagnostics.ParseErrors != 0 {
		t.Fatalf("unexpected parse errors: %+v", diagnostics)
	}
	events := dedupeClaude(candidates)
	if len(events) != 1 {
		t.Fatalf("got %d events, want 1", len(events))
	}
	if got := events[0].ProcessedTokens(); got != 40 {
		t.Fatalf("processed tokens = %d, want 40", got)
	}
	if !events[0].PricingMatched || events[0].CostUSD <= 0 {
		t.Fatalf("expected priced event: %+v", events[0])
	}
}

func TestClaudeParserRequiresTokenFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.jsonl")
	if err := os.WriteFile(path, []byte(`{"timestamp":"2026-08-29T08:00:00Z","message":{"usage":{"input_tokens":10}}}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	events, diagnostics := parseClaudeFile(context.Background(), path)
	if len(events) != 0 || diagnostics.RecordsSkipped != 1 {
		t.Fatalf("unexpected result: %d %+v", len(events), diagnostics)
	}
}

func TestClaudeParserSkipsErrorsAndCountsSplitOnlyCacheWrites(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.jsonl")
	data := "" +
		`{"timestamp":"2026-08-29T08:00:00Z","isApiErrorMessage":true,"message":{"id":"bad","model":"claude-sonnet-4-6","usage":{"input_tokens":100,"output_tokens":20}}}` + "\n" +
		`{"timestamp":"2026-08-29T08:00:01Z","message":{"id":"good","model":"claude-sonnet-4-6","usage":{"input_tokens":10,"output_tokens":2,"cache_creation":{"ephemeral_1h_input_tokens":7,"ephemeral_5m_input_tokens":3}}}}` + "\n"
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	candidates, diagnostics := parseClaudeFile(context.Background(), path)
	if len(candidates) != 1 || diagnostics.RecordsSkipped != 1 {
		t.Fatalf("unexpected parse result: %d %+v", len(candidates), diagnostics)
	}
	if candidates[0].event.CacheWrite != 10 || candidates[0].event.ProcessedTokens() != 22 {
		t.Fatalf("unexpected split cache usage: %+v", candidates[0].event)
	}
}
