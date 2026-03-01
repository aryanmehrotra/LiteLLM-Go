package models

// RerankRequest is the rerank API request format.
type RerankRequest struct {
	Model     string   `json:"model"`
	Query     string   `json:"query"`
	Documents []string `json:"documents"`
	TopN      *int     `json:"top_n,omitempty"`
}

// RerankResponse is the rerank API response format.
type RerankResponse struct {
	Model   string         `json:"model"`
	Results []RerankResult `json:"results"`
}

// RerankResult represents a single rerank result.
type RerankResult struct {
	Index          int     `json:"index"`
	RelevanceScore float64 `json:"relevance_score"`
	Document       string  `json:"document,omitempty"`
}
