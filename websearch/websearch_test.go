package websearch

import (
	"strings"
	"testing"
)

func TestFormatAsSystemMessage_EmptyResults(t *testing.T) {
	msg := formatAsSystemMessage(nil, "medium")
	if msg.Content != "" {
		t.Error("expected empty content for nil results")
	}

	if msg.Role != "" {
		t.Error("expected empty role for nil results")
	}
}

func TestFormatAsSystemMessage_LowContext(t *testing.T) {
	results := []SearchResult{
		{Title: "Go Programming", URL: "https://go.dev", Snippet: "The Go programming language"},
	}

	msg := formatAsSystemMessage(results, "low")
	if msg.Role != "system" {
		t.Errorf("expected role 'system', got %q", msg.Role)
	}

	if !strings.Contains(msg.Content, "Go Programming") {
		t.Error("expected title in low context")
	}

	if !strings.Contains(msg.Content, "https://go.dev") {
		t.Error("expected URL in low context")
	}

	// Low context should NOT include snippet
	if strings.Contains(msg.Content, "The Go programming language") {
		t.Error("low context should not include snippet")
	}
}

func TestFormatAsSystemMessage_MediumContext(t *testing.T) {
	results := []SearchResult{
		{Title: "Go Programming", URL: "https://go.dev", Snippet: "The Go programming language"},
	}

	msg := formatAsSystemMessage(results, "medium")

	if !strings.Contains(msg.Content, "Go Programming") {
		t.Error("expected title in medium context")
	}

	if !strings.Contains(msg.Content, "The Go programming language") {
		t.Error("expected snippet in medium context")
	}
}

func TestFormatAsSystemMessage_HighContext(t *testing.T) {
	results := []SearchResult{
		{Title: "Go Programming", URL: "https://go.dev", Snippet: "The Go programming language is an open-source project"},
	}

	msg := formatAsSystemMessage(results, "high")

	if !strings.Contains(msg.Content, "**Go Programming**") {
		t.Error("expected bold title in high context")
	}

	if !strings.Contains(msg.Content, "open-source project") {
		t.Error("expected full snippet in high context")
	}
}

func TestFormatAsSystemMessage_DefaultIsMedium(t *testing.T) {
	results := []SearchResult{
		{Title: "Test", URL: "https://test.com", Snippet: "snippet"},
	}

	medium := formatAsSystemMessage(results, "medium")
	defaultCtx := formatAsSystemMessage(results, "")

	if medium.Content != defaultCtx.Content {
		t.Error("default context size should be medium")
	}
}

func TestFormatAsSystemMessage_LongSnippetTruncated(t *testing.T) {
	longSnippet := strings.Repeat("a", 300)
	results := []SearchResult{
		{Title: "Test", URL: "https://test.com", Snippet: longSnippet},
	}

	msg := formatAsSystemMessage(results, "medium")

	// Medium context truncates snippets > 200 chars
	if !strings.Contains(msg.Content, "...") {
		t.Error("expected truncation marker in medium context for long snippets")
	}
}

func TestFormatAsSystemMessage_HighContextPreservesLongSnippet(t *testing.T) {
	longSnippet := strings.Repeat("a", 300)
	results := []SearchResult{
		{Title: "Test", URL: "https://test.com", Snippet: longSnippet},
	}

	msg := formatAsSystemMessage(results, "high")

	// High context should NOT truncate
	if strings.Contains(msg.Content, "...") {
		t.Error("high context should not truncate snippets")
	}
}

func TestFormatAsSystemMessage_MultipleResults(t *testing.T) {
	results := []SearchResult{
		{Title: "Result 1", URL: "https://one.com", Snippet: "First"},
		{Title: "Result 2", URL: "https://two.com", Snippet: "Second"},
		{Title: "Result 3", URL: "https://three.com", Snippet: "Third"},
	}

	msg := formatAsSystemMessage(results, "medium")

	if !strings.Contains(msg.Content, "1. ") {
		t.Error("expected numbered results")
	}

	if !strings.Contains(msg.Content, "2. ") {
		t.Error("expected result 2")
	}

	if !strings.Contains(msg.Content, "3. ") {
		t.Error("expected result 3")
	}
}

func TestFormatAsSystemMessage_CiteInstructions(t *testing.T) {
	results := []SearchResult{
		{Title: "Test", URL: "https://test.com", Snippet: "content"},
	}

	msg := formatAsSystemMessage(results, "medium")

	if !strings.Contains(msg.Content, "Cite sources") {
		t.Error("expected citation instructions")
	}
}

