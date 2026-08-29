package parser

import (
	"context"
	"errors"
	"io/fs"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

type Monitor struct {
	options  ScanOptions
	interval time.Duration
	index    *Indexer

	refreshMu sync.Mutex
	mu        sync.RWMutex
	stats     Stats
	lastErr   error
	refreshed time.Time
}

func NewMonitor(options ScanOptions, interval time.Duration) *Monitor {
	if interval < time.Second {
		interval = time.Second
	}
	return &Monitor{options: options, interval: interval, index: NewIndexer(options)}
}

func (m *Monitor) Refresh(ctx context.Context) error {
	m.refreshMu.Lock()
	defer m.refreshMu.Unlock()
	stats, err := m.index.Refresh(ctx)
	m.mu.Lock()
	m.stats = stats
	m.lastErr = err
	m.refreshed = time.Now()
	m.mu.Unlock()
	return err
}

func (m *Monitor) Snapshot() Stats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.stats
}

func (m *Monitor) LastError() error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.lastErr
}

func (m *Monitor) LastRefresh() time.Time {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.refreshed
}

func (m *Monitor) Run(ctx context.Context) error {
	_ = m.Refresh(ctx)
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer watcher.Close()

	for _, root := range append([]string{m.options.ClaudeRoot}, codexRoots(m.options.CodexRoot)...) {
		if err := addWatchTree(watcher, root); err != nil {
			m.recordWatcherError()
		}
	}

	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()
	var debounce <-chan time.Time
	var timer *time.Timer
	schedule := func() {
		if timer == nil {
			timer = time.NewTimer(350 * time.Millisecond)
		} else {
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timer.Reset(350 * time.Millisecond)
		}
		debounce = timer.C
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			_ = m.Refresh(ctx)
		case <-debounce:
			debounce = nil
			_ = m.Refresh(ctx)
		case event, ok := <-watcher.Events:
			if !ok {
				return errors.New("file watcher event channel closed unexpectedly")
			}
			if event.Op&fsnotify.Create != 0 {
				if err := addWatchTree(watcher, event.Name); err != nil {
					m.recordWatcherError()
				}
			}
			if strings.EqualFold(filepath.Ext(event.Name), ".jsonl") {
				schedule()
			}
		case _, ok := <-watcher.Errors:
			if !ok {
				return errors.New("file watcher error channel closed unexpectedly")
			}
			m.recordWatcherError()
		}
	}
}

func (m *Monitor) recordWatcherError() {
	m.mu.Lock()
	m.lastErr = errors.Join(m.lastErr, errors.New("file watcher reported an error"))
	m.mu.Unlock()
}

func addWatchTree(watcher *fsnotify.Watcher, root string) error {
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		if entry.Type()&fs.ModeSymlink != 0 {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			return watcher.Add(path)
		}
		return nil
	})
}
