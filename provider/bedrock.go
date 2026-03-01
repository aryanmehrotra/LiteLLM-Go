package provider

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"
	"gofr.dev/pkg/gofr"

	"aryanmehrotra/llm-gateway/models"
)

const bedrockServiceName = "bedrock"

// Bedrock implements Provider for AWS Bedrock's Converse API.
// Requests are authenticated with AWS SigV4 signatures computed inline.
type Bedrock struct {
	accessKey string
	secretKey string
	region    string
	timeout   time.Duration
}

// NewBedrock creates a Bedrock provider with AWS credentials.
func NewBedrock(accessKey, secretKey, region string, timeout time.Duration) *Bedrock {
	if region == "" {
		region = "us-east-1"
	}

	return &Bedrock{
		accessKey: accessKey,
		secretKey: secretKey,
		region:    region,
		timeout:   timeout,
	}
}

func (*Bedrock) Name() string { return "bedrock" }

func (*Bedrock) Models() []string {
	return []string{
		"anthropic.claude-3-5-sonnet-20241022-v2:0",
		"anthropic.claude-3-haiku-20240307-v1:0",
		"amazon.nova-pro-v1:0",
		"amazon.nova-lite-v1:0",
		"meta.llama3-70b-instruct-v1:0",
		"mistral.mistral-7b-instruct-v0:2",
	}
}

// --- Bedrock Converse API request/response types ---

type bedrockRequest struct {
	Messages        []bedrockMessage     `json:"messages"`
	System          []bedrockSystemBlock `json:"system,omitempty"`
	InferenceConfig *bedrockInferCfg     `json:"inferenceConfig,omitempty"`
	ToolConfig      *bedrockToolConfig   `json:"toolConfig,omitempty"`
}

type bedrockMessage struct {
	Role    string               `json:"role"`
	Content []bedrockContentItem `json:"content"`
}

type bedrockSystemBlock struct {
	Text string `json:"text"`
}

type bedrockContentItem struct {
	Text       string             `json:"text,omitempty"`
	ToolUse    *bedrockToolUse    `json:"toolUse,omitempty"`
	ToolResult *bedrockToolResult `json:"toolResult,omitempty"`
}

type bedrockToolUse struct {
	ToolUseID string `json:"toolUseId"`
	Name      string `json:"name"`
	Input     any    `json:"input"`
}

type bedrockToolResult struct {
	ToolUseID string               `json:"toolUseId"`
	Content   []bedrockContentItem `json:"content"`
}

type bedrockInferCfg struct {
	MaxTokens   *int     `json:"maxTokens,omitempty"`
	Temperature *float64 `json:"temperature,omitempty"`
	TopP        *float64 `json:"topP,omitempty"`
	StopSeqs    []string `json:"stopSequences,omitempty"`
}

type bedrockToolConfig struct {
	Tools []bedrockTool `json:"tools"`
}

type bedrockTool struct {
	ToolSpec bedrockToolSpec `json:"toolSpec"`
}

type bedrockToolSpec struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	InputSchema any    `json:"inputSchema"`
}

type bedrockResponse struct {
	Output     bedrockOutput `json:"output"`
	StopReason string        `json:"stopReason"`
	Usage      bedrockUsage  `json:"usage"`
}

type bedrockOutput struct {
	Message bedrockMessage `json:"message"`
}

type bedrockUsage struct {
	InputTokens  int `json:"inputTokens"`
	OutputTokens int `json:"outputTokens"`
	TotalTokens  int `json:"totalTokens"`
}

