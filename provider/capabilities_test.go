package provider

import (
	"testing"

	"examples/llm-gateway/models"
)

func TestHasCapability(t *testing.T) {
	tests := []struct {
		name  string
		model string
		cap   Capability
		want  bool
	}{
		// OpenAI models — full capability
		{
			name:  "gpt-4o supports tools",
			model: "gpt-4o",
			cap:   CapTools,
			want:  true,
		},
		{
			name:  "gpt-4o supports vision",
			model: "gpt-4o",
			cap:   CapVision,
			want:  true,
		},
		{
			name:  "gpt-4o supports JSON mode",
			model: "gpt-4o",
			cap:   CapJSON,
			want:  true,
		},
		{
			name:  "gpt-4o supports streaming",
			model: "gpt-4o",
			cap:   CapStreaming,
			want:  true,
		},
		{
			name:  "gpt-3.5-turbo supports tools but not vision",
			model: "gpt-3.5-turbo",
			cap:   CapVision,
			want:  false,
		},
		{
			name:  "gpt-3.5-turbo supports tools",
			model: "gpt-3.5-turbo",
			cap:   CapTools,
			want:  true,
		},

		// Anthropic models
		{
			name:  "claude-sonnet-4 supports tools",
			model: "claude-sonnet-4-20250514",
			cap:   CapTools,
			want:  true,
		},
		{
			name:  "claude-sonnet-4 supports vision",
			model: "claude-sonnet-4-20250514",
			cap:   CapVision,
			want:  true,
		},

		// Groq models — mixed capabilities
		{
			name:  "llama-3.3-70b supports tools",
			model: "llama-3.3-70b-versatile",
			cap:   CapTools,
			want:  true,
		},
		{
			name:  "mixtral-8x7b does not support tools",
			model: "mixtral-8x7b-32768",
			cap:   CapTools,
			want:  false,
		},
		{
			name:  "mixtral-8x7b supports streaming",
			model: "mixtral-8x7b-32768",
			cap:   CapStreaming,
			want:  true,
		},
		{
			name:  "gemma2-9b-it does not support tools",
			model: "gemma2-9b-it",
			cap:   CapTools,
			want:  false,
		},

		// DeepSeek models
		{
			name:  "deepseek-chat supports tools",
			model: "deepseek-chat",
			cap:   CapTools,
			want:  true,
		},
		{
			name:  "deepseek-reasoner does not support tools",
			model: "deepseek-reasoner",
			cap:   CapTools,
			want:  false,
		},
		{
			name:  "deepseek-reasoner supports streaming",
			model: "deepseek-reasoner",
			cap:   CapStreaming,
			want:  true,
		},

		// Gemini models
		{
			name:  "gemini-2.0-flash supports tools",
			model: "gemini-2.0-flash",
			cap:   CapTools,
			want:  true,
		},
		{
			name:  "gemini-2.0-flash supports vision",
			model: "gemini-2.0-flash",
			cap:   CapVision,
			want:  true,
		},
		{
			name:  "gemini-2.0-flash-lite does not support vision",
			model: "gemini-2.0-flash-lite",
			cap:   CapVision,
			want:  false,
		},

		// Ollama models
		{
			name:  "llama3 does not support tools",
			model: "llama3",
			cap:   CapTools,
			want:  false,
		},
		{
			name:  "llama3.1 supports tools",
			model: "llama3.1",
			cap:   CapTools,
			want:  true,
		},
		{
			name:  "codellama does not support tools",
			model: "codellama",
			cap:   CapTools,
			want:  false,
		},

		// Unknown models — HasCapability returns true (assume capable)
		{
			name:  "unknown model assumes tools capable",
			model: "totally-unknown-model",
			cap:   CapTools,
			want:  true,
		},
		{
			name:  "unknown model assumes vision capable",
			model: "future-model-xyz",
			cap:   CapVision,
			want:  true,
		},
		{
			name:  "unknown model assumes streaming capable",
			model: "mystery-model",
			cap:   CapStreaming,
			want:  true,
		},
		{
			name:  "unknown model assumes JSON capable",
			model: "another-unknown",
			cap:   CapJSON,
			want:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HasCapability(tt.model, tt.cap)
			if got != tt.want {
				t.Errorf("HasCapability(%q, %d) = %v, want %v", tt.model, tt.cap, got, tt.want)
			}
		})
	}
}

