package handler

import (
	"strings"
	"testing"

	"aryanmehrotra/litellm-go/models"
)

func TestValidateTools(t *testing.T) {
	tests := []struct {
		name    string
		tools   []models.Tool
		wantErr bool
		errMsg  string // substring expected in error message
	}{
		{
			name: "valid single function tool",
			tools: []models.Tool{
				{
					Type: "function",
					Function: models.ToolFunction{
						Name:        "get_weather",
						Description: "Get weather for a location",
						Parameters: map[string]any{
							"type": "object",
							"properties": map[string]any{
								"location": map[string]any{"type": "string"},
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid multiple function tools",
			tools: []models.Tool{
				{
					Type: "function",
					Function: models.ToolFunction{
						Name:        "get_weather",
						Description: "Get weather",
						Parameters: map[string]any{
							"type": "object",
						},
					},
				},
				{
					Type: "function",
					Function: models.ToolFunction{
						Name:        "get_time",
						Description: "Get current time",
						Parameters: map[string]any{
							"type": "object",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid tool without parameters",
			tools: []models.Tool{
				{
					Type: "function",
					Function: models.ToolFunction{
						Name:        "get_random_number",
						Description: "Generate a random number",
					},
				},
			},
			wantErr: false,
		},
		{
			name:    "empty tool list returns nil",
			tools:   []models.Tool{},
			wantErr: false,
		},
		{
			name: "missing function name",
			tools: []models.Tool{
				{
					Type: "function",
					Function: models.ToolFunction{
						Name:        "",
						Description: "A tool with no name",
					},
				},
			},
			wantErr: true,
			errMsg:  "function.name is required",
		},
		{
			name: "wrong type value",
			tools: []models.Tool{
				{
					Type: "webhook",
					Function: models.ToolFunction{
						Name: "my_hook",
					},
				},
			},
			wantErr: true,
			errMsg:  `type must be "function"`,
		},
		{
			name: "empty type value",
			tools: []models.Tool{
				{
					Type: "",
					Function: models.ToolFunction{
						Name: "my_tool",
					},
				},
			},
			wantErr: true,
			errMsg:  `type must be "function"`,
		},
		{
			name: "parameters missing type field",
			tools: []models.Tool{
				{
					Type: "function",
					Function: models.ToolFunction{
						Name: "bad_params",
						Parameters: map[string]any{
							"properties": map[string]any{
								"x": map[string]any{"type": "string"},
							},
						},
					},
				},
			},
			wantErr: true,
			errMsg:  `parameters should have a "type" field`,
		},
		{
			name: "multiple errors reported together",
			tools: []models.Tool{
				{
					Type: "webhook",
					Function: models.ToolFunction{
						Name: "",
					},
				},
			},
			wantErr: true,
			errMsg:  `type must be "function"`,
		},
		{
			name: "mixed valid and invalid tools",
			tools: []models.Tool{
				{
					Type: "function",
					Function: models.ToolFunction{
						Name: "good_tool",
					},
				},
				{
					Type: "function",
					Function: models.ToolFunction{
						Name: "", // invalid
					},
				},
			},
			wantErr: true,
			errMsg:  "tools[1]: function.name is required",
		},
		{
			name: "second tool has wrong type",
			tools: []models.Tool{
				{
					Type: "function",
					Function: models.ToolFunction{
						Name: "first_tool",
					},
				},
				{
					Type: "retrieval",
					Function: models.ToolFunction{
						Name: "second_tool",
					},
				},
			},
			wantErr: true,
			errMsg:  "tools[1]:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTools(tt.tools)

			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
				return
			}

			if !tt.wantErr && err != nil {
				t.Errorf("expected no error, got: %v", err)
				return
			}

			if tt.wantErr && tt.errMsg != "" {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.errMsg)
				}
			}
		})
	}
}

func TestValidateTools_ErrorIndexing(t *testing.T) {
	// Ensure the error message includes the correct tool index
	tools := []models.Tool{
		{Type: "function", Function: models.ToolFunction{Name: "ok_tool_0"}},
		{Type: "function", Function: models.ToolFunction{Name: "ok_tool_1"}},
		{Type: "function", Function: models.ToolFunction{Name: ""}}, // index 2 is bad
	}

	err := ValidateTools(tools)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !strings.Contains(err.Error(), "tools[2]") {
		t.Errorf("error should reference tools[2], got: %v", err)
	}
}

func TestValidateTools_NonMapParameters(t *testing.T) {
	// When parameters is not a map[string]any, the type assertion fails
	// and the validator should not error on the missing "type" field.
	tools := []models.Tool{
		{
			Type: "function",
			Function: models.ToolFunction{
				Name:       "string_params_tool",
				Parameters: "not-a-map",
			},
		},
	}

	err := ValidateTools(tools)
	if err != nil {
		t.Errorf("expected no error for non-map parameters, got: %v", err)
	}
}
