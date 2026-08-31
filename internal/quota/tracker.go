package quota

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/maazrashid/AgentsUsage/internal/parser"
)

type UsageSource interface {
	Snapshot() parser.Stats
	Refresh(context.Context) error
	LastError() error
	LastRefresh() time.Time
}

type Fetcher interface {
	Fetch(context.Context) (*parser.QuotaSnapshot, error)
}

type Tracker struct {
	base     UsageSource
	claude   Fetcher
	codex    Fetcher
	interval time.Duration

	refreshMu      sync.Mutex
	mu             sync.RWMutex
	claudeSnapshot *parser.QuotaSnapshot
	codexSnapshot  *parser.QuotaSnapshot
}

func NewTracker(base UsageSource, claude, codex Fetcher, interval time.Duration) *Tracker {
	if interval < 30*time.Second {
		interval = 30 * time.Second
	}
	return &Tracker{base: base, claude: claude, codex: codex, interval: interval}
}

func (t *Tracker) Run(ctx context.Context) {
	t.refreshQuotas(ctx)
	ticker := time.NewTicker(t.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			t.refreshQuotas(ctx)
		}
	}
}

func (t *Tracker) Refresh(ctx context.Context) error {
	err := t.base.Refresh(ctx)
	t.refreshQuotas(ctx)
	return err
}

func (t *Tracker) Snapshot() parser.Stats {
	stats := t.base.Snapshot()
	values := make([]*parser.QuotaSnapshot, 0, len(stats.Quotas)+1)
	for index := range stats.Quotas {
		value := stats.Quotas[index]
		values = append(values, &value)
	}
	t.mu.RLock()
	if t.claudeSnapshot != nil {
		value := *t.claudeSnapshot
		value.Windows = append([]parser.QuotaWindow(nil), t.claudeSnapshot.Windows...)
		values = append(values, &value)
	}
	if t.codexSnapshot != nil {
		value := *t.codexSnapshot
		value.Windows = append([]parser.QuotaWindow(nil), t.codexSnapshot.Windows...)
		values = append(values, &value)
	}
	t.mu.RUnlock()
	stats.Quotas = parser.LiveQuotaSnapshots(values, time.Now())
	return stats
}

func (t *Tracker) LastError() error       { return t.base.LastError() }
func (t *Tracker) LastRefresh() time.Time { return t.base.LastRefresh() }

func (t *Tracker) refreshQuotas(ctx context.Context) {
	t.refreshMu.Lock()
	defer t.refreshMu.Unlock()
	t.refreshClaude(ctx)
	t.refreshCodex(ctx)
}

func (t *Tracker) refreshClaude(ctx context.Context) {
	if t.claude == nil {
		return
	}
	snapshot, err := t.claude.Fetch(ctx)
	t.mu.Lock()
	defer t.mu.Unlock()
	if err == nil {
		t.claudeSnapshot = snapshot
		return
	}
	if errors.Is(err, ErrUnavailable) {
		t.claudeSnapshot = nil
	}
}

func (t *Tracker) refreshCodex(ctx context.Context) {
	if t.codex == nil {
		return
	}
	snapshot, err := t.codex.Fetch(ctx)
	t.mu.Lock()
	defer t.mu.Unlock()
	if err == nil {
		t.codexSnapshot = snapshot
		return
	}
	if errors.Is(err, ErrUnavailable) {
		t.codexSnapshot = nil
	}
}
