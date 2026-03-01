package handler

import (
	"gofr.dev/pkg/gofr"
	gofrHTTP "gofr.dev/pkg/gofr/http"
	"gofr.dev/pkg/gofr/http/response"

	"aryanmehrotra/litellm-go/models"
)

// Completions handles POST /v1/completions (legacy completions API).
// Converts the prompt to a chat message internally and uses the chat completions provider.
func (h *APIHandler) Completions() gofr.Handler {
	return func(ctx *gofr.Context) (any, error) {
		var req models.CompletionRequest
		if err := ctx.Bind(&req); err != nil {
			return nil, gofrHTTP.ErrorInvalidParam{Params: []string{"request body"}}
		}

		if req.Prompt == "" {
			return nil, gofrHTTP.ErrorMissingParam{Params: []string{"prompt"}}
		}

		if req.Model == "" {
			return nil, gofrHTTP.ErrorMissingParam{Params: []string{"model"}}
		}

		p, modelName, err := h.Registry.ResolveProvider(req.Model)
		if err != nil {
			return nil, gofrHTTP.ErrorInvalidParam{Params: []string{"model"}}
		}

		// Convert to chat completion request
		chatReq := models.ChatCompletionRequest{
			Model:            modelName,
			Messages:         []models.Message{{Role: "user", Content: req.Prompt}},
			Temperature:      req.Temperature,
			TopP:             req.TopP,
			MaxTokens:        req.MaxTokens,
			Stop:             req.Stop,
			PresencePenalty:  req.PresencePenalty,
			FrequencyPenalty: req.FrequencyPenalty,
		}

		// Check cache
		if cached, found := h.Cache.Get(ctx, &chatReq); found {
			return response.Raw{Data: toCompletionResponse(cached)}, nil
		}

		chatResp, err := h.Router.ChatCompletion(ctx, p, modelName, chatReq)
		if err != nil {
			ctx.Errorf("completion error: %v", err)
			return nil, err
		}

		h.Cache.Set(ctx, &chatReq, chatResp)

		return response.Raw{Data: toCompletionResponse(chatResp)}, nil
	}
}

func toCompletionResponse(chatResp *models.ChatCompletionResponse) models.CompletionResponse {
	resp := models.CompletionResponse{
		ID:      chatResp.ID,
		Object:  "text_completion",
		Created: chatResp.Created,
		Model:   chatResp.Model,
		Usage:   chatResp.Usage,
	}

	for _, c := range chatResp.Choices {
		resp.Choices = append(resp.Choices, models.CompletionChoice{
			Text:         c.Message.Content,
			Index:        c.Index,
			FinishReason: c.FinishReason,
		})
	}

	return resp
}
