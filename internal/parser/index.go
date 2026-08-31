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
	info        os.FileInfo
	cursor      streamCursor
	candidates  []claudeCandidate
	diagnostics Diagnostics
}

type codexCacheEntry struct {
	stamp       fileStamp
	info        os.FileInfo
	cursor      streamCursor
	state       codexState
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
	bytesRead := i.refreshClaude(ctx, claudeFiles, &refreshErrors)

	var codexFiles []string
	for _, root := range codexRoots(i.options.CodexRoot) {
		files, discoverErr := jsonlFiles(root)
		if discoverErr != nil {
			refreshErrors = append(refreshErrors, errors.New("Codex log discovery failed"))
			continue
		}
		codexFiles = append(codexFiles, files...)
	}
	bytesRead += i.refreshCodex(ctx, codexFiles, &refreshErrors)
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
	quotaValues := make([]*QuotaSnapshot, 0, len(i.codex))
	for _, cached := range i.codex {
		quotaValues = append(quotaValues, cached.state.quota)
	}
	stats.Quotas = LiveQuotaSnapshots(quotaValues, now())
	diagnostics.BytesRead = bytesRead
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

func (i *Indexer) refreshClaude(ctx context.Context, files []string, refreshErrors *[]error) int64 {
	seen := make(map[string]struct{}, len(files))
	var bytesRead int64
	for _, path := range files {
		seen[path] = struct{}{}
		info, err := os.Stat(path)
		if err != nil {
			*refreshErrors = append(*refreshErrors, errors.New("A Claude usage file changed during refresh"))
			continue
		}
		stamp := stampFromInfo(info)
		cached, exists := i.claude[path]
		sameFile := exists && cached.info != nil && os.SameFile(cached.info, info)
		if sameFile && cached.stamp == stamp && cached.cursor.offset == info.Size() {
			continue
		}

		var parsed []claudeCandidate
		var cursor streamCursor
		var diagnostics Diagnostics
		var read int64
		appendSafe := false
		if sameFile && info.Size() > cached.cursor.offset {
			appendSafe, err = cursorMatchesFile(path, cached.cursor)
			if err != nil {
				*refreshErrors = append(*refreshErrors, errors.New("A Claude usage file changed during refresh"))
				continue
			}
		}
		if appendSafe {
			var delta Diagnostics
			parsed, cursor, delta, read, err = parseClaudeTail(ctx, path, cached.cursor)
			diagnostics = cached.diagnostics
			mergeDiagnostics(&diagnostics, delta)
			parsed = append(cached.candidates, parsed...)
		} else {
			parsed, cursor, diagnostics, read, err = parseClaudeTail(ctx, path, streamCursor{})
			diagnostics.FilesScanned = 1
		}
		bytesRead += read
		if err != nil {
			*refreshErrors = append(*refreshErrors, errors.New("A Claude usage file could not be refreshed"))
			continue
		}
		postInfo, statErr := os.Stat(path)
		if statErr != nil || !os.SameFile(info, postInfo) || postInfo.Size() < cursor.offset {
			*refreshErrors = append(*refreshErrors, errors.New("A Claude usage file changed during refresh"))
			continue
		}
		i.claude[path] = claudeCacheEntry{
			stamp: stampFromInfo(postInfo), info: postInfo, cursor: cursor,
			candidates: parsed, diagnostics: diagnostics,
		}
	}
	for path := range i.claude {
		if _, ok := seen[path]; !ok {
			delete(i.claude, path)
		}
	}
	return bytesRead
}

func (i *Indexer) refreshCodex(ctx context.Context, files []string, refreshErrors *[]error) int64 {
	seen := make(map[string]struct{}, len(files))
	var bytesRead int64
	for _, path := range files {
		seen[path] = struct{}{}
		info, err := os.Stat(path)
		if err != nil {
			*refreshErrors = append(*refreshErrors, errors.New("A Codex usage file changed during refresh"))
			continue
		}
		stamp := stampFromInfo(info)
		cached, exists := i.codex[path]
		sameFile := exists && cached.info != nil && os.SameFile(cached.info, info)
		if sameFile && cached.stamp == stamp && cached.cursor.offset == info.Size() {
			continue
		}

		var events []Event
		var cursor streamCursor
		var state codexState
		var diagnostics Diagnostics
		var read int64
		appendSafe := false
		if sameFile && info.Size() > cached.cursor.offset {
			appendSafe, err = cursorMatchesFile(path, cached.cursor)
			if err != nil {
				*refreshErrors = append(*refreshErrors, errors.New("A Codex usage file changed during refresh"))
				continue
			}
		}
		if appendSafe {
			var delta Diagnostics
			events, cursor, state, delta, read, err = parseCodexTail(ctx, path, cached.cursor, cached.state)
			diagnostics = cached.diagnostics
			mergeDiagnostics(&diagnostics, delta)
			events = append(cached.events, events...)
		} else {
			initial := codexState{signatures: make(map[string]string)}
			events, cursor, state, diagnostics, read, err = parseCodexTail(ctx, path, streamCursor{}, initial)
			diagnostics.FilesScanned = 1
		}
		bytesRead += read
		if err != nil {
			*refreshErrors = append(*refreshErrors, errors.New("A Codex usage file could not be refreshed"))
			continue
		}
		postInfo, statErr := os.Stat(path)
		if statErr != nil || !os.SameFile(info, postInfo) || postInfo.Size() < cursor.offset {
			*refreshErrors = append(*refreshErrors, errors.New("A Codex usage file changed during refresh"))
			continue
		}
		i.codex[path] = codexCacheEntry{
			stamp: stampFromInfo(postInfo), info: postInfo, cursor: cursor, state: state,
			events: events, diagnostics: diagnostics,
		}
	}
	for path := range i.codex {
		if _, ok := seen[path]; !ok {
			delete(i.codex, path)
		}
	}
	return bytesRead
}

func stampFromInfo(info os.FileInfo) fileStamp {
	return fileStamp{size: info.Size(), modTime: info.ModTime().UnixNano()}
}
