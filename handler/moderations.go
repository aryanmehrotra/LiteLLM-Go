package handler

import (
	"gofr.dev/pkg/gofr"
	gofrHTTP "gofr.dev/pkg/gofr/http"
	"gofr.dev/pkg/gofr/http/response"

	"aryanmehrotra/litellm-go/models"
)

// ModerationProvider is an optional interface for providers supporting moderations.
type ModerationProvider interface {
	Moderation(ctx *gofr.Context, req models.ModerationRequest) (*models.ModerationResponse, error)
}

// Moderations handles POST /v1/moderations.
func (h *APIHandler) Moderations() gofr.Handler {
	return func(ctx *gofr.Context) (any, error) {
		var req models.ModerationRequest
		if err := ctx.Bind(&req); err != nil {
			return nil, gofrHTTP.ErrorInvalidParam{Params: []string{"request body"}}
		}

		if req.Input == "" {
			return nil, gofrHTTP.ErrorMissingParam{Params: []string{"input"}}
		}

		// Default to OpenAI for moderations
		providerName := "openai"
		if req.Model != "" {
			parts := splitModel(req.Model)
			if len(parts) == 2 {
				providerName = parts[0]
				req.Model = parts[1]
			}
		}

		p, ok := h.Registry.GetProvider(providerName)
		if !ok {
			return nil, gofrHTTP.ErrorInvalidParam{Params: []string{"model"}}
		}

		mp, ok := p.(ModerationProvider)
		if !ok {
			return nil, gofrHTTP.ErrorInvalidParam{Params: []string{"model (provider does not support moderations)"}}
		}

		resp, err := mp.Moderation(ctx, req)
		if err != nil {
			ctx.Errorf("moderation error: %v", err)
			return nil, err
		}

		return response.Raw{Data: resp}, nil
	}
}

func splitModel(model string) []string {
	for i, c := range model {
		if c == '/' {
			return []string{model[:i], model[i+1:]}
		}
	}

	return []string{model}
}
