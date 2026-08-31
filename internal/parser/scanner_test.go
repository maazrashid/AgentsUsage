package parser

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestScanAggregatesBothProvidersAndAllowlistedCodexRoots(t *testing.T) {
	root := t.TempDir()
	claude := filepath.Join(root, "claude")
	codex := filepath.Join(root, "codex")
	if err := os.MkdirAll(claude, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(codex, "sessions", "2026", "08"), 0o700); err != nil {
		t.Fatal(err)
	}
	claudeLine := `{"timestamp":"2026-08-29T08:00:00Z","requestId":"r","message":{"id":"m","model":"claude-sonnet-4-6","usage":{"input_tokens":10,"output_tokens":5,"cache_read_input_tokens":20}}}` + "\n"
	codexLines := `{"timestamp":"2026-08-29T09:00:00Z","type":"turn_context","payload":{"model":"gpt-5.4"}}` + "\n" +
		`{"timestamp":"2026-08-29T09:00:01Z","type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":100,"cached_input_tokens":80,"output_tokens":20,"reasoning_output_tokens":4}}}}` + "\n"
	if err := os.WriteFile(filepath.Join(claude, "a.jsonl"), []byte(claudeLine), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(codex, "sessions", "2026", "08", "b.jsonl"), []byte(codexLines), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(codex, "auth.jsonl"), []byte(codexLines), 0o600); err != nil {
		t.Fatal(err)
	}

	now := time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)
	stats, err := Scan(context.Background(), ScanOptions{ClaudeRoot: claude, CodexRoot: codex, Now: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	if stats.AllTime.ProcessedTokens != 155 {
		t.Fatalf("processed = %d, want 155", stats.AllTime.ProcessedTokens)
	}
	if stats.Diagnostics.FilesScanned != 2 {
		t.Fatalf("files scanned = %d, want 2", stats.Diagnostics.FilesScanned)
	}
	if len(stats.Providers) != 2 {
		t.Fatalf("providers = %+v", stats.Providers)
	}
	if len(stats.Consumption) != 2 {
		t.Fatalf("consumption providers = %+v", stats.Consumption)
	}
	if stats.Consumption[0].Provider != ProviderClaude || stats.Consumption[0].Today.Totals.UsageRecords != 1 || len(stats.Consumption[0].Today.Models) != 1 {
		t.Fatalf("unexpected Claude consumption: %+v", stats.Consumption[0])
	}
	if stats.Consumption[1].Provider != ProviderCodex || stats.Consumption[1].Last30Days.Totals.ProcessedTokens != 120 || len(stats.Consumption[1].AllTime.Timeline) != 1 {
		t.Fatalf("unexpected Codex consumption: %+v", stats.Consumption[1])
	}
}

func TestCodexRootsRejectEmptyPath(t *testing.T) {
	if roots := codexRoots(""); len(roots) != 0 {
		t.Fatalf("empty root produced %v", roots)
	}
}

func TestIndexerReusesUnchangedParsedFiles(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "a.jsonl")
	line := `{"timestamp":"2026-08-29T08:00:00Z","requestId":"r","message":{"id":"m","model":"claude-sonnet-4-6","usage":{"input_tokens":10,"output_tokens":5}}}` + "\n"
	if err := os.WriteFile(path, []byte(line), 0o600); err != nil {
		t.Fatal(err)
	}
	index := NewIndexer(ScanOptions{ClaudeRoot: root})
	firstStats, err := index.Refresh(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if firstStats.Diagnostics.BytesRead != int64(len(line)) {
		t.Fatalf("initial bytes read = %d, want %d", firstStats.Diagnostics.BytesRead, len(line))
	}
	first := index.claude[path].candidates
	if len(first) != 1 {
		t.Fatalf("initial candidates = %d", len(first))
	}
	secondStats, err := index.Refresh(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if secondStats.Diagnostics.BytesRead != 0 {
		t.Fatalf("unchanged bytes read = %d, want 0", secondStats.Diagnostics.BytesRead)
	}
	second := index.claude[path].candidates
	if len(second) != 1 || &first[0] != &second[0] {
		t.Fatal("unchanged file was reparsed instead of reused")
	}
}

func TestIndexerTailsClaudeAppendAndDeduplicatesSnapshots(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "session.jsonl")
	firstLine := `{"timestamp":"2026-08-29T08:00:00Z","requestId":"req-1","message":{"id":"msg-1","model":"claude-sonnet-4-6","usage":{"input_tokens":10,"output_tokens":5}}}` + "\n"
	secondLine := `{"timestamp":"2026-08-29T08:00:01Z","requestId":"req-1","message":{"id":"msg-1","model":"claude-sonnet-4-6","usage":{"input_tokens":12,"output_tokens":7}}}` + "\n"
	if err := os.WriteFile(path, []byte(firstLine), 0o600); err != nil {
		t.Fatal(err)
	}
	index := NewIndexer(ScanOptions{ClaudeRoot: root})
	first, err := index.Refresh(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if first.AllTime.ProcessedTokens != 15 {
		t.Fatalf("initial processed tokens = %d, want 15", first.AllTime.ProcessedTokens)
	}
	appendText(t, path, secondLine)
	second, err := index.Refresh(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if second.Diagnostics.BytesRead != int64(len(secondLine)) {
		t.Fatalf("append bytes read = %d, want %d", second.Diagnostics.BytesRead, len(secondLine))
	}
	if second.Diagnostics.RecordsParsed != 2 || second.AllTime.ProcessedTokens != 19 {
		t.Fatalf("unexpected appended stats: %+v", second)
	}
}

func TestIndexerCarriesPartialClaudeLineAcrossRefreshes(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "session.jsonl")
	line := `{"timestamp":"2026-08-29T08:00:00Z","requestId":"r","message":{"id":"m","model":"claude-sonnet-4-6","usage":{"input_tokens":10,"output_tokens":5}}}`
	split := len(line) / 2
	if err := os.WriteFile(path, []byte(line[:split]), 0o600); err != nil {
		t.Fatal(err)
	}
	index := NewIndexer(ScanOptions{ClaudeRoot: root})
	first, err := index.Refresh(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if first.Diagnostics.ParseErrors != 0 || first.Diagnostics.RecordsParsed != 0 {
		t.Fatalf("partial line produced diagnostics: %+v", first.Diagnostics)
	}
	appendText(t, path, line[split:]+"\n")
	second, err := index.Refresh(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if second.Diagnostics.BytesRead != int64(len(line)-split+1) {
		t.Fatalf("completion bytes read = %d, want %d", second.Diagnostics.BytesRead, len(line)-split+1)
	}
	if second.Diagnostics.RecordsParsed != 1 || second.AllTime.ProcessedTokens != 15 {
		t.Fatalf("completed line was not parsed: %+v", second)
	}
}

func TestIndexerPreservesCodexStateAcrossAppend(t *testing.T) {
	root := filepath.Join(t.TempDir(), "sessions")
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "rollout.jsonl")
	initial := `{"timestamp":"2026-08-29T09:00:00Z","type":"turn_context","payload":{"model":"gpt-5.4"}}` + "\n" +
		`{"timestamp":"2026-08-29T09:00:01Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":100,"output_tokens":20}}}}` + "\n"
	appended := `{"timestamp":"2026-08-29T09:00:02Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":160,"output_tokens":30}}}}` + "\n"
	if err := os.WriteFile(path, []byte(initial), 0o600); err != nil {
		t.Fatal(err)
	}
	index := NewIndexer(ScanOptions{CodexRoot: root})
	first, err := index.Refresh(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if first.AllTime.ProcessedTokens != 120 {
		t.Fatalf("initial processed tokens = %d, want 120", first.AllTime.ProcessedTokens)
	}
	appendText(t, path, appended)
	second, err := index.Refresh(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if second.Diagnostics.BytesRead != int64(len(appended)) {
		t.Fatalf("append bytes read = %d, want %d", second.Diagnostics.BytesRead, len(appended))
	}
	if second.AllTime.ProcessedTokens != 190 {
		t.Fatalf("cumulative processed tokens = %d, want 190", second.AllTime.ProcessedTokens)
	}
	if len(second.Models) != 1 || second.Models[0].Key != "gpt-5.4" {
		t.Fatalf("Codex model state was not preserved: %+v", second.Models)
	}
}

func TestIndexerRebuildsOnTruncateAndReplacement(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "session.jsonl")
	large := `{"timestamp":"2026-08-29T08:00:00Z","requestId":"large-request","message":{"id":"large-message","model":"claude-sonnet-4-6","usage":{"input_tokens":100,"output_tokens":50}}}` + "\n"
	small := `{"timestamp":"2026-08-29T08:00:00Z","message":{"id":"s","model":"claude-sonnet-4-6","usage":{"input_tokens":2,"output_tokens":1}}}` + "\n"
	if err := os.WriteFile(path, []byte(large), 0o600); err != nil {
		t.Fatal(err)
	}
	index := NewIndexer(ScanOptions{ClaudeRoot: root})
	if _, err := index.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(small), 0o600); err != nil {
		t.Fatal(err)
	}
	truncated, err := index.Refresh(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if truncated.Diagnostics.BytesRead != int64(len(small)) || truncated.AllTime.ProcessedTokens != 3 {
		t.Fatalf("truncate did not rebuild the file: %+v", truncated)
	}

	replacement := `{"timestamp":"2026-08-29T08:00:00Z","message":{"id":"r","model":"claude-opus-4-1","usage":{"input_tokens":4,"output_tokens":2}}}` + "\n"
	temporary := filepath.Join(root, "replacement.tmp")
	if err := os.WriteFile(temporary, []byte(replacement), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(temporary, path); err != nil {
		t.Fatal(err)
	}
	replaced, err := index.Refresh(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if replaced.Diagnostics.BytesRead != int64(len(replacement)) || replaced.AllTime.ProcessedTokens != 6 {
		t.Fatalf("replacement did not rebuild the file: %+v", replaced)
	}
}

func TestIndexerRebuildsOnSameFileRewriteThatGrows(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "session.jsonl")
	initial := `{"timestamp":"2026-08-29T08:00:00Z","message":{"id":"a","model":"claude-sonnet-4-6","usage":{"input_tokens":2,"output_tokens":1}}}` + "\n"
	rewritten := `{"timestamp":"2026-08-29T08:00:00Z","requestId":"larger-rewrite","message":{"id":"b","model":"claude-opus-4-1","usage":{"input_tokens":40,"output_tokens":20,"cache_read_input_tokens":10}}}` + "\n"
	if len(rewritten) <= len(initial) {
		t.Fatal("test fixture must grow the file")
	}
	if err := os.WriteFile(path, []byte(initial), 0o600); err != nil {
		t.Fatal(err)
	}
	index := NewIndexer(ScanOptions{ClaudeRoot: root})
	if _, err := index.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(rewritten), 0o600); err != nil {
		t.Fatal(err)
	}
	stats, err := index.Refresh(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if stats.Diagnostics.BytesRead != int64(len(rewritten)) || stats.AllTime.ProcessedTokens != 70 {
		t.Fatalf("growing rewrite was treated as an append: %+v", stats)
	}
	if stats.Diagnostics.RecordsParsed != 1 || len(stats.Models) != 1 || stats.Models[0].Key != "claude-opus-4-1" {
		t.Fatalf("old rewrite state remained indexed: %+v", stats)
	}
}

func TestIndexerDropsDeletedFiles(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "session.jsonl")
	line := `{"timestamp":"2026-08-29T08:00:00Z","message":{"id":"m","model":"claude-sonnet-4-6","usage":{"input_tokens":10,"output_tokens":5}}}` + "\n"
	if err := os.WriteFile(path, []byte(line), 0o600); err != nil {
		t.Fatal(err)
	}
	index := NewIndexer(ScanOptions{ClaudeRoot: root})
	if _, err := index.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	stats, err := index.Refresh(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if stats.AllTime.ProcessedTokens != 0 || stats.Diagnostics.FilesScanned != 0 || stats.Diagnostics.BytesRead != 0 {
		t.Fatalf("deleted file remained indexed: %+v", stats)
	}
}

func appendText(t *testing.T, path, value string) {
	t.Helper()
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString(value); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}
