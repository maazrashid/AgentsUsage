package quota

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"time"

	"github.com/maazrashid/AgentsUsage/internal/parser"
)

type CodexClient struct {
	command string
	now     func() time.Time
}

type rpcResponse struct {
	ID     json.RawMessage `json:"id"`
	Result json.RawMessage `json:"result"`
	Error  json.RawMessage `json:"error"`
}

type codexRateLimitWindow struct {
	UsedPercent        float64 `json:"usedPercent"`
	WindowDurationMins int     `json:"windowDurationMins"`
	ResetsAt           int64   `json:"resetsAt"`
}

type codexRateLimitSnapshot struct {
	LimitName string                `json:"limitName"`
	Primary   *codexRateLimitWindow `json:"primary"`
	Secondary *codexRateLimitWindow `json:"secondary"`
}

func NewCodexClient() *CodexClient {
	return &CodexClient{command: "codex", now: time.Now}
}

func (c *CodexClient) Fetch(ctx context.Context) (*parser.QuotaSnapshot, error) {
	if _, err := exec.LookPath(c.command); err != nil {
		return nil, ErrUnavailable
	}
	requestCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	command := exec.CommandContext(requestCtx, c.command, "app-server", "--stdio")
	configureBackgroundProcess(command)
	stdin, err := command.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := command.Start(); err != nil {
		return nil, fmt.Errorf("start Codex app-server: %w", err)
	}
	defer func() {
		_ = stdin.Close()
		if command.Process != nil {
			_ = command.Process.Kill()
		}
		_ = command.Wait()
	}()

	encoder := json.NewEncoder(stdin)
	decoder := json.NewDecoder(bufio.NewReader(stdout))
	if err := encoder.Encode(map[string]any{
		"id": 1, "method": "initialize",
		"params": map[string]any{
			"clientInfo":   map[string]any{"name": "agentsusage", "title": "AgentsUsage", "version": "1"},
			"capabilities": nil,
		},
	}); err != nil {
		return nil, err
	}
	if _, err := readRPCResult(decoder, 1); err != nil {
		return nil, err
	}
	if err := encoder.Encode(map[string]any{"id": 2, "method": "account/rateLimits/read", "params": nil}); err != nil {
		return nil, err
	}
	result, err := readRPCResult(decoder, 2)
	if err != nil {
		return nil, err
	}
	return parseCodexRateLimits(result, c.now())
}

func readRPCResult(decoder *json.Decoder, wantedID int) (json.RawMessage, error) {
	for {
		var response rpcResponse
		if err := decoder.Decode(&response); err != nil {
			if errors.Is(err, io.EOF) {
				return nil, ErrUnavailable
			}
			return nil, fmt.Errorf("read Codex app-server response: %w", err)
		}
		var id int
		if len(response.ID) == 0 || json.Unmarshal(response.ID, &id) != nil || id != wantedID {
			continue
		}
		if len(response.Error) > 0 && string(response.Error) != "null" {
			return nil, ErrUnavailable
		}
		if len(response.Result) == 0 {
			return nil, ErrUnavailable
		}
		return response.Result, nil
	}
}

func parseCodexRateLimits(result json.RawMessage, observedAt time.Time) (*parser.QuotaSnapshot, error) {
	var payload struct {
		RateLimits *codexRateLimitSnapshot `json:"rateLimits"`
	}
	if err := json.Unmarshal(result, &payload); err != nil || payload.RateLimits == nil {
		return nil, ErrUnavailable
	}
	windows := make([]parser.QuotaWindow, 0, 2)
	ordered := []struct {
		label string
		value *codexRateLimitWindow
	}{{"primary", payload.RateLimits.Primary}, {"secondary", payload.RateLimits.Secondary}}
	for _, item := range ordered {
		if item.value == nil {
			continue
		}
		var resetsAt *time.Time
		if item.value.ResetsAt > 0 {
			parsed := time.Unix(item.value.ResetsAt, 0)
			resetsAt = &parsed
		}
		kind := "other"
		if item.value.WindowDurationMins <= 5*60 {
			kind = "session"
		} else if item.value.WindowDurationMins >= 7*24*60 {
			kind = "weekly"
		}
		windows = append(windows, parser.QuotaWindow{
			Kind: kind, Label: item.label, UsedPercent: clampPercent(item.value.UsedPercent),
			WindowMinutes: item.value.WindowDurationMins, ResetsAt: resetsAt,
		})
	}
	if len(windows) == 0 {
		return nil, ErrUnavailable
	}
	return &parser.QuotaSnapshot{
		Provider: parser.ProviderCodex, ObservedAt: observedAt, Source: "cli",
		Confidence: "live", LimitName: payload.RateLimits.LimitName, Windows: windows,
	}, nil
}
