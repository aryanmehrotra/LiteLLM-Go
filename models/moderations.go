package models

// ModerationRequest is the OpenAI-compatible moderation request format.
type ModerationRequest struct {
	Input string `json:"input"`
	Model string `json:"model,omitempty"`
}

// ModerationResponse is the OpenAI-compatible moderation response format.
type ModerationResponse struct {
	ID      string             `json:"id"`
	Model   string             `json:"model"`
	Results []ModerationResult `json:"results"`
}

// ModerationResult represents the moderation result for a single input.
type ModerationResult struct {
	Flagged        bool               `json:"flagged"`
	Categories     map[string]bool    `json:"categories"`
	CategoryScores map[string]float64 `json:"category_scores"`
}
