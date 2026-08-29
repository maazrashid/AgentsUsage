package parser

import "strings"

type rates struct {
	input, output, cacheRead, cacheWrite5m, cacheWrite1h float64
}

const perMillion = 1_000_000

var exactRates = map[string]rates{
	"gpt-5.6-sol":   perMillionRates(5, 30, 0.5),
	"gpt-5.6-terra": perMillionRates(2, 12, 0.2),
	"gpt-5.6-luna":  perMillionRates(0.2, 1.2, 0.02),
	"gpt-5.5":       perMillionRates(5, 30, 0.5),
	"gpt-5.4":       perMillionRates(2.5, 15, 0.25),
	"gpt-5":         perMillionRates(1.25, 10, 0.125),
	"gpt-5-mini":    perMillionRates(0.25, 2, 0.025),
	"gpt-4.1":       perMillionRates(2, 8, 0.5),
	"gpt-4o":        perMillionRates(2.5, 10, 1.25),
}

func perMillionRates(input, output, cached float64) rates {
	return rates{input: input / perMillion, output: output / perMillion, cacheRead: cached / perMillion}
}

func modelRates(model string) (rates, bool) {
	model = strings.ToLower(strings.TrimSpace(model))
	if r, ok := exactRates[model]; ok {
		return r, true
	}
	switch {
	case strings.Contains(model, "fable") || strings.Contains(model, "mythos"):
		return claudeRates(10, 50), true
	case strings.Contains(model, "opus-4-20250514") || strings.Contains(model, "opus-4-1"):
		return claudeRates(15, 75), true
	case strings.Contains(model, "opus"):
		return claudeRates(5, 25), true
	case strings.Contains(model, "sonnet"):
		return claudeRates(3, 15), true
	case strings.Contains(model, "haiku-4-5"):
		return claudeRates(1, 5), true
	case strings.Contains(model, "haiku"):
		return claudeRates(0.8, 4), true
	default:
		return rates{}, false
	}
}

func claudeRates(input, output float64) rates {
	return rates{
		input:        input / perMillion,
		output:       output / perMillion,
		cacheRead:    input * 0.1 / perMillion,
		cacheWrite5m: input * 1.25 / perMillion,
		cacheWrite1h: input * 2 / perMillion,
	}
}

func priceClaude(model string, input, output, cacheRead, cacheWrite, cacheWrite1h int64) (float64, bool) {
	r, ok := modelRates(model)
	if !ok {
		return 0, false
	}
	write1h := min64(max64(cacheWrite1h, 0), cacheWrite)
	write5m := cacheWrite - write1h
	return float64(input)*r.input + float64(output)*r.output + float64(cacheRead)*r.cacheRead +
		float64(write5m)*r.cacheWrite5m + float64(write1h)*r.cacheWrite1h, true
}

func priceCodex(model string, input, output, cached int64) (float64, bool) {
	r, ok := modelRates(model)
	if !ok {
		return 0, false
	}
	cached = min64(max64(cached, 0), input)
	return float64(input-cached)*r.input + float64(cached)*r.cacheRead + float64(output)*r.output, true
}

func min64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}
func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
