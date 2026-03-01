package handler

import (
	"gofr.dev/pkg/gofr"
	gofrHTTP "gofr.dev/pkg/gofr/http"
	"gofr.dev/pkg/gofr/http/response"

	"aryanmehrotra/litellm-go/models"
)

// ImageProvider is an optional interface for providers supporting image generation.
type ImageProvider interface {
	ImageGeneration(ctx *gofr.Context, req models.ImageGenerationRequest) (*models.ImageResponse, error)
}

// ImageGenerations handles POST /v1/images/generations.
func (h *APIHandler) ImageGenerations() gofr.Handler {
	return func(ctx *gofr.Context) (any, error) {
		var req models.ImageGenerationRequest
		if err := ctx.Bind(&req); err != nil {
			return nil, gofrHTTP.ErrorInvalidParam{Params: []string{"request body"}}
		}

		if req.Prompt == "" {
			return nil, gofrHTTP.ErrorMissingParam{Params: []string{"prompt"}}
		}

		providerName := "openai"
		modelName := req.Model

		if req.Model != "" {
			parts := splitModel(req.Model)
			if len(parts) == 2 {
				providerName = parts[0]
				modelName = parts[1]
			}
		}

		req.Model = modelName

		p, ok := h.Registry.GetProvider(providerName)
		if !ok {
			return nil, gofrHTTP.ErrorInvalidParam{Params: []string{"model"}}
		}

		ip, ok := p.(ImageProvider)
		if !ok {
			return nil, gofrHTTP.ErrorInvalidParam{Params: []string{"model (provider does not support image generation)"}}
		}

		resp, err := ip.ImageGeneration(ctx, req)
		if err != nil {
			ctx.Errorf("image generation error: %v", err)
			return nil, err
		}

		return response.Raw{Data: resp}, nil
	}
}

// ImageEdits handles POST /v1/images/edits.
func (h *APIHandler) ImageEdits() gofr.Handler {
	return func(ctx *gofr.Context) (any, error) {
		return response.Raw{Data: map[string]string{"status": "image edit endpoint registered"}}, nil
	}
}

// ImageVariations handles POST /v1/images/variations.
func (h *APIHandler) ImageVariations() gofr.Handler {
	return func(ctx *gofr.Context) (any, error) {
		return response.Raw{Data: map[string]string{"status": "image variation endpoint registered"}}, nil
	}
}
