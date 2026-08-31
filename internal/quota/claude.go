package quota

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/maazrashid/AgentsUsage/internal/parser"
)

const claudeUsageEndpoint = "https://api.anthropic.com/api/oauth/usage"

var ErrUnavailable = errors.New("quota unavailable")

type ClaudeClient struct {
	credentialsPath string
	endpoint        string
	httpClient      *http.Client
	now             func() time.Time
}

type claudeCredentials struct {
	OAuth struct {
		AccessToken string `json:"accessToken"`
		ExpiresAt   int64  `json:"expiresAt"`
	} `json:"claudeAiOauth"`
}

type claudeLimit struct {
	Utilization *float64 `json:"utilization"`
	ResetsAt    *string  `json:"resets_at"`
}

type claudeLimitEntry struct {
	Kind     string   `json:"kind"`
	Group    string   `json:"group"`
	Percent  *float64 `json:"percent"`
	ResetsAt *string  `json:"resets_at"`
	IsActive bool     `json:"is_active"`
	Scope    *struct {
		Model *struct {
			DisplayName string `json:"display_name"`
		} `json:"model"`
		Surface string `json:"surface"`
	} `json:"scope"`
}

type claudeUsageResponse struct {
	FiveHour       *claudeLimit       `json:"five_hour"`
	SevenDay       *claudeLimit       `json:"seven_day"`
	SevenDayOpus   *claudeLimit       `json:"seven_day_opus"`
	SevenDaySonnet *claudeLimit       `json:"seven_day_sonnet"`
	Limits         []claudeLimitEntry `json:"limits"`
}

func NewClaudeClient(credentialsPath string) *ClaudeClient {
	return &ClaudeClient{
		credentialsPath: credentialsPath,
		endpoint:        claudeUsageEndpoint,
		httpClient:      &http.Client{Timeout: 15 * time.Second},
		now:             time.Now,
	}
}

func (c *ClaudeClient) Fetch(ctx context.Context) (*parser.QuotaSnapshot, error) {
	data, err := os.ReadFile(c.credentialsPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrUnavailable
	}
	if err != nil {
		return nil, fmt.Errorf("read Claude quota credentials: %w", err)
	}
	var credentials claudeCredentials
	if err := json.Unmarshal(data, &credentials); err != nil || strings.TrimSpace(credentials.OAuth.AccessToken) == "" {
		return nil, ErrUnavailable
	}
	if credentials.OAuth.ExpiresAt > 0 && c.now().UnixMilli() >= credentials.OAuth.ExpiresAt {
		return nil, ErrUnavailable
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.endpoint, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", "Bearer "+credentials.OAuth.AccessToken)
	request.Header.Set("anthropic-beta", "oauth-2025-04-20")
	request.Header.Set("Accept", "application/json")
	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("fetch Claude quota: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		return nil, ErrUnavailable
	}
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch Claude quota: HTTP %d", response.StatusCode)
	}
	var usage claudeUsageResponse
	decoder := json.NewDecoder(io.LimitReader(response.Body, 1<<20))
	if err := decoder.Decode(&usage); err != nil {
		return nil, fmt.Errorf("decode Claude quota response: %w", err)
	}
	windows := normalizeClaudeWindows(usage)
	if len(windows) == 0 {
		return nil, ErrUnavailable
	}
	return &parser.QuotaSnapshot{
		Provider: parser.ProviderClaude, ObservedAt: c.now(), Source: "oauth",
		Confidence: "exact", LimitName: "Claude", Windows: windows,
	}, nil
}

func normalizeClaudeWindows(usage claudeUsageResponse) []parser.QuotaWindow {
	if len(usage.Limits) > 0 {
		windows := make([]parser.QuotaWindow, 0, len(usage.Limits))
		for _, value := range usage.Limits {
			kind := ""
			switch {
			case value.Kind == "session" || value.Group == "session":
				kind = "session"
			case value.Kind == "weekly_all":
				kind = "weekly"
			case value.Kind == "weekly_scoped" || value.Group == "weekly":
				kind = "weekly_scoped"
			}
			if kind == "" || value.Percent == nil || math.IsNaN(*value.Percent) || math.IsInf(*value.Percent, 0) {
				continue
			}
			label := map[string]string{"session": "5h", "weekly": "Weekly", "weekly_scoped": "Weekly scoped"}[kind]
			if kind == "weekly_scoped" && value.Scope != nil {
				if value.Scope.Model != nil && strings.TrimSpace(value.Scope.Model.DisplayName) != "" {
					label = value.Scope.Model.DisplayName
				} else if strings.TrimSpace(value.Scope.Surface) != "" {
					label = value.Scope.Surface
				}
			}
			windows = append(windows, parser.QuotaWindow{
				Kind: kind, Label: label, UsedPercent: clampPercent(*value.Percent),
				WindowMinutes: quotaMinutes(kind), ResetsAt: parseReset(value.ResetsAt), Active: value.IsActive,
			})
		}
		if len(windows) > 0 {
			return windows
		}
	}
	values := []struct {
		limit *claudeLimit
		kind  string
		label string
	}{
		{usage.FiveHour, "session", "5h"},
		{usage.SevenDay, "weekly", "Weekly"},
		{usage.SevenDayOpus, "weekly_scoped", "Opus"},
		{usage.SevenDaySonnet, "weekly_scoped", "Sonnet"},
	}
	windows := make([]parser.QuotaWindow, 0, len(values))
	for _, value := range values {
		if value.limit == nil || value.limit.Utilization == nil {
			continue
		}
		windows = append(windows, parser.QuotaWindow{
			Kind: value.kind, Label: value.label, UsedPercent: clampPercent(*value.limit.Utilization),
			WindowMinutes: quotaMinutes(value.kind), ResetsAt: parseReset(value.limit.ResetsAt),
		})
	}
	return windows
}

func parseReset(value *string) *time.Time {
	if value == nil || strings.TrimSpace(*value) == "" {
		return nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, *value)
	if err != nil {
		return nil
	}
	return &parsed
}

func quotaMinutes(kind string) int {
	if kind == "session" {
		return 5 * 60
	}
	return 7 * 24 * 60
}

func clampPercent(value float64) float64 {
	return math.Max(0, math.Min(100, value))
}
