package parser

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestCodexParserUsesLastUsageAndSuppressesReplay(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rollout.jsonl")
	data := "" +
		`{"timestamp":"2026-08-29T09:00:00Z","type":"turn_context","payload":{"model":"gpt-5.6-sol","effort":"high"}}` + "\n" +
		`{"timestamp":"2026-08-29T09:00:01Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":1000,"cached_input_tokens":700,"output_tokens":200,"reasoning_output_tokens":50,"total_tokens":1200},"last_token_usage":{"input_tokens":100,"cached_input_tokens":70,"output_tokens":20,"reasoning_output_tokens":5,"total_tokens":120}}}}` + "\n" +
		`{"timestamp":"2026-08-29T09:00:02Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":1000,"cached_input_tokens":700,"output_tokens":200,"reasoning_output_tokens":50,"total_tokens":1200},"last_token_usage":{"input_tokens":100,"cached_input_tokens":70,"output_tokens":20,"reasoning_output_tokens":5,"total_tokens":120}}}}` + "\n"
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	events, diagnostics := parseCodexFile(context.Background(), path)
	if diagnostics.ParseErrors != 0 {
		t.Fatalf("unexpected diagnostics: %+v", diagnostics)
	}
	if len(events) != 1 {
		t.Fatalf("got %d events, want 1", len(events))
	}
	event := events[0]
	if event.InputTokens != 100 || event.CacheRead != 70 || event.OutputTokens != 20 || event.Reasoning != 5 {
		t.Fatalf("unexpected token vector: %+v", event)
	}
	if event.ProcessedTokens() != 120 {
		t.Fatalf("processed = %d, want 120", event.ProcessedTokens())
	}
}

func TestCodexParserFallsBackToCumulativeDelta(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rollout.jsonl")
	data := "" +
		`{"timestamp":"2026-08-29T09:00:00Z","type":"turn_context","payload":{"model":"gpt-5.4"}}` + "\n" +
		`{"timestamp":"2026-08-29T09:00:01Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":100,"cached_input_tokens":60,"output_tokens":20,"reasoning_output_tokens":5,"total_tokens":120}}}}` + "\n" +
		`{"timestamp":"2026-08-29T09:00:02Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":160,"cached_input_tokens":90,"output_tokens":30,"reasoning_output_tokens":8,"total_tokens":190}}}}` + "\n"
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	events, _ := parseCodexFile(context.Background(), path)
	if len(events) != 2 {
		t.Fatalf("got %d events, want 2", len(events))
	}
	if events[1].InputTokens != 60 || events[1].OutputTokens != 10 || events[1].CacheRead != 30 {
		t.Fatalf("unexpected delta: %+v", events[1])
	}
}

func TestCodexInvalidTimestampDoesNotAdvanceHighWater(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rollout.jsonl")
	data := "" +
		`{"timestamp":"not-a-time","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":100,"output_tokens":20}}}}` + "\n" +
		`{"timestamp":"2026-08-29T09:00:02Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":160,"output_tokens":30}}}}` + "\n"
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	events, _ := parseCodexFile(context.Background(), path)
	if len(events) != 1 {
		t.Fatalf("got %d events, want 1", len(events))
	}
	if events[0].InputTokens != 160 || events[0].OutputTokens != 30 {
		t.Fatalf("unexpected baseline: %+v", events[0])
	}
}

func TestParseTimestampRejectsInvalidNumbers(t *testing.T) {
	for _, value := range []any{json.Number("NaN"), json.Number("-1"), json.Number("9999999999999999")} {
		if _, ok := parseTimestamp(value); ok {
			t.Fatalf("accepted invalid timestamp %v", value)
		}
	}
}

func TestCodexParserCapturesFiveHourAndWeeklyLimits(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rollout.jsonl")
	line := `{"timestamp":"2026-08-29T09:00:01Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":100,"output_tokens":20}},"rate_limits":{"limit_id":"codex-main","limit_name":"Codex","primary":{"used_percent":42,"window_minutes":300,"resets_at":1787997600},"secondary":{"used_percent":67,"window_minutes":10080,"resets_at":1788500000}}}}` + "\n"
	if err := os.WriteFile(path, []byte(line), 0o600); err != nil {
		t.Fatal(err)
	}
	state := codexState{signatures: make(map[string]string)}
	_, _, state, diagnostics, _, err := parseCodexTail(context.Background(), path, streamCursor{}, state)
	if err != nil || diagnostics.ParseErrors != 0 {
		t.Fatalf("parse failed: %v %+v", err, diagnostics)
	}
	if state.quota == nil || len(state.quota.Windows) != 2 {
		t.Fatalf("quota not captured: %+v", state.quota)
	}
	if state.quota.Windows[0].Kind != "session" || state.quota.Windows[0].UsedPercent != 42 {
		t.Fatalf("unexpected session window: %+v", state.quota.Windows[0])
	}
	if state.quota.Windows[1].Kind != "weekly" || state.quota.Windows[1].WindowMinutes != 10080 {
		t.Fatalf("unexpected weekly window: %+v", state.quota.Windows[1])
	}
}
