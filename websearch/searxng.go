package websearch

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"

	"gofr.dev/pkg/gofr"
)

// SearXNG implements SearchClient using a SearXNG instance.
type SearXNG struct{}

// Name returns the backend name.
func (s *SearXNG) Name() string { return "searxng" }

// Search performs a web search via SearXNG's JSON API.
func (s *SearXNG) Search(ctx *gofr.Context, query string, maxResults int) ([]SearchResult, error) {
	svc := ctx.GetHTTPService("websearch")

	params := map[string]any{
		"q":          url.QueryEscape(query),
		"format":     "json",
		"categories": "general",
	}

	resp, err := svc.Get(ctx, "search", params)
	if err != nil {
		return nil, fmt.Errorf("searxng search: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("searxng read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("searxng returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var parsed struct {
		Results []struct {
			Title   string `json:"title"`
			URL     string `json:"url"`
			Content string `json:"content"`
		} `json:"results"`
	}

	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, fmt.Errorf("searxng parse: %w", err)
	}

	results := make([]SearchResult, 0, maxResults)

	for i, r := range parsed.Results {
		if i >= maxResults {
			break
		}

		results = append(results, SearchResult{
			Title:   r.Title,
			URL:     r.URL,
			Snippet: r.Content,
		})
	}

	return results, nil
}
