package cost

import (
	"math"
	"testing"

	"aryanmehrotra/llm-gateway/models"
)

func TestCalculate(t *testing.T) {
	tests := []struct {
		name     string
		model    string
		usage    models.Usage
		wantCost float64
	}{
		{
			name:  "known model gpt-4o returns correct cost",
			model: "gpt-4o",
			usage: models.Usage{
				PromptTokens:     1000,
				CompletionTokens: 500,
				TotalTokens:      1500,
			},
			// input: 1000/1000 * 0.0025 = 0.0025
			// output: 500/1000 * 0.01 = 0.005
			// total: 0.0075
			wantCost: 0.0075,
		},
		{
			name:  "known model gpt-4o-mini returns correct cost",
			model: "gpt-4o-mini",
			usage: models.Usage{
				PromptTokens:     2000,
				CompletionTokens: 1000,
				TotalTokens:      3000,
			},
			// input: 2000/1000 * 0.00015 = 0.0003
			// output: 1000/1000 * 0.0006 = 0.0006
			// total: 0.0009
			wantCost: 0.0009,
		},
		{
			name:  "known model deepseek-chat returns correct cost",
			model: "deepseek-chat",
			usage: models.Usage{
				PromptTokens:     500,
				CompletionTokens: 200,
				TotalTokens:      700,
			},
			// input: 500/1000 * 0.00014 = 0.00007
			// output: 200/1000 * 0.00028 = 0.000056
			// total: 0.000126
			wantCost: 0.000126,
		},
		{
			name:  "unknown model returns 0",
			model: "nonexistent-model-xyz",
			usage: models.Usage{
				PromptTokens:     1000,
				CompletionTokens: 500,
				TotalTokens:      1500,
			},
			wantCost: 0,
		},
		{
			name:  "zero tokens returns 0",
			model: "gpt-4o",
			usage: models.Usage{
				PromptTokens:     0,
				CompletionTokens: 0,
				TotalTokens:      0,
			},
			wantCost: 0,
		},
		{
			name:  "embedding model has zero output cost",
			model: "text-embedding-3-small",
			usage: models.Usage{
				PromptTokens:     1000,
				CompletionTokens: 0,
				TotalTokens:      1000,
			},
			// input: 1000/1000 * 0.00002 = 0.00002
			// output: 0/1000 * 0 = 0
			wantCost: 0.00002,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Calculate(tt.model, tt.usage)
			if math.Abs(got-tt.wantCost) > 1e-10 {
				t.Errorf("Calculate(%q, %+v) = %v, want %v", tt.model, tt.usage, got, tt.wantCost)
			}
		})
	}
}

func TestGetPricing(t *testing.T) {
	tests := []struct {
		name      string
		model     string
		wantOK    bool
		wantInput float64
	}{
		{
			name:      "known model returns pricing",
			model:     "gpt-4o",
			wantOK:    true,
			wantInput: 0.0025,
		},
		{
			name:      "another known model returns pricing",
			model:     "claude-sonnet-4-20250514",
			wantOK:    true,
			wantInput: 0.003,
		},
		{
			name:   "unknown model returns not found",
			model:  "nonexistent-model",
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pricing, ok := GetPricing(tt.model)
			if ok != tt.wantOK {
				t.Errorf("GetPricing(%q) ok = %v, want %v", tt.model, ok, tt.wantOK)
			}

			if ok && math.Abs(pricing.InputPer1KTokens-tt.wantInput) > 1e-10 {
				t.Errorf("GetPricing(%q).InputPer1KTokens = %v, want %v",
					tt.model, pricing.InputPer1KTokens, tt.wantInput)
			}
		})
	}
}

func TestSetPricing(t *testing.T) {
	tests := []struct {
		name       string
		model      string
		pricing    ModelPricing
		wantInput  float64
		wantOutput float64
	}{
		{
			name:  "adds custom pricing for new model",
			model: "custom-model-test-1",
			pricing: ModelPricing{
				InputPer1KTokens:  0.005,
				OutputPer1KTokens: 0.015,
			},
			wantInput:  0.005,
			wantOutput: 0.015,
		},
		{
			name:  "overrides existing model pricing",
			model: "gpt-4o",
			pricing: ModelPricing{
				InputPer1KTokens:  0.099,
				OutputPer1KTokens: 0.199,
			},
			wantInput:  0.099,
			wantOutput: 0.199,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetPricing(tt.model, tt.pricing)

			got, ok := GetPricing(tt.model)
			if !ok {
				t.Fatalf("GetPricing(%q) returned ok=false after SetPricing", tt.model)
			}

			if math.Abs(got.InputPer1KTokens-tt.wantInput) > 1e-10 {
				t.Errorf("InputPer1KTokens = %v, want %v", got.InputPer1KTokens, tt.wantInput)
			}

			if math.Abs(got.OutputPer1KTokens-tt.wantOutput) > 1e-10 {
				t.Errorf("OutputPer1KTokens = %v, want %v", got.OutputPer1KTokens, tt.wantOutput)
			}
		})
	}

	// Cleanup: remove overrides so other tests aren't affected
	t.Cleanup(func() {
		overrideMu.Lock()
		delete(overridePricing, "custom-model-test-1")
		delete(overridePricing, "gpt-4o")
		overrideMu.Unlock()
	})
}

