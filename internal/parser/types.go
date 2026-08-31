package parser

import "time"

type Provider string

const (
	ProviderClaude Provider = "claude"
	ProviderCodex  Provider = "codex"
)

type Event struct {
	Provider       Provider
	Timestamp      time.Time
	Model          string
	InputTokens    int64
	OutputTokens   int64
	CacheRead      int64
	CacheWrite     int64
	Reasoning      int64
	CostUSD        float64
	CostEstimated  bool
	PricingMatched bool
}

func (e Event) ProcessedTokens() int64 {
	if e.Provider == ProviderClaude {
		return e.InputTokens + e.OutputTokens + e.CacheRead + e.CacheWrite
	}
	return e.InputTokens + e.OutputTokens
}

type Totals struct {
	InputTokens      int64   `json:"inputTokens"`
	OutputTokens     int64   `json:"outputTokens"`
	CacheReadTokens  int64   `json:"cacheReadTokens"`
	CacheWriteTokens int64   `json:"cacheWriteTokens"`
	ReasoningTokens  int64   `json:"reasoningTokens"`
	ProcessedTokens  int64   `json:"processedTokens"`
	UsageRecords     int64   `json:"usageRecords"`
	EstimatedCostUSD float64 `json:"estimatedCostUSD"`
	PricedTokens     int64   `json:"pricedTokens"`
}

type Breakdown struct {
	Key    string `json:"key"`
	Totals Totals `json:"totals"`
}

type ConsumptionPeriod struct {
	Totals   Totals      `json:"totals"`
	Models   []Breakdown `json:"models"`
	Timeline []Breakdown `json:"timeline"`
}

type ProviderConsumption struct {
	Provider   Provider          `json:"provider"`
	Today      ConsumptionPeriod `json:"today"`
	Last30Days ConsumptionPeriod `json:"last30Days"`
	AllTime    ConsumptionPeriod `json:"allTime"`
}

type QuotaWindow struct {
	Kind          string     `json:"kind"`
	Label         string     `json:"label"`
	UsedPercent   float64    `json:"usedPercent"`
	WindowMinutes int        `json:"windowMinutes,omitempty"`
	ResetsAt      *time.Time `json:"resetsAt,omitempty"`
	Active        bool       `json:"active,omitempty"`
}

type QuotaSnapshot struct {
	Provider   Provider      `json:"provider"`
	ObservedAt time.Time     `json:"observedAt"`
	Source     string        `json:"source"`
	Confidence string        `json:"confidence"`
	LimitName  string        `json:"limitName,omitempty"`
	Windows    []QuotaWindow `json:"windows"`
}

type Diagnostics struct {
	FilesScanned   int      `json:"filesScanned"`
	RecordsParsed  int      `json:"recordsParsed"`
	RecordsSkipped int      `json:"recordsSkipped"`
	ParseErrors    int      `json:"parseErrors"`
	BytesRead      int64    `json:"bytesReadThisRefresh"`
	Warnings       []string `json:"warnings,omitempty"`
}

type Stats struct {
	GeneratedAt time.Time             `json:"generatedAt"`
	Today       Totals                `json:"today"`
	Last7Days   Totals                `json:"last7Days"`
	AllTime     Totals                `json:"allTime"`
	Providers   []Breakdown           `json:"providers"`
	Models      []Breakdown           `json:"models"`
	Daily       []Breakdown           `json:"daily"`
	Hourly      []Breakdown           `json:"hourly"`
	Consumption []ProviderConsumption `json:"consumption"`
	Quotas      []QuotaSnapshot       `json:"quotas"`
	Diagnostics Diagnostics           `json:"diagnostics"`
}

type ScanOptions struct {
	ClaudeRoot string
	CodexRoot  string
	Now        func() time.Time
}
