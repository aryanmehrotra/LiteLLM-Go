package models

// ImageGenerationRequest is the OpenAI-compatible image generation request.
type ImageGenerationRequest struct {
	Model          string `json:"model,omitempty"`
	Prompt         string `json:"prompt"`
	N              *int   `json:"n,omitempty"`
	Size           string `json:"size,omitempty"`
	Quality        string `json:"quality,omitempty"`
	ResponseFormat string `json:"response_format,omitempty"`
	Style          string `json:"style,omitempty"`
	User           string `json:"user,omitempty"`
}

// ImageResponse is the OpenAI-compatible image generation response.
type ImageResponse struct {
	Created int64       `json:"created"`
	Data    []ImageData `json:"data"`
}

// ImageData represents a single generated image.
type ImageData struct {
	URL           string `json:"url,omitempty"`
	B64JSON       string `json:"b64_json,omitempty"`
	RevisedPrompt string `json:"revised_prompt,omitempty"`
}

// ImageEditRequest is the request for image edits (multipart).
type ImageEditRequest struct {
	Image          []byte `json:"-"`
	Mask           []byte `json:"-"`
	Prompt         string `json:"prompt"`
	Model          string `json:"model,omitempty"`
	N              *int   `json:"n,omitempty"`
	Size           string `json:"size,omitempty"`
	ResponseFormat string `json:"response_format,omitempty"`
}

// ImageVariationRequest is the request for image variations (multipart).
type ImageVariationRequest struct {
	Image          []byte `json:"-"`
	Model          string `json:"model,omitempty"`
	N              *int   `json:"n,omitempty"`
	Size           string `json:"size,omitempty"`
	ResponseFormat string `json:"response_format,omitempty"`
}
