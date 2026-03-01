package provider

import (
	"fmt"
	"time"
)

// NewCloudflare creates a Cloudflare Workers AI provider using the shared
// OpenAI-compatible base. The Cloudflare AI Gateway endpoint embeds the
// account ID in the URL path:
//
//	https://api.cloudflare.com/client/v4/accounts/{accountID}/ai/v1/chat/completions
//
// accountID is the Cloudflare account identifier (found in the dashboard).
// apiToken is a Cloudflare API token with the "Workers AI" permission.
func NewCloudflare(apiToken, accountID string, timeout time.Duration) *OpenAICompatible {
	chatPath := fmt.Sprintf("client/v4/accounts/%s/ai/v1/chat/completions", accountID)

	p := NewOpenAICompatible(
		"cloudflare", "cloudflare", apiToken,
		[]string{
			"@cf/meta/llama-3.3-70b-instruct-fp8-fast",
			"@cf/meta/llama-3.1-70b-instruct",
			"@cf/meta/llama-3.1-8b-instruct",
			"@cf/mistral/mistral-7b-instruct-v0.2",
			"@cf/qwen/qwen1.5-14b-chat-awq",
		},
		timeout,
	)
	p.SetChatPath(chatPath)

	return p
}
