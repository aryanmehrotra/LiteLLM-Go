package provider

import (
	"strings"
	"testing"
	"time"
)

func TestNewProviders(t *testing.T) {
	tests := []struct {
		name        string
		providerFn  func() Provider
		wantName    string
		minModels   int
	}{
		{
			name:       "TogetherAI",
			providerFn: func() Provider { return NewTogetherAI("key", 0) },
			wantName:   "togetherai",
			minModels:  3,
		},
		{
			name:       "Fireworks",
			providerFn: func() Provider { return NewFireworks("key", 0) },
			wantName:   "fireworks",
			minModels:  3,
		},
		{
			name:       "Perplexity",
			providerFn: func() Provider { return NewPerplexity("key", 0) },
			wantName:   "perplexity",
			minModels:  2,
		},
		{
			name:       "xAI",
			providerFn: func() Provider { return NewXAI("key", 0) },
			wantName:   "xai",
			minModels:  2,
		},
		{
			name:       "Mistral",
			providerFn: func() Provider { return NewMistral("key", 0) },
			wantName:   "mistral",
			minModels:  2,
		},
		{
			name:       "Cohere",
			providerFn: func() Provider { return NewCohere("key", 0) },
			wantName:   "cohere",
			minModels:  3,
		},
		{
			name:       "Azure",
			providerFn: func() Provider { return NewAzure("key", "2024-02-01", "gpt-4o,gpt-4o-mini", 0) },
			wantName:   "azure",
			minModels:  2,
		},
		{
			name:       "Bedrock",
			providerFn: func() Provider { return NewBedrock("key", "secret", "us-east-1", 0) },
			wantName:   "bedrock",
			minModels:  4,
		},
		// New providers
		{
			name:       "Cerebras",
			providerFn: func() Provider { return NewCerebras("key", 0) },
			wantName:   "cerebras",
			minModels:  2,
		},
		{
			name:       "SambaNova",
			providerFn: func() Provider { return NewSambaNova("key", 0) },
			wantName:   "sambanova",
			minModels:  3,
		},
		{
			name:       "AI21",
			providerFn: func() Provider { return NewAI21("key", 0) },
			wantName:   "ai21",
			minModels:  2,
		},
		{
			name:       "OpenRouter",
			providerFn: func() Provider { return NewOpenRouter("key", 0) },
			wantName:   "openrouter",
			minModels:  4,
		},
		{
			name:       "Novita",
			providerFn: func() Provider { return NewNovita("key", 0) },
			wantName:   "novita",
			minModels:  3,
		},
		{
			name:       "Nvidia NIM",
			providerFn: func() Provider { return NewNvidianim("key", 0) },
			wantName:   "nvidia",
			minModels:  3,
		},
		{
			name:       "Cloudflare",
			providerFn: func() Provider { return NewCloudflare("token", "acct123", 0) },
			wantName:   "cloudflare",
			minModels:  3,
		},
		{
			name:       "Vertex",
			providerFn: func() Provider { return NewVertex("proj", "us-central1", "token", 0) },
			wantName:   "vertex",
			minModels:  3,
		},
		{
			name:       "HuggingFace",
			providerFn: func() Provider { return NewHuggingFace("key", 0) },
			wantName:   "huggingface",
			minModels:  4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := tt.providerFn()

			if got := p.Name(); got != tt.wantName {
				t.Errorf("Name() = %q, want %q", got, tt.wantName)
			}

			if got := len(p.Models()); got < tt.minModels {
				t.Errorf("len(Models()) = %d, want >= %d", got, tt.minModels)
			}
		})
	}
}

func TestAzureDefaultDeployments(t *testing.T) {
	// When no deployments are specified, should fall back to defaults
	az := NewAzure("key", "", "", 0)

	if len(az.Models()) == 0 {
		t.Error("Azure with empty deployments should have default models")
	}

	if az.apiVersion == "" {
		t.Error("Azure should have a default API version")
	}
}

func TestPerplexityChatPath(t *testing.T) {
	p := NewPerplexity("key", time.Second)

	// Perplexity should override the default v1/chat/completions path
	if p.chatPath != "chat/completions" {
		t.Errorf("Perplexity chatPath = %q, want %q", p.chatPath, "chat/completions")
	}

	if p.chatCompletionsPath() != "chat/completions" {
		t.Errorf("chatCompletionsPath() = %q, want %q", p.chatCompletionsPath(), "chat/completions")
	}
}

func TestDefaultChatPath(t *testing.T) {
	// Standard providers should use the default v1/chat/completions path
	providers := []struct {
		name string
		p    *OpenAICompatible
	}{
		{"openai", NewOpenAI("key", 0)},
		{"groq", NewGroq("key", 0)},
		{"togetherai", NewTogetherAI("key", 0)},
		{"xai", NewXAI("key", 0)},
		{"mistral", NewMistral("key", 0)},
	}

	for _, tt := range providers {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.p.chatCompletionsPath(); got != "v1/chat/completions" {
				t.Errorf("%s chatCompletionsPath() = %q, want %q", tt.name, got, "v1/chat/completions")
			}
		})
	}
}

