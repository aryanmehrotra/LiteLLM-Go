package websearch

import (
	"encoding/json"
	"fmt"
	"io"

	"gofr.dev/pkg/gofr"
)

// Tavily implements SearchClient using the Tavily AI search API.
type Tavily struct {
	APIKey string
}

// NewTavily creates a new Tavily search client.
func NewTavily(apiKey string) *Tavily {
	return &Tavily{APIKey: apiKey}
}

// Name returns the backend name.
func (t *Tavily) Name() string { return "tavily" }

// Search performs a web search via Tavily's API.
func (t *Tavily) Search(ctx *gofr.Context, query string, maxResults int) ([]SearchResult, error) {
	svc := ctx.GetHTTPService("websearch")

	body, err := json.Marshal(map[string]any{
		"api_key":     t.APIKey,
		"query":       query,
		"max_results": maxResults,
	})
	if err != nil {
		return nil, fmt.Errorf("tavily marshal: %w", err)
	}

	resp, err := svc.PostWithHeaders(ctx, "search", nil, body, map[string]string{
		"Content-Type": "application/json",
	})
	if err != nil {
		return nil, fmt.Errorf("tavily search: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("tavily read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("tavily returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var parsed struct {
		Results []struct {
			Title   string `json:"title"`
			URL     string `json:"url"`
			Content string `json:"content"`
		} `json:"results"`
	}

	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, fmt.Errorf("tavily parse: %w", err)
	}

	results := make([]SearchResult, 0, len(parsed.Results))

	for _, r := range parsed.Results {
		results = append(results, SearchResult{
			Title:   r.Title,
			URL:     r.URL,
			Snippet: r.Content,
		})
	}

	return results, nil
}