func TestFormatAsSystemMessage_EmptySnippet(t *testing.T) {
	results := []SearchResult{
		{Title: "No Snippet", URL: "https://test.com", Snippet: ""},
	}

	// Should not panic and should still have content
	msg := formatAsSystemMessage(results, "medium")
	if msg.Content == "" {
		t.Error("expected non-empty content even with empty snippet")
	}

	msg = formatAsSystemMessage(results, "high")
	if msg.Content == "" {
		t.Error("expected non-empty content even with empty snippet in high context")
	}
}

func TestBuildAnnotations_Empty(t *testing.T) {
	annotations := buildAnnotations(nil)
	if len(annotations) != 0 {
		t.Errorf("expected 0 annotations, got %d", len(annotations))
	}
}

func TestBuildAnnotations(t *testing.T) {
	results := []SearchResult{
		{Title: "Go", URL: "https://go.dev", Snippet: "Go lang"},
		{Title: "Rust", URL: "https://rust-lang.org", Snippet: "Rust lang"},
	}

	annotations := buildAnnotations(results)
	if len(annotations) != 2 {
		t.Fatalf("expected 2 annotations, got %d", len(annotations))
	}

	if annotations[0].Type != "url_citation" {
		t.Errorf("expected type 'url_citation', got %q", annotations[0].Type)
	}

	if annotations[0].URLCitation == nil {
		t.Fatal("expected non-nil URLCitation")
	}

	if annotations[0].URLCitation.URL != "https://go.dev" {
		t.Errorf("expected URL 'https://go.dev', got %q", annotations[0].URLCitation.URL)
	}

	if annotations[0].URLCitation.Title != "Go" {
		t.Errorf("expected title 'Go', got %q", annotations[0].URLCitation.Title)
	}

	if annotations[1].URLCitation.URL != "https://rust-lang.org" {
		t.Errorf("expected second URL 'https://rust-lang.org', got %q", annotations[1].URLCitation.URL)
	}
}

func TestNewRegistry(t *testing.T) {
	reg := NewRegistry("searxng")
	if reg == nil {
		t.Fatal("expected non-nil registry")
	}

	if reg.defaultName != "searxng" {
		t.Errorf("expected default 'searxng', got %q", reg.defaultName)
	}
}

func TestRegistry_Get_NoClients(t *testing.T) {
	reg := NewRegistry("searxng")

	_, err := reg.Get("searxng")
	if err == nil {
		t.Error("expected error when no clients registered")
	}
}

func TestParseConfig(t *testing.T) {
	getter := func(key, def string) string {
		m := map[string]string{
			"WEBSEARCH_ENABLED":     "true",
			"WEBSEARCH_PROVIDER":    "tavily",
			"WEBSEARCH_BASE_URL":    "https://search.example.com",
			"WEBSEARCH_API_KEY":     "sk-search-key",
			"WEBSEARCH_MAX_RESULTS": "10",
			"WEBSEARCH_CACHE_TTL":   "600",
			"WEBSEARCH_QUERY_MODEL": "openai/gpt-4o-mini",
		}
		if v, ok := m[key]; ok {
			return v
		}
		return def
	}

	cfg := ParseConfig(getter)

	if !cfg.Enabled {
		t.Error("expected enabled=true")
	}

	if cfg.Provider != "tavily" {
		t.Errorf("expected provider 'tavily', got %q", cfg.Provider)
	}

	if cfg.BaseURL != "https://search.example.com" {
		t.Errorf("expected base_url, got %q", cfg.BaseURL)
	}

	if cfg.APIKey != "sk-search-key" {
		t.Errorf("expected api_key, got %q", cfg.APIKey)
	}

	if cfg.MaxResults != 10 {
		t.Errorf("expected max_results 10, got %d", cfg.MaxResults)
	}

	if cfg.CacheTTL != 600 {
		t.Errorf("expected cache_ttl 600, got %d", cfg.CacheTTL)
	}

	if cfg.QueryModel != "openai/gpt-4o-mini" {
		t.Errorf("expected query_model, got %q", cfg.QueryModel)
	}
}

func TestParseConfig_Defaults(t *testing.T) {
	getter := func(key, def string) string { return def }

	cfg := ParseConfig(getter)

	if cfg.Enabled {
		t.Error("expected enabled=false by default")
	}

	if cfg.Provider != "searxng" {
		t.Errorf("expected default provider 'searxng', got %q", cfg.Provider)
	}

	if cfg.MaxResults != 5 {
		t.Errorf("expected default max_results 5, got %d", cfg.MaxResults)
	}

	if cfg.CacheTTL != 300 {
		t.Errorf("expected default cache_ttl 300, got %d", cfg.CacheTTL)
	}
}