func TestBedrockSigV4Headers(t *testing.T) {
	b := NewBedrock("AKIAIOSFODNN7EXAMPLE", "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY", "us-east-1", 0)

	body := []byte(`{"messages":[]}`)
	headers := b.sigV4Headers("POST", "bedrock-runtime.us-east-1.amazonaws.com", "/model/test-model/converse", body)

	required := []string{"Content-Type", "X-Amz-Date", "X-Amz-Content-Sha256", "Authorization"}
	for _, h := range required {
		if v, ok := headers[h]; !ok || v == "" {
			t.Errorf("SigV4 header %q missing or empty", h)
		}
	}

	// Authorization header should contain the expected components
	auth := headers["Authorization"]
	if !strings.HasPrefix(auth, "AWS4-HMAC-SHA256 ") {
		t.Errorf("Authorization header should start with AWS4-HMAC-SHA256, got: %s", auth)
	}

	if !strings.Contains(auth, "Credential=AKIAIOSFODNN7EXAMPLE/") {
		t.Errorf("Authorization should contain access key credential, got: %s", auth)
	}

	if !strings.Contains(auth, "us-east-1/bedrock/aws4_request") {
		t.Errorf("Authorization should contain region/service scope, got: %s", auth)
	}

	if !strings.Contains(auth, "SignedHeaders=content-type;host;x-amz-content-sha256;x-amz-date") {
		t.Errorf("Authorization should list signed headers, got: %s", auth)
	}

	if !strings.Contains(auth, "Signature=") {
		t.Errorf("Authorization should contain Signature, got: %s", auth)
	}

	// X-Amz-Date should be in the correct format (YYYYMMDDTHHmmSSZ)
	amzDate := headers["X-Amz-Date"]
	if len(amzDate) != 16 || !strings.HasSuffix(amzDate, "Z") {
		t.Errorf("X-Amz-Date has unexpected format: %q", amzDate)
	}
}

func TestBedrockDefaultRegion(t *testing.T) {
	// Empty region should default to us-east-1
	b := NewBedrock("key", "secret", "", 0)

	if b.region != "us-east-1" {
		t.Errorf("default region = %q, want %q", b.region, "us-east-1")
	}
}

func TestTranslateCohereFinishReason(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"COMPLETE", "stop"},
		{"MAX_TOKENS", "length"},
		{"TOOL_CALL", "tool_calls"},
		{"ERROR", "content_filter"},
		{"ERROR_TOXIC", "content_filter"},
		{"UNKNOWN", "stop"},
	}

	for _, tt := range tests {
		got := translateCohereFinishReason(tt.input)
		if got != tt.want {
			t.Errorf("translateCohereFinishReason(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestTranslateBedrockStopReason(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"end_turn", "stop"},
		{"max_tokens", "length"},
		{"tool_use", "tool_calls"},
		{"guardrail_intervened", "content_filter"},
		{"content_filtered", "content_filter"},
		{"unknown_reason", "stop"},
	}

	for _, tt := range tests {
		got := translateBedrockStopReason(tt.input)
		if got != tt.want {
			t.Errorf("translateBedrockStopReason(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestCloudflareAccountIDInPath(t *testing.T) {
	accountID := "abc123def456"
	p := NewCloudflare("token", accountID, 0)

	wantPath := "client/v4/accounts/" + accountID + "/ai/v1/chat/completions"
	if p.chatPath != wantPath {
		t.Errorf("Cloudflare chatPath = %q, want %q", p.chatPath, wantPath)
	}

	if p.chatCompletionsPath() != wantPath {
		t.Errorf("chatCompletionsPath() = %q, want %q", p.chatCompletionsPath(), wantPath)
	}
}

func TestOpenRouterPath(t *testing.T) {
	p := NewOpenRouter("key", 0)

	if p.chatPath != "api/v1/chat/completions" {
		t.Errorf("OpenRouter chatPath = %q, want %q", p.chatPath, "api/v1/chat/completions")
	}
}

func TestNovitaPath(t *testing.T) {
	p := NewNovita("key", 0)

	if p.chatPath != "v3/openai/chat/completions" {
		t.Errorf("Novita chatPath = %q, want %q", p.chatPath, "v3/openai/chat/completions")
	}
}

func TestVertexPaths(t *testing.T) {
	v := NewVertex("my-project", "europe-west4", "access-token", 0)

	model := "gemini-2.0-flash-001"

	wantPath := "v1/projects/my-project/locations/europe-west4/publishers/google/models/gemini-2.0-flash-001:generateContent"
	if got := v.vertexPath(model); got != wantPath {
		t.Errorf("vertexPath() = %q, want %q", got, wantPath)
	}

	wantStream := wantPath[:len(wantPath)-len(":generateContent")] + ":streamGenerateContent?alt=sse"
	if got := v.vertexStreamPath(model); got != wantStream {
		t.Errorf("vertexStreamPath() = %q, want %q", got, wantStream)
	}
}

func TestVertexDefaultLocation(t *testing.T) {
	v := NewVertex("proj", "", "token", 0)

	if v.location != "us-central1" {
		t.Errorf("default location = %q, want %q", v.location, "us-central1")
	}
}

func TestHuggingFaceName(t *testing.T) {
	h := NewHuggingFace("key", 0)

	if h.Name() != "huggingface" {
		t.Errorf("Name() = %q, want %q", h.Name(), "huggingface")
	}

	if len(h.Models()) < 4 {
		t.Errorf("expected at least 4 models, got %d", len(h.Models()))
	}
}
