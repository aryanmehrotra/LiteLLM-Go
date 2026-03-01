package handler

import (
	"gofr.dev/pkg/gofr"
	gofrHTTP "gofr.dev/pkg/gofr/http"
	"gofr.dev/pkg/gofr/http/response"

	"examples/llm-gateway/models"
)

// SpeechProvider is an optional interface for providers supporting text-to-speech.
type SpeechProvider interface {
	Speech(ctx *gofr.Context, req models.SpeechRequest) ([]byte, string, error) // returns audio bytes, content-type, error
}

// TranscriptionProvider is an optional interface for providers supporting speech-to-text.
type TranscriptionProvider interface {
	Transcription(ctx *gofr.Context, req models.TranscriptionRequest) (*models.TranscriptionResponse, error)
}

// AudioSpeech handles POST /v1/audio/speech.
func (h *APIHandler) AudioSpeech() gofr.Handler {
	return func(ctx *gofr.Context) (any, error) {
		var req models.SpeechRequest
		if err := ctx.Bind(&req); err != nil {
			return nil, gofrHTTP.ErrorInvalidParam{Params: []string{"request body"}}
		}

		if req.Input == "" {
			return nil, gofrHTTP.ErrorMissingParam{Params: []string{"input"}}
		}

		if req.Model == "" {
			return nil, gofrHTTP.ErrorMissingParam{Params: []string{"model"}}
		}

		if req.Voice == "" {
			return nil, gofrHTTP.ErrorMissingParam{Params: []string{"voice"}}
		}

		providerName := "openai"
		modelName := req.Model

		parts := splitModel(req.Model)
		if len(parts) == 2 {
			providerName = parts[0]
			modelName = parts[1]
		}

		req.Model = modelName

		p, ok := h.Registry.GetProvider(providerName)
		if !ok {
			return nil, gofrHTTP.ErrorInvalidParam{Params: []string{"model"}}
		}

		sp, ok := p.(SpeechProvider)
		if !ok {
			return nil, gofrHTTP.ErrorInvalidParam{Params: []string{"model (provider does not support speech)"}}
		}

		audioData, contentType, err := sp.Speech(ctx, req)
		if err != nil {
			ctx.Errorf("speech error: %v", err)
			return nil, err
		}

		_ = contentType

		return response.Raw{Data: audioData}, nil
	}
}

// AudioTranscriptions handles POST /v1/audio/transcriptions.
func (h *APIHandler) AudioTranscriptions() gofr.Handler {
	return func(ctx *gofr.Context) (any, error) {
		return response.Raw{Data: map[string]string{"status": "transcription endpoint registered"}}, nil
	}
}
