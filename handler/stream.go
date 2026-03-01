package handler

import (
	"gofr.dev/pkg/gofr"
	gofrHTTP "gofr.dev/pkg/gofr/http"

	"examples/llm-gateway/guardrails"
	"examples/llm-gateway/middleware"
	"examples/llm-gateway/models"
)

// ChatCompletionStream handles WebSocket streaming at /v1/chat/completions/stream.
// The client sends a ChatCompletionRequest JSON message and receives multiple
// StreamChunk JSON messages back, ending with a "[DONE]" message.
func (h *APIHandler) ChatCompletionStream() gofr.Handler {
	return func(ctx *gofr.Context) (any, error) {
		var req models.ChatCompletionRequest
		if err := ctx.Bind(&req); err != nil {
			return nil, err
		}

		if req.Model == "" {
			return nil, gofrHTTP.ErrorMissingParam{Params: []string{"model"}}
		}

		if len(req.Messages) == 0 {
			return nil, gofrHTTP.ErrorMissingParam{Params: []string{"messages"}}
		}

		// Pre-call guardrails
		keyHash := ""
		if authKey := middleware.GetAuthKey(ctx); authKey != "" {
			keyHash = sha256hex(authKey)
		}

		grCfg := guardrails.LoadConfig(ctx, keyHash, h.Guardrails)
		if err := guardrails.Check(grCfg, req.Messages); err != nil {
			return nil, gofrHTTP.ErrorInvalidParam{Params: []string{err.Error()}}
		}

		// Web search augmentation — non-fatal
		if h.Search != nil {
			h.Search.Augment(ctx, &req)
		}

		sp, modelName, err := h.Registry.ResolveStreamingProvider(req.Model)
		if err != nil {
			return nil, gofrHTTP.ErrorInvalidParam{Params: []string{"model"}}
		}

		// Route through Router (retry + cooldown)
		err = h.Router.ChatCompletionStream(ctx, sp, modelName, req, func(chunk models.StreamChunk) {
			ctx.WriteMessageToSocket(chunk)
		})
		if err != nil {
			return nil, err
		}

		ctx.WriteMessageToSocket("[DONE]")

		return nil, nil
	}
}
