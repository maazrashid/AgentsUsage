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
	if _, err := index.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	first := index.claude[path].candidates
	if len(first) != 1 {
		t.Fatalf("initial candidates = %d", len(first))
	}
	if _, err := index.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	second := index.claude[path].candidates
	if len(second) != 1 || &first[0] != &second[0] {
		t.Fatal("unchanged file was reparsed instead of reused")
	}
}
