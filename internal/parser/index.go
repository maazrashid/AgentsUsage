package parser

import (
	"context"
	"errors"
	"os"
	"sort"
	"time"
)

type fileStamp struct {
	size    int64
	modTime int64
}

type claudeCacheEntry struct {
	stamp       fileStamp
	candidates  []claudeCandidate
	diagnostics Diagnostics
}

type codexCacheEntry struct {
	stamp       fileStamp
	events      []Event
	diagnostics Diagnostics
}

// Indexer retains parsed per-file aggregates between refreshes. Directory
// metadata is still checked on each fallback interval, but unchanged JSONL
// bodies are never reopened or reparsed.
type Indexer struct {
	options ScanOptions
	claude  map[string]claudeCacheEntry
	codex   map[string]codexCacheEntry
}

func NewIndexer(options ScanOptions) *Indexer {
	return &Indexer{
		options: options,
		claude:  make(map[string]claudeCacheEntry),
		codex:   make(map[string]codexCacheEntry),
	}
}

func (i *Indexer) Refresh(ctx context.Context) (Stats, error) {
	now := time.Now
	if i.options.Now != nil {
		now = i.options.Now
	}
	var refreshErrors []error

	claudeFiles, err := jsonlFiles(i.options.ClaudeRoot)
	if err != nil {
		refreshErrors = append(refreshErrors, errors.New("Claude log discovery failed"))
	}
	i.refreshClaude(ctx, claudeFiles, &refreshErrors)

	var codexFiles []string
	for _, root := range codexRoots(i.options.CodexRoot) {
		files, discoverErr := jsonlFiles(root)
		if discoverErr != nil {
			refreshErrors = append(refreshErrors, errors.New("Codex log discovery failed"))
			continue
		}
		codexFiles = append(codexFiles, files...)
	}
	i.refreshCodex(ctx, codexFiles, &refreshErrors)
	if err := ctx.Err(); err != nil {
		return Stats{}, err
	}

	var candidates []claudeCandidate
	var events []Event
	var diagnostics Diagnostics
	for _, path := range sortedClaudeCacheKeys(i.claude) {
		cached := i.claude[path]
		candidates = append(candidates, cached.candidates...)
		mergeDiagnostics(&diagnostics, cached.diagnostics)
	}
	events = append(events, dedupeClaude(candidates)...)
	for _, path := range sortedCodexCacheKeys(i.codex) {
		cached := i.codex[path]
		events = append(events, cached.events...)
		mergeDiagnostics(&diagnostics, cached.diagnostics)
	}
	stats := aggregate(events, now())
	stats.Diagnostics = diagnostics
	return stats, errors.Join(refreshErrors...)
}

func sortedClaudeCacheKeys(cache map[string]claudeCacheEntry) []string {
	keys := make([]string, 0, len(cache))
	for key := range cache {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortedCodexCacheKeys(cache map[string]codexCacheEntry) []string {
	keys := make([]string, 0, len(cache))
	for key := range cache {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func (i *Indexer) refreshClaude(ctx context.Context, files []string, refreshErrors *[]error) {
	seen := make(map[string]struct{}, len(files))
	for _, path := range files {
		seen[path] = struct{}{}
		stamp, err := statStamp(path)
		if err != nil {
			*refreshErrors = append(*refreshErrors, errors.New("A Claude usage file changed during refresh"))
			continue
		}
		if cached, ok := i.claude[path]; ok && cached.stamp == stamp {
			continue
		}
		parsed, diagnostics := parseClaudeFile(ctx, path)
		i.claude[path] = claudeCacheEntry{stamp: stamp, candidates: parsed, diagnostics: diagnostics}
	}
	for path := range i.claude {
		if _, ok := seen[path]; !ok {
			delete(i.claude, path)
		}
	}
}

func (i *Indexer) refreshCodex(ctx context.Context, files []string, refreshErrors *[]error) {
	seen := make(map[string]struct{}, len(files))
	for _, path := range files {
		seen[path] = struct{}{}
		stamp, err := statStamp(path)
		if err != nil {
			*refreshErrors = append(*refreshErrors, errors.New("A Codex usage file changed during refresh"))
			continue
		}
		if cached, ok := i.codex[path]; ok && cached.stamp == stamp {
			continue
		}
		parsed, diagnostics := parseCodexFile(ctx, path)
		i.codex[path] = codexCacheEntry{stamp: stamp, events: parsed, diagnostics: diagnostics}
	}
	for path := range i.codex {
		if _, ok := seen[path]; !ok {
			delete(i.codex, path)
		}
	}
}

func statStamp(path string) (fileStamp, error) {
	info, err := os.Stat(path)
	if err != nil {
		return fileStamp{}, err
	}
	return fileStamp{size: info.Size(), modTime: info.ModTime().UnixNano()}, nil
}
