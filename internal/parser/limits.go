package parser

import (
	"math"
	"sort"
	"time"
)

func copyQuota(snapshot *QuotaSnapshot) *QuotaSnapshot {
	if snapshot == nil {
		return nil
	}
	cloned := *snapshot
	cloned.Windows = append([]QuotaWindow(nil), snapshot.Windows...)
	return &cloned
}

func LiveQuotaSnapshots(values []*QuotaSnapshot, now time.Time) []QuotaSnapshot {
	latest := make(map[Provider]QuotaSnapshot)
	for _, value := range values {
		if value == nil || value.Provider == "" {
			continue
		}
		candidate := *value
		candidate.Windows = liveQuotaWindows(value.Windows, now)
		if len(candidate.Windows) == 0 {
			continue
		}
		current, ok := latest[value.Provider]
		if !ok || quotaConfidenceRank(candidate.Confidence) > quotaConfidenceRank(current.Confidence) ||
			(quotaConfidenceRank(candidate.Confidence) == quotaConfidenceRank(current.Confidence) && candidate.ObservedAt.After(current.ObservedAt)) {
			latest[value.Provider] = candidate
		}
	}
	result := make([]QuotaSnapshot, 0, len(latest))
	for _, value := range latest {
		result = append(result, value)
	}
	sort.Slice(result, func(left, right int) bool { return result[left].Provider < result[right].Provider })
	return result
}

func quotaConfidenceRank(value string) int {
	switch value {
	case "live", "exact":
		return 2
	case "last-observed":
		return 1
	default:
		return 0
	}
}

func liveQuotaWindows(values []QuotaWindow, now time.Time) []QuotaWindow {
	result := make([]QuotaWindow, 0, len(values))
	for _, value := range values {
		if !math.IsNaN(value.UsedPercent) && !math.IsInf(value.UsedPercent, 0) &&
			(value.ResetsAt == nil || value.ResetsAt.After(now)) {
			result = append(result, value)
		}
	}
	return result
}

func quotaWindowKind(label string, minutes int) string {
	switch {
	case minutes > 0 && minutes <= 5*60:
		return "session"
	case minutes >= 7*24*60:
		return "weekly"
	case label == "primary":
		return "session"
	case label == "secondary":
		return "weekly"
	default:
		return "other"
	}
}
