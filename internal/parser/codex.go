package parser

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"
)

type codexCounts struct {
	input, cached, output, reasoning, total int64
}

type codexState struct {
	model       string
	highWater   *codexCounts
	signatures  map[string]string
	previousSig string
}

func parseCodexFile(ctx context.Context, path string) ([]Event, Diagnostics) {
	file, err := os.Open(path)
	if err != nil {
		return nil, Diagnostics{ParseErrors: 1, Warnings: []string{"A Codex usage file could not be read."}}
	}
	defer file.Close()

	state := codexState{signatures: make(map[string]string)}
	var events []Event
	diagnostics := Diagnostics{FilesScanned: 1}
	err = scanLines(ctx, file, func(line []byte) {
		entry, ok := decodeObject(line)
		if !ok {
			diagnostics.ParseErrors++
			return
		}
		typeName, _ := entry["type"].(string)
		payload, _ := entry["payload"].(map[string]any)
		switch typeName {
		case "turn_context":
			if model, ok := payload["model"].(string); ok {
				state.model = normalizedLabel(model)
			}
		case "event_msg":
			payloadType, _ := payload["type"].(string)
			if payloadType != "token_count" {
				return
			}
			event, found := parseCodexTokenCount(entry, payload, &state)
			if !found {
				diagnostics.RecordsSkipped++
				return
			}
			events = append(events, event)
			diagnostics.RecordsParsed++
		}
	})
	if err != nil && err != context.Canceled {
		diagnostics.ParseErrors++
		diagnostics.Warnings = append(diagnostics.Warnings, "A Codex usage file could not be scanned.")
	}
	return events, diagnostics
}

func parseCodexTokenCount(entry, payload map[string]any, state *codexState) (Event, bool) {
	info, _ := payload["info"].(map[string]any)
	if info == nil {
		return Event{}, false
	}
	current, hasCurrent := decodeCodexCounts(info["total_token_usage"])
	last, hasLast := decodeCodexCounts(info["last_token_usage"])
	if !hasCurrent && !hasLast {
		return Event{}, false
	}
	timestamp, ok := parseTimestamp(entry["timestamp"])
	if !ok {
		return Event{}, false
	}
	signature := countsSignature(current, hasCurrent) + "|" + countsSignature(last, hasLast)
	source := rateLimitSource(payload, info)
	duplicate := hasCurrent && (state.signatures[source] == signature || state.previousSig == signature)
	previous := codexCounts{}
	if state.highWater != nil {
		previous = *state.highWater
	}
	if hasCurrent {
		next := componentMax(previous, current)
		state.highWater = &next
		state.signatures[source] = signature
	}
	state.previousSig = signature

	var usage codexCounts
	switch {
	case duplicate:
		return Event{}, false
	case hasLast:
		usage = last
	case hasCurrent:
		usage = componentDelta(*state.highWater, previous)
	}
	usage.cached = min64(usage.cached, usage.input)
	usage.reasoning = min64(usage.reasoning, usage.output)
	if usage.input+usage.output == 0 {
		return Event{}, false
	}
	cost, priced := priceCodex(state.model, usage.input, usage.output, usage.cached)
	return Event{
		Provider: ProviderCodex, Timestamp: timestamp, Model: normalizedLabel(state.model),
		InputTokens: usage.input, OutputTokens: usage.output, CacheRead: usage.cached,
		Reasoning: usage.reasoning, CostUSD: cost, CostEstimated: true, PricingMatched: priced,
	}, true
}

func decodeObject(line []byte) (map[string]any, bool) {
	decoder := json.NewDecoder(bytes.NewReader(line))
	decoder.UseNumber()
	var value map[string]any
	if err := decoder.Decode(&value); err != nil || value == nil {
		return nil, false
	}
	return value, true
}

func decodeCodexCounts(value any) (codexCounts, bool) {
	object, ok := value.(map[string]any)
	if !ok {
		return codexCounts{}, false
	}
	input, inputOK := integerField(object, "input_tokens")
	output, outputOK := integerField(object, "output_tokens")
	if !inputOK || !outputOK || input < 0 || output < 0 {
		return codexCounts{}, false
	}
	cached, cachedOK := integerField(object, "cached_input_tokens")
	if !cachedOK {
		cached, _ = integerField(object, "cache_read_input_tokens")
	}
	reasoning, _ := integerField(object, "reasoning_output_tokens")
	total, ok := integerField(object, "total_tokens")
	if !ok {
		total = input + output
	}
	return codexCounts{input: input, cached: nonNegative(cached), output: output, reasoning: nonNegative(reasoning), total: nonNegative(total)}, true
}

func integerField(object map[string]any, key string) (int64, bool) {
	value, exists := object[key]
	if !exists {
		return 0, false
	}
	switch typed := value.(type) {
	case json.Number:
		result, err := typed.Int64()
		return result, err == nil
	case float64:
		result := int64(typed)
		return result, float64(result) == typed
	default:
		return 0, false
	}
}

func parseTimestamp(value any) (time.Time, bool) {
	if text, ok := value.(string); ok {
		parsed, err := time.Parse(time.RFC3339Nano, text)
		return parsed, err == nil
	}
	var number float64
	switch typed := value.(type) {
	case json.Number:
		var err error
		number, err = strconv.ParseFloat(typed.String(), 64)
		if err != nil {
			return time.Time{}, false
		}
	case float64:
		number = typed
	default:
		return time.Time{}, false
	}
	if math.IsNaN(number) || math.IsInf(number, 0) || number < 0 || number > 253402300799000 {
		return time.Time{}, false
	}
	if number > 1e12 {
		number /= 1000
	}
	seconds := int64(number)
	nanos := int64((number - float64(seconds)) * 1e9)
	return time.Unix(seconds, nanos), true
}

func rateLimitSource(payload, info map[string]any) string {
	for _, parent := range []map[string]any{payload, info} {
		limits, _ := parent["rate_limits"].(map[string]any)
		if id, ok := limits["limit_id"].(string); ok && id != "" {
			return id
		}
	}
	return "default"
}

func countsSignature(value codexCounts, exists bool) string {
	if !exists {
		return "-"
	}
	return fmt.Sprintf("%d:%d:%d:%d:%d", value.input, value.cached, value.output, value.reasoning, value.total)
}

func componentMax(a, b codexCounts) codexCounts {
	return codexCounts{max64(a.input, b.input), max64(a.cached, b.cached), max64(a.output, b.output), max64(a.reasoning, b.reasoning), max64(a.total, b.total)}
}

func componentDelta(a, b codexCounts) codexCounts {
	return codexCounts{max64(0, a.input-b.input), max64(0, a.cached-b.cached), max64(0, a.output-b.output), max64(0, a.reasoning-b.reasoning), max64(0, a.total-b.total)}
}

func normalizedLabel(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "unknown"
	}
	if len(value) > 120 || strings.ContainsAny(value, "\r\n\x00") {
		return "unknown"
	}
	return value
}
