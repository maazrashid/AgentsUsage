package parser

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func Scan(ctx context.Context, options ScanOptions) (Stats, error) {
	return NewIndexer(options).Refresh(ctx)
}

func jsonlFiles(root string) ([]string, error) {
	if strings.TrimSpace(root) == "" {
		return nil, nil
	}
	info, err := os.Lstat(root)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return nil, nil
	}
	var files []string
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.Type()&os.ModeSymlink != 0 {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Type().IsRegular() && strings.EqualFold(filepath.Ext(entry.Name()), ".jsonl") {
			files = append(files, path)
		}
		return nil
	})
	sort.Strings(files)
	return files, err
}

func codexRoots(root string) []string {
	if strings.TrimSpace(root) == "" {
		return nil
	}
	name := strings.ToLower(filepath.Base(filepath.Clean(root)))
	if name == "sessions" || name == "archived_sessions" {
		return []string{root}
	}
	return []string{filepath.Join(root, "sessions"), filepath.Join(root, "archived_sessions")}
}

func aggregate(events []Event, now time.Time) Stats {
	stats := Stats{GeneratedAt: now}
	providers := make(map[string]Totals)
	models := make(map[string]Totals)
	daily := make(map[string]Totals)
	location := now.Location()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, location)
	weekStart := todayStart.AddDate(0, 0, -6)
	dailyStart := todayStart.AddDate(0, 0, -29)

	for _, event := range events {
		addEvent(&stats.AllTime, event)
		if !event.Timestamp.Before(weekStart) && !event.Timestamp.After(now) {
			addEvent(&stats.Last7Days, event)
		}
		if !event.Timestamp.Before(todayStart) && !event.Timestamp.After(now) {
			addEvent(&stats.Today, event)
		}
		providerTotal := providers[string(event.Provider)]
		addEvent(&providerTotal, event)
		providers[string(event.Provider)] = providerTotal
		modelTotal := models[event.Model]
		addEvent(&modelTotal, event)
		models[event.Model] = modelTotal
		if !event.Timestamp.Before(dailyStart) {
			key := event.Timestamp.In(location).Format("2006-01-02")
			dayTotal := daily[key]
			addEvent(&dayTotal, event)
			daily[key] = dayTotal
		}
	}
	stats.Providers = sortedBreakdowns(providers, false)
	stats.Models = sortedBreakdowns(models, true)
	stats.Daily = sortedBreakdowns(daily, false)
	return stats
}

func addEvent(total *Totals, event Event) {
	total.InputTokens += event.InputTokens
	total.OutputTokens += event.OutputTokens
	total.CacheReadTokens += event.CacheRead
	total.CacheWriteTokens += event.CacheWrite
	total.ReasoningTokens += event.Reasoning
	total.ProcessedTokens += event.ProcessedTokens()
	total.EstimatedCostUSD += event.CostUSD
	if event.PricingMatched {
		total.PricedTokens += event.ProcessedTokens()
	}
}

func sortedBreakdowns(values map[string]Totals, byTokens bool) []Breakdown {
	rows := make([]Breakdown, 0, len(values))
	for key, totals := range values {
		rows = append(rows, Breakdown{Key: key, Totals: totals})
	}
	sort.Slice(rows, func(i, j int) bool {
		if byTokens && rows[i].Totals.ProcessedTokens != rows[j].Totals.ProcessedTokens {
			return rows[i].Totals.ProcessedTokens > rows[j].Totals.ProcessedTokens
		}
		return rows[i].Key < rows[j].Key
	})
	return rows
}

func mergeDiagnostics(target *Diagnostics, source Diagnostics) {
	target.FilesScanned += source.FilesScanned
	target.RecordsParsed += source.RecordsParsed
	target.RecordsSkipped += source.RecordsSkipped
	target.ParseErrors += source.ParseErrors
	target.Warnings = append(target.Warnings, source.Warnings...)
}
