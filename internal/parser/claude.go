package parser

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"
)

type claudeCandidate struct {
	event     Event
	messageID string
	requestID string
}

type claudeRecord struct {
	Timestamp         string   `json:"timestamp"`
	RequestID         any      `json:"requestId"`
	CostUSD           *float64 `json:"costUSD"`
	IsAPIErrorMessage bool     `json:"isApiErrorMessage"`
	Message           struct {
		ID    any    `json:"id"`
		Model string `json:"model"`
		Usage struct {
			Input         *int64 `json:"input_tokens"`
			Output        *int64 `json:"output_tokens"`
			CacheRead     int64  `json:"cache_read_input_tokens"`
			CacheWrite    int64  `json:"cache_creation_input_tokens"`
			CacheCreation struct {
				OneHour int64 `json:"ephemeral_1h_input_tokens"`
				FiveMin int64 `json:"ephemeral_5m_input_tokens"`
			} `json:"cache_creation"`
		} `json:"usage"`
	} `json:"message"`
}

func parseClaudeFile(ctx context.Context, path string) ([]claudeCandidate, Diagnostics) {
	file, err := os.Open(path)
	if err != nil {
		return nil, Diagnostics{ParseErrors: 1, Warnings: []string{"A Claude usage file could not be read."}}
	}
	defer file.Close()

	var result []claudeCandidate
	diagnostics := Diagnostics{FilesScanned: 1}
	err = scanLines(ctx, file, func(line []byte) {
		var raw claudeRecord
		if err := json.Unmarshal(line, &raw); err != nil {
			diagnostics.ParseErrors++
			return
		}
		if raw.Message.Usage.Input == nil || raw.Message.Usage.Output == nil {
			diagnostics.RecordsSkipped++
			return
		}
		if raw.IsAPIErrorMessage || raw.Message.Model == "" || raw.Message.Model == "<synthetic>" {
			diagnostics.RecordsSkipped++
			return
		}
		timestamp, err := time.Parse(time.RFC3339Nano, raw.Timestamp)
		if err != nil {
			diagnostics.RecordsSkipped++
			return
		}
		cacheWrite := max64(nonNegative(raw.Message.Usage.CacheWrite),
			nonNegative(raw.Message.Usage.CacheCreation.OneHour)+nonNegative(raw.Message.Usage.CacheCreation.FiveMin))
		event := Event{
			Provider:      ProviderClaude,
			Timestamp:     timestamp,
			Model:         normalizedLabel(raw.Message.Model),
			InputTokens:   nonNegative(*raw.Message.Usage.Input),
			OutputTokens:  nonNegative(*raw.Message.Usage.Output),
			CacheRead:     nonNegative(raw.Message.Usage.CacheRead),
			CacheWrite:    cacheWrite,
			CostEstimated: true,
		}
		if raw.CostUSD != nil && *raw.CostUSD >= 0 {
			event.CostUSD = *raw.CostUSD
			event.PricingMatched = true
		} else {
			event.CostUSD, event.PricingMatched = priceClaude(
				event.Model,
				event.InputTokens,
				event.OutputTokens,
				event.CacheRead,
				event.CacheWrite,
				nonNegative(raw.Message.Usage.CacheCreation.OneHour),
			)
		}
		if event.ProcessedTokens() == 0 {
			diagnostics.RecordsSkipped++
			return
		}
		result = append(result, claudeCandidate{
			event: event, messageID: identityString(raw.Message.ID), requestID: identityString(raw.RequestID),
		})
		diagnostics.RecordsParsed++
	})
	if err != nil && err != context.Canceled {
		diagnostics.ParseErrors++
		diagnostics.Warnings = append(diagnostics.Warnings, "A Claude usage file could not be scanned.")
	}
	return result, diagnostics
}

func dedupeClaude(candidates []claudeCandidate) []Event {
	requestsByMessage := make(map[string]map[string]struct{})
	for _, candidate := range candidates {
		if candidate.messageID == "" || candidate.requestID == "" {
			continue
		}
		set := requestsByMessage[candidate.messageID]
		if set == nil {
			set = make(map[string]struct{})
			requestsByMessage[candidate.messageID] = set
		}
		set[candidate.requestID] = struct{}{}
	}

	result := make([]Event, 0, len(candidates))
	indexes := make(map[string]int)
	for _, candidate := range candidates {
		requestID := candidate.requestID
		if candidate.messageID != "" && requestID == "" {
			if requests := requestsByMessage[candidate.messageID]; len(requests) == 1 {
				for request := range requests {
					requestID = request
				}
			}
		}
		key := ""
		switch {
		case candidate.messageID != "":
			key = "message:" + candidate.messageID + ":" + requestID
		case requestID != "":
			key = "request:" + requestID
		}
		if key == "" {
			result = append(result, candidate.event)
			continue
		}
		index, exists := indexes[key]
		if !exists {
			indexes[key] = len(result)
			result = append(result, candidate.event)
			continue
		}
		if candidate.event.ProcessedTokens() > result[index].ProcessedTokens() {
			result[index] = candidate.event
		}
	}
	return result
}

func scanLines(ctx context.Context, reader io.Reader, visit func([]byte)) error {
	scanner := bufio.NewScanner(reader)
	buffer := make([]byte, 64*1024)
	scanner.Buffer(buffer, 16*1024*1024)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		line := scanner.Bytes()
		if len(line) > 0 {
			visit(line)
		}
	}
	return scanner.Err()
}

func identityString(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case float64:
		return fmt.Sprintf("%g", typed)
	case json.Number:
		return typed.String()
	default:
		return ""
	}
}

func nonNegative(value int64) int64 {
	if value < 0 {
		return 0
	}
	return value
}