func (b *Bedrock) ChatCompletion(ctx *gofr.Context, req models.ChatCompletionRequest) (*models.ChatCompletionResponse, error) {
	svc := ctx.GetHTTPService(bedrockServiceName)
	if svc == nil {
		return nil, fmt.Errorf("HTTP service %q not registered", bedrockServiceName)
	}

	bReq := translateToBedrock(req)

	body, err := json.Marshal(bReq)
	if err != nil {
		return nil, fmt.Errorf("marshal bedrock request: %w", err)
	}

	// The Converse API path includes the model ID
	path := fmt.Sprintf("model/%s/converse", req.Model)
	host := fmt.Sprintf("bedrock-runtime.%s.amazonaws.com", b.region)

	headers := b.sigV4Headers("POST", host, "/"+path, body)

	resp, err := svc.PostWithHeaders(ctx, path, nil, body, headers)
	if err != nil {
		return nil, fmt.Errorf("bedrock request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read bedrock response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("bedrock returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var bResp bedrockResponse
	if err := json.Unmarshal(respBody, &bResp); err != nil {
		return nil, fmt.Errorf("decode bedrock response: %w", err)
	}

	return translateFromBedrock(bResp, req.Model), nil
}

func translateToBedrock(req models.ChatCompletionRequest) bedrockRequest {
	bReq := bedrockRequest{
		InferenceConfig: &bedrockInferCfg{
			MaxTokens:   req.MaxTokens,
			Temperature: req.Temperature,
			TopP:        req.TopP,
			StopSeqs:    req.Stop,
		},
	}

	// Convert tools
	if len(req.Tools) > 0 {
		var tools []bedrockTool
		for _, t := range req.Tools {
			tools = append(tools, bedrockTool{
				ToolSpec: bedrockToolSpec{
					Name:        t.Function.Name,
					Description: t.Function.Description,
					InputSchema: map[string]any{"json": t.Function.Parameters},
				},
			})
		}

		bReq.ToolConfig = &bedrockToolConfig{Tools: tools}
	}

	for _, m := range req.Messages {
		if m.Role == "system" {
			bReq.System = append(bReq.System, bedrockSystemBlock{Text: m.Content})
			continue
		}

		if m.Role == "tool" {
			bReq.Messages = append(bReq.Messages, bedrockMessage{
				Role: "user",
				Content: []bedrockContentItem{{
					ToolResult: &bedrockToolResult{
						ToolUseID: m.ToolCallID,
						Content:   []bedrockContentItem{{Text: m.Content}},
					},
				}},
			})

			continue
		}

		if m.Role == "assistant" && len(m.ToolCalls) > 0 {
			var content []bedrockContentItem
			if m.Content != "" {
				content = append(content, bedrockContentItem{Text: m.Content})
			}

			for _, tc := range m.ToolCalls {
				var input any

				if err := json.Unmarshal([]byte(tc.Function.Arguments), &input); err != nil {
					input = tc.Function.Arguments
				}

				content = append(content, bedrockContentItem{
					ToolUse: &bedrockToolUse{
						ToolUseID: tc.ID,
						Name:      tc.Function.Name,
						Input:     input,
					},
				})
			}

			bReq.Messages = append(bReq.Messages, bedrockMessage{
				Role:    "assistant",
				Content: content,
			})

			continue
		}

		bReq.Messages = append(bReq.Messages, bedrockMessage{
			Role:    m.Role,
			Content: []bedrockContentItem{{Text: m.Content}},
		})
	}

	return bReq
}

func translateFromBedrock(resp bedrockResponse, model string) *models.ChatCompletionResponse {
	var content string
	var toolCalls []models.ToolCall

	for _, item := range resp.Output.Message.Content {
		if item.ToolUse != nil {
			argsBytes, _ := json.Marshal(item.ToolUse.Input)
			toolCalls = append(toolCalls, models.ToolCall{
				ID:   item.ToolUse.ToolUseID,
				Type: "function",
				Function: models.FunctionCall{
					Name:      item.ToolUse.Name,
					Arguments: string(argsBytes),
				},
			})
		} else if item.Text != "" {
			content += item.Text
		}
	}

	finishReason := translateBedrockStopReason(resp.StopReason)
	if len(toolCalls) > 0 {
		finishReason = "tool_calls"
	}

	return &models.ChatCompletionResponse{
		ID:      "chatcmpl-" + uuid.NewString(),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   model,
		Choices: []models.Choice{
			{
				Index:        0,
				Message:      models.Message{Role: "assistant", Content: content},
				FinishReason: finishReason,
				ToolCalls:    toolCalls,
			},
		},
		Usage: models.Usage{
			PromptTokens:     resp.Usage.InputTokens,
			CompletionTokens: resp.Usage.OutputTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		},
		Provider: "bedrock",
	}
}

func translateBedrockStopReason(reason string) string {
	switch reason {
	case "end_turn":
		return "stop"
	case "max_tokens":
		return "length"
	case "tool_use":
		return "tool_calls"
	case "guardrail_intervened", "content_filtered":
		return "content_filter"
	default:
		return "stop"
	}
}

// sigV4Headers computes the AWS SigV4 authorization headers for a request.
// Only content-type, host, x-amz-content-sha256, and x-amz-date are signed.
func (b *Bedrock) sigV4Headers(method, host, uriPath string, body []byte) map[string]string {
	now := time.Now().UTC()
	amzDate := now.Format("20060102T150405Z")
	dateStamp := now.Format("20060102")

	bodyHash := sha256Hex(body)

	// Canonical headers (alphabetically sorted, lowercase names)
	canonicalHeaders := "content-type:application/json\n" +
		"host:" + host + "\n" +
		"x-amz-content-sha256:" + bodyHash + "\n" +
		"x-amz-date:" + amzDate + "\n"
	signedHeaders := "content-type;host;x-amz-content-sha256;x-amz-date"

	// Canonical request
	canonicalRequest := strings.Join([]string{
		method,
		uriPath,
		"", // empty query string
		canonicalHeaders,
		signedHeaders,
		bodyHash,
	}, "\n")

	// Credential scope
	scope := dateStamp + "/" + b.region + "/bedrock/aws4_request"

	// String to sign
	stringToSign := "AWS4-HMAC-SHA256\n" +
		amzDate + "\n" +
		scope + "\n" +
		sha256Hex([]byte(canonicalRequest))

	// Signing key
	kDate := hmacSHA256([]byte("AWS4"+b.secretKey), dateStamp)
	kRegion := hmacSHA256(kDate, b.region)
	kService := hmacSHA256(kRegion, "bedrock")
	kSigning := hmacSHA256(kService, "aws4_request")

	signature := fmt.Sprintf("%x", hmacSHA256(kSigning, stringToSign))

	authorization := fmt.Sprintf(
		"AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		b.accessKey, scope, signedHeaders, signature,
	)

	return map[string]string{
		"Content-Type":         "application/json",
		"X-Amz-Date":           amzDate,
		"X-Amz-Content-Sha256": bodyHash,
		"Authorization":        authorization,
	}
}

func hmacSHA256(key []byte, data string) []byte {
	h := hmac.New(sha256.New, key)
	h.Write([]byte(data))

	return h.Sum(nil)
}

func sha256Hex(data []byte) string {
	h := sha256.New()
	h.Write(data)

	return fmt.Sprintf("%x", h.Sum(nil))
}
