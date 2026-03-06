package llm

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCalculateCost(t *testing.T) {
	tests := []struct {
		model  string
		input  int
		output int
		want   float64
	}{
		{"claude-sonnet-4-5-20250514", 1000, 500, 0.003 + 0.0075},
		{"gpt-4o", 1_000_000, 0, 2.50},
		{"unknown-model", 1000, 1000, 0.003 + 0.015},
	}
	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			got := CalculateCost(tt.model, tt.input, tt.output)
			assert.InDelta(t, tt.want, got, 0.0001)
		})
	}
}
