package handler

import (
	"gofr.dev/pkg/gofr"
	gofrHTTP "gofr.dev/pkg/gofr/http"
	"gofr.dev/pkg/gofr/http/response"

	"examples/llm-gateway/models"
	"examples/llm-gateway/provider"
)

// Embeddings handles POST /v1/embeddings.
func (h *APIHandler) Embeddings() gofr.Handler {
	return func(ctx *gofr.Context) (any, error) {
		var req models.EmbeddingRequest
		if err := ctx.Bind(&req); err != nil {
			return nil, gofrHTTP.ErrorInvalidParam{Params: []string{"request body"}}
		}

		if req.Input == nil {
			return nil, gofrHTTP.ErrorMissingParam{Params: []string{"input"}}
		}

		if req.Model == "" {
			return nil, gofrHTTP.ErrorMissingParam{Params: []string{"model"}}
		}

		p, modelName, err := h.Registry.ResolveProvider(req.Model)
		if err != nil {
			return nil, gofrHTTP.ErrorInvalidParam{Params: []string{"model"}}
		}

		ep, ok := p.(provider.EmbeddingProvider)
		if !ok {
			return nil, gofrHTTP.ErrorInvalidParam{Params: []string{"model (provider does not support embeddings)"}}
		}

		req.Model = modelName

		resp, err := ep.Embedding(ctx, req)
		if err != nil {
			ctx.Errorf("embedding error: %v", err)
			return nil, err
		}

		return response.Raw{Data: resp}, nil
	}
}
