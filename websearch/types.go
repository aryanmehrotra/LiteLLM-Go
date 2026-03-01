package websearch

import (
	"strconv"

	"gofr.dev/pkg/gofr"
)

// SearchResult represents a single web search result.
type SearchResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
}

// SearchClient is the interface all search backends must implement.
type SearchClient interface {
	Name() string
	Search(ctx *gofr.Context, query string, maxResults int) ([]SearchResult, error)
}

// Config holds web search configuration parsed from environment variables.
type Config struct {
	Enabled    bool
	Provider   string // "searxng", "tavily", "duckduckgo"
	BaseURL    string // SearXNG instance URL
	APIKey     string // Tavily API key
	MaxResults int    // max results to inject
	CacheTTL   int    // cache TTL in seconds
	QueryModel string // model for query formulation (empty = use request's model)
}

// ParseConfig reads web search env vars.
func ParseConfig(getOrDefault func(string, string) string) *Config {
	maxResults, _ := strconv.Atoi(getOrDefault("WEBSEARCH_MAX_RESULTS", "5"))
	cacheTTL, _ := strconv.Atoi(getOrDefault("WEBSEARCH_CACHE_TTL", "300"))

	return &Config{
		Enabled:    getOrDefault("WEBSEARCH_ENABLED", "false") == "true",
		Provider:   getOrDefault("WEBSEARCH_PROVIDER", "searxng"),
		BaseURL:    getOrDefault("WEBSEARCH_BASE_URL", "http://localhost:8080"),
		APIKey:     getOrDefault("WEBSEARCH_API_KEY", ""),
		MaxResults: maxResults,
		CacheTTL:   cacheTTL,
		QueryModel: getOrDefault("WEBSEARCH_QUERY_MODEL", ""),
	}
}
