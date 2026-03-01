package models

// SpeechRequest is the OpenAI-compatible text-to-speech request.
type SpeechRequest struct {
	Model          string  `json:"model"`
	Input          string  `json:"input"`
	Voice          string  `json:"voice"`
	ResponseFormat string  `json:"response_format,omitempty"`
	Speed          float64 `json:"speed,omitempty"`
}

// TranscriptionRequest is the OpenAI-compatible audio transcription request.
type TranscriptionRequest struct {
	File           []byte  `json:"-"`
	Model          string  `json:"model"`
	Language       string  `json:"language,omitempty"`
	Prompt         string  `json:"prompt,omitempty"`
	ResponseFormat string  `json:"response_format,omitempty"`
	Temperature    float64 `json:"temperature,omitempty"`
}

// TranscriptionResponse is the OpenAI-compatible audio transcription response.
type TranscriptionResponse struct {
	Text string `json:"text"`
}
