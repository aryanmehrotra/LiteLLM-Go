package handler

import (
	"gofr.dev/pkg/gofr"
	gofrHTTP "gofr.dev/pkg/gofr/http"
	"gofr.dev/pkg/gofr/http/response"

	"examples/llm-gateway/models"
)

// RerankProvider is an optional interface for providers supporting reranking.
type RerankProvider interface {
	Rerank(ctx *gofr.Context, req models.RerankRequest) (*models.RerankResponse, error)
}

// Rerank handles POST /v1/rerank.
func (h *APIHandler) Rerank() gofr.Handler {
	return func(ctx *gofr.Context) (any, error) {
		var req models.RerankRequest
		if err := ctx.Bind(&req); err != nil {
			return nil, gofrHTTP.ErrorInvalidParam{Params: []string{"request body"}}
		}

		if req.Query == "" {
			return nil, gofrHTTP.ErrorMissingParam{Params: []string{"query"}}
		}

		if len(req.Documents) == 0 {
			return nil, gofrHTTP.ErrorMissingParam{Params: []string{"documents"}}
		}

		if req.Model == "" {
			return nil, gofrHTTP.ErrorMissingParam{Params: []string{"model"}}
		}

		p, modelName, err := h.Registry.ResolveProvider(req.Model)
		if err != nil {
			return nil, gofrHTTP.ErrorInvalidParam{Params: []string{"model"}}
		}

		rp, ok := p.(RerankProvider)
		if !ok {
			return nil, gofrHTTP.ErrorInvalidParam{Params: []string{"model (provider does not support reranking)"}}
		}

		req.Model = modelName

		resp, err := rp.Rerank(ctx, req)
		if err != nil {
			ctx.Errorf("rerank error: %v", err)
			return nil, err
		}

		return response.Raw{Data: resp}, nil
	}
}
