package websearch

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"

	"gofr.dev/pkg/gofr"
)

// DuckDuckGo implements SearchClient using the DuckDuckGo Instant Answer API.
// This is a free, no-API-key fallback. Results are limited compared to SearXNG/Tavily.
type DuckDuckGo struct{}

// Name returns the backend name.
func (d *DuckDuckGo) Name() string { return "duckduckgo" }

// Search performs a web search via DuckDuckGo's Instant Answer API.
func (d *DuckDuckGo) Search(ctx *gofr.Context, query string, maxResults int) ([]SearchResult, error) {
	svc := ctx.GetHTTPService("websearch")

	params := map[string]any{
		"q":       url.QueryEscape(query),
		"format":  "json",
		"no_html": "1",
	}

	resp, err := svc.Get(ctx, "", params)
	if err != nil {
		return nil, fmt.Errorf("duckduckgo search: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("duckduckgo read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("duckduckgo returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var parsed struct {
		Abstract       string `json:"Abstract"`
		AbstractURL    string `json:"AbstractURL"`
		AbstractSource string `json:"AbstractSource"`
		RelatedTopics  []struct {
			Text     string `json:"Text"`
			FirstURL string `json:"FirstURL"`
		} `json:"RelatedTopics"`
	}

	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, fmt.Errorf("duckduckgo parse: %w", err)
	}

	var results []SearchResult

	// Add abstract as first result if available
	if parsed.Abstract != "" {
		results = append(results, SearchResult{
			Title:   parsed.AbstractSource,
			URL:     parsed.AbstractURL,
			Snippet: parsed.Abstract,
		})
	}

	// Add related topics
	for _, topic := range parsed.RelatedTopics {
		if len(results) >= maxResults {
			break
		}

		if topic.FirstURL == "" || topic.Text == "" {
			continue
		}

		results = append(results, SearchResult{
			Title:   topic.Text,
			URL:     topic.FirstURL,
			Snippet: topic.Text,
		})
	}

	return results, nil
}
