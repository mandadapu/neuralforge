package llm

var modelCosts = map[string]struct{ input, output float64 }{
	"claude-sonnet-4-5-20250514": {3.0, 15.0},
	"claude-opus-4-6":            {15.0, 75.0},
	"claude-haiku-4-5-20251001":  {0.80, 4.0},
	"gpt-4o":                     {2.50, 10.0},
	"gpt-4o-mini":                {0.15, 0.60},
}

func CalculateCost(model string, inputTokens, outputTokens int) float64 {
	costs, ok := modelCosts[model]
	if !ok {
		costs = struct{ input, output float64 }{3.0, 15.0}
	}
	return (float64(inputTokens)/1_000_000)*costs.input +
		(float64(outputTokens)/1_000_000)*costs.output
}
