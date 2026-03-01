package routing

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"aryanmehrotra/llm-gateway/models"

	"gofr.dev/pkg/gofr"
)

// mockProvider implements ChatProvider for testing.
type mockProvider struct {
	name string
}

func (m *mockProvider) Name() string { return m.name }
func (m *mockProvider) ChatCompletion(_ *gofr.Context, _ models.ChatCompletionRequest) (*models.ChatCompletionResponse, error) {
	return nil, nil
}

// mockStreamProvider implements StreamChatProvider for testing.
type mockStreamProvider struct {
	mockProvider
}

func (m *mockStreamProvider) ChatCompletionStream(_ *gofr.Context, _ models.ChatCompletionRequest, _ func(models.StreamChunk)) error {
	return nil
}

func TestFilterByTags(t *testing.T) {
	deployments := []Deployment{
		{
			ID:       "us-east-openai",
			Provider: &mockProvider{name: "openai"},
			Tags:     map[string]string{"region": "us-east", "tier": "premium"},
		},
		{
			ID:       "eu-west-openai",
			Provider: &mockProvider{name: "openai"},
			Tags:     map[string]string{"region": "eu-west", "tier": "standard"},
		},
		{
			ID:       "us-east-anthropic",
			Provider: &mockProvider{name: "anthropic"},
			Tags:     map[string]string{"region": "us-east", "tier": "standard"},
		},
	}

	tests := []struct {
		name    string
		tags    map[string]string
		wantIDs []string
	}{
		{
			name:    "nil tags returns all deployments",
			tags:    nil,
			wantIDs: []string{"us-east-openai", "eu-west-openai", "us-east-anthropic"},
		},
		{
			name:    "empty tags returns all deployments",
			tags:    map[string]string{},
			wantIDs: []string{"us-east-openai", "eu-west-openai", "us-east-anthropic"},
		},
		{
			name:    "single tag match",
			tags:    map[string]string{"region": "us-east"},
			wantIDs: []string{"us-east-openai", "us-east-anthropic"},
		},
		{
			name:    "multiple tags match",
			tags:    map[string]string{"region": "us-east", "tier": "premium"},
			wantIDs: []string{"us-east-openai"},
		},
		{
			name:    "no matches falls back to all deployments",
			tags:    map[string]string{"region": "ap-south"},
			wantIDs: []string{"us-east-openai", "eu-west-openai", "us-east-anthropic"},
		},
		{
			name:    "partial tag match filters correctly",
			tags:    map[string]string{"tier": "standard"},
			wantIDs: []string{"eu-west-openai", "us-east-anthropic"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FilterByTags(deployments, tt.tags)

			gotIDs := make([]string, len(result))
			for i, d := range result {
				gotIDs[i] = d.ID
			}

			assert.Equal(t, tt.wantIDs, gotIDs)
		})
	}
}

func TestFilterByTags_EmptyDeployments(t *testing.T) {
	result := FilterByTags(nil, map[string]string{"region": "us-east"})
	assert.Nil(t, result)
}

func TestDeployment_AsStreaming(t *testing.T) {
	t.Run("provider supports streaming", func(t *testing.T) {
		d := Deployment{
			ID:       "stream-provider",
			Provider: &mockStreamProvider{mockProvider{name: "openai"}},
		}

		sd, ok := d.AsStreaming()
		assert.True(t, ok)
		assert.NotNil(t, sd)
		assert.Equal(t, "stream-provider", sd.ID)
		assert.NotNil(t, sd.StreamProvider)
		assert.Equal(t, "openai", sd.StreamProvider.Name())
	})

	t.Run("provider does not support streaming", func(t *testing.T) {
		d := Deployment{
			ID:       "basic-provider",
			Provider: &mockProvider{name: "basic"},
		}

		sd, ok := d.AsStreaming()
		assert.False(t, ok)
		assert.Nil(t, sd)
	})
}

func TestMatchesTags(t *testing.T) {
	tests := []struct {
		name           string
		deploymentTags map[string]string
		requestTags    map[string]string
		want           bool
	}{
		{
			name:           "empty request tags matches everything",
			deploymentTags: map[string]string{"region": "us"},
			requestTags:    map[string]string{},
			want:           true,
		},
		{
			name:           "exact match",
			deploymentTags: map[string]string{"region": "us"},
			requestTags:    map[string]string{"region": "us"},
			want:           true,
		},
		{
			name:           "superset deployment tags matches",
			deploymentTags: map[string]string{"region": "us", "tier": "premium"},
			requestTags:    map[string]string{"region": "us"},
			want:           true,
		},
		{
			name:           "missing tag in deployment",
			deploymentTags: map[string]string{"region": "us"},
			requestTags:    map[string]string{"region": "us", "tier": "premium"},
			want:           false,
		},
		{
			name:           "value mismatch",
			deploymentTags: map[string]string{"region": "eu"},
			requestTags:    map[string]string{"region": "us"},
			want:           false,
		},
		{
			name:           "nil deployment tags with request tags",
			deploymentTags: nil,
			requestTags:    map[string]string{"region": "us"},
			want:           false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchesTags(tt.deploymentTags, tt.requestTags)
			assert.Equal(t, tt.want, got)
		})
	}
}