func TestStripUnsupportedParams(t *testing.T) {
	sampleTools := []models.Tool{
		{
			Type: "function",
			Function: models.ToolFunction{
				Name:        "get_weather",
				Description: "Get weather",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"location": map[string]any{"type": "string"},
					},
				},
			},
		},
	}

	tests := []struct {
		name          string
		model         string
		tools         []models.Tool
		toolChoice    any
		wantStripped  bool
		wantToolsNil  bool
		wantChoiceNil bool
	}{
		{
			name:          "model without tool support strips tools",
			model:         "mixtral-8x7b-32768",
			tools:         sampleTools,
			toolChoice:    "auto",
			wantStripped:  true,
			wantToolsNil:  true,
			wantChoiceNil: true,
		},
		{
			name:          "model with tool support keeps tools",
			model:         "gpt-4o",
			tools:         sampleTools,
			toolChoice:    "auto",
			wantStripped:  false,
			wantToolsNil:  false,
			wantChoiceNil: false,
		},
		{
			name:          "deepseek-reasoner without tool support strips tools",
			model:         "deepseek-reasoner",
			tools:         sampleTools,
			toolChoice:    "required",
			wantStripped:  true,
			wantToolsNil:  true,
			wantChoiceNil: true,
		},
		{
			name:          "gemma2-9b-it without tool support strips tools",
			model:         "gemma2-9b-it",
			tools:         sampleTools,
			toolChoice:    "auto",
			wantStripped:  true,
			wantToolsNil:  true,
			wantChoiceNil: true,
		},
		{
			name:          "model with tools but no tools in request does not strip",
			model:         "gpt-4o",
			tools:         nil,
			toolChoice:    nil,
			wantStripped:  false,
			wantToolsNil:  true,
			wantChoiceNil: true,
		},
		{
			name:          "model without tools and no tools in request does not strip",
			model:         "mixtral-8x7b-32768",
			tools:         nil,
			toolChoice:    nil,
			wantStripped:  false,
			wantToolsNil:  true,
			wantChoiceNil: true,
		},
		{
			name:          "unknown model keeps tools (assumes capable)",
			model:         "future-model",
			tools:         sampleTools,
			toolChoice:    "auto",
			wantStripped:  false,
			wantToolsNil:  false,
			wantChoiceNil: false,
		},
		{
			name:          "ollama llama3 without tool support strips tools",
			model:         "llama3",
			tools:         sampleTools,
			toolChoice:    "auto",
			wantStripped:  true,
			wantToolsNil:  true,
			wantChoiceNil: true,
		},
		{
			name:          "ollama llama3.1 with tool support keeps tools",
			model:         "llama3.1",
			tools:         sampleTools,
			toolChoice:    "auto",
			wantStripped:  false,
			wantToolsNil:  false,
			wantChoiceNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Build a fresh request for each test case to avoid mutation across tests
			req := &models.ChatCompletionRequest{
				Model: tt.model,
				Messages: []models.Message{
					{Role: "user", Content: "test"},
				},
			}

			// Copy tools so mutations don't affect the shared slice
			if tt.tools != nil {
				toolsCopy := make([]models.Tool, len(tt.tools))
				copy(toolsCopy, tt.tools)
				req.Tools = toolsCopy
			}

			req.ToolChoice = tt.toolChoice

			got := StripUnsupportedParams(tt.model, req)

			if got != tt.wantStripped {
				t.Errorf("StripUnsupportedParams() returned %v, want %v", got, tt.wantStripped)
			}

			if tt.wantToolsNil && req.Tools != nil {
				t.Error("expected Tools to be nil after stripping")
			}

			if !tt.wantToolsNil && req.Tools == nil {
				t.Error("expected Tools to not be nil")
			}

			if tt.wantChoiceNil && req.ToolChoice != nil {
				t.Error("expected ToolChoice to be nil after stripping")
			}

			if !tt.wantChoiceNil && req.ToolChoice == nil {
				t.Error("expected ToolChoice to not be nil")
			}
		})
	}
}

func TestCapabilityFlags(t *testing.T) {
	// Verify the bitmask values are distinct powers of 2.
	caps := []Capability{CapTools, CapVision, CapJSON, CapStreaming}
	seen := make(map[Capability]bool)

	for _, c := range caps {
		if c == 0 {
			t.Errorf("capability %d should not be 0", c)
		}

		if seen[c] {
			t.Errorf("duplicate capability value: %d", c)
		}

		seen[c] = true

		// Verify it's a power of 2
		if c&(c-1) != 0 {
			t.Errorf("capability %d is not a power of 2", c)
		}
	}
}

func TestCapabilityCombinations(t *testing.T) {
	// Verify that combined capability checks work correctly.
	// gpt-4o should have all four capabilities.
	model := "gpt-4o"

	allCaps := []struct {
		name string
		cap  Capability
	}{
		{"tools", CapTools},
		{"vision", CapVision},
		{"json", CapJSON},
		{"streaming", CapStreaming},
	}

	for _, c := range allCaps {
		t.Run("gpt-4o has "+c.name, func(t *testing.T) {
			if !HasCapability(model, c.cap) {
				t.Errorf("gpt-4o should have %s capability", c.name)
			}
		})
	}
}