func TestSetPricing_OverrideTakesPrecedence(t *testing.T) {
	model := "gpt-3.5-turbo"
	originalPricing, _ := GetPricing(model)

	customPricing := ModelPricing{
		InputPer1KTokens:  0.123,
		OutputPer1KTokens: 0.456,
	}

	SetPricing(model, customPricing)

	got, ok := GetPricing(model)
	if !ok {
		t.Fatal("GetPricing returned ok=false after SetPricing")
	}

	if math.Abs(got.InputPer1KTokens-customPricing.InputPer1KTokens) > 1e-10 {
		t.Errorf("override not taking precedence: got input=%v, want %v",
			got.InputPer1KTokens, customPricing.InputPer1KTokens)
	}

	// Verify it's different from the original
	if math.Abs(got.InputPer1KTokens-originalPricing.InputPer1KTokens) < 1e-10 {
		t.Error("override pricing is the same as original; expected different")
	}

	// Cleanup
	t.Cleanup(func() {
		overrideMu.Lock()
		delete(overridePricing, model)
		overrideMu.Unlock()
	})
}

func TestParseCustomPricing(t *testing.T) {
	tests := []struct {
		name       string
		config     string
		checkModel string
		wantOK     bool
		wantInput  float64
		wantOutput float64
	}{
		{
			name:       "parses single entry",
			config:     "my-model:0.01:0.02",
			checkModel: "my-model",
			wantOK:     true,
			wantInput:  0.01,
			wantOutput: 0.02,
		},
		{
			name:       "parses multiple entries",
			config:     "model-a:0.001:0.002,model-b:0.003:0.004",
			checkModel: "model-b",
			wantOK:     true,
			wantInput:  0.003,
			wantOutput: 0.004,
		},
		{
			name:       "handles whitespace in entries",
			config:     " spaced-model : 0.005 : 0.006 ",
			checkModel: "spaced-model",
			wantOK:     true,
			wantInput:  0.005,
			wantOutput: 0.006,
		},
		{
			name:       "empty config does nothing",
			config:     "",
			checkModel: "no-such-model-empty",
			wantOK:     false,
		},
		{
			name:       "invalid format with two parts is skipped",
			config:     "bad-model:0.01",
			checkModel: "bad-model",
			wantOK:     false,
		},
		{
			name:       "non-numeric input is skipped",
			config:     "bad-nums:abc:0.01",
			checkModel: "bad-nums",
			wantOK:     false,
		},
		{
			name:       "non-numeric output is skipped",
			config:     "bad-nums2:0.01:xyz",
			checkModel: "bad-nums2",
			wantOK:     false,
		},
		{
			name:       "mixed valid and invalid entries",
			config:     "invalid:abc:def,valid-mixed:0.007:0.008",
			checkModel: "valid-mixed",
			wantOK:     true,
			wantInput:  0.007,
			wantOutput: 0.008,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ParseCustomPricing(tt.config)

			got, ok := GetPricing(tt.checkModel)
			if ok != tt.wantOK {
				t.Fatalf("GetPricing(%q) ok = %v, want %v", tt.checkModel, ok, tt.wantOK)
			}

			if ok {
				if math.Abs(got.InputPer1KTokens-tt.wantInput) > 1e-10 {
					t.Errorf("InputPer1KTokens = %v, want %v", got.InputPer1KTokens, tt.wantInput)
				}

				if math.Abs(got.OutputPer1KTokens-tt.wantOutput) > 1e-10 {
					t.Errorf("OutputPer1KTokens = %v, want %v", got.OutputPer1KTokens, tt.wantOutput)
				}
			}
		})
	}

	// Cleanup all custom models
	t.Cleanup(func() {
		overrideMu.Lock()
		for _, tt := range tests {
			delete(overridePricing, tt.checkModel)
		}
		overrideMu.Unlock()
	})
}

func TestParseCustomPricing_DoesNotPanic(t *testing.T) {
	inputs := []string{
		"",
		"::::",
		"a",
		"a:b",
		"a:b:c:d",
		",,,",
		"model:::",
		":0.01:0.02",
		"model:0.01:",
		"model::0.02",
		"   ",
	}

	for _, input := range inputs {
		t.Run(input, func(t *testing.T) {
			// Should not panic
			ParseCustomPricing(input)
		})
	}
}
