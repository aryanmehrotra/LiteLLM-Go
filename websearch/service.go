package websearch

import (
	"gofr.dev/pkg/gofr"

	"aryanmehrotra/litellm-go/models"
	"aryanmehrotra/litellm-go/provider"
	"aryanmehrotra/litellm-go/routing"
)

// Service encapsulates all web search orchestration.
// Constructed once at startup, injected into handler closures.
type Service struct {
	Registry  *Registry
	Cache     *SearchCache
	Config    *Config
	LLMRouter *routing.Router
	LLMReg    *provider.Registry
}

// Augment is the single entry point called by handlers.
// It performs the full two-pass flow:
//  1. Ask LLM to formulate query (or use last user message directly)
//  2. Check cache → search backend → cache result
//  3. Inject search context into request messages
//  4. Return annotations for the response
//
// Non-fatal: returns nil annotations on any failure (chat proceeds without search).
func (s *Service) Augment(ctx *gofr.Context, req *models.ChatCompletionRequest) []models.Annotation {
	if req.WebSearchOptions == nil || s.Config == nil || !s.Config.Enabled {
		return nil
	}

	// Step 1: Formulate search query from conversation
	query := s.formulateQuery(ctx, req.Messages, req.Model)
	if query == "" || query == "NONE" {
		return nil
	}

	// Step 2: Search (with cache)
	results := s.search(ctx, query)
	if len(results) == 0 {
		return nil
	}

	// Step 3: Inject search context into messages
	contextSize := req.WebSearchOptions.SearchContextSize
	if contextSize == "" {
		contextSize = "medium"
	}

	s.injectContext(req, results, contextSize)

	// Step 4: Build annotations
	annotations := buildAnnotations(results)

	// Strip web_search_options before forwarding to provider
	req.WebSearchOptions = nil

	return annotations
}

// formulateQuery uses a fast LLM call to generate a search query from the conversation,
// or falls back to extracting from the last user message.
func (s *Service) formulateQuery(ctx *gofr.Context, messages []models.Message, requestModel string) string {
	// Find the last user message
	var lastUserMsg string

	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			lastUserMsg = messages[i].Content
			break
		}
	}

	if lastUserMsg == "" {
		return ""
	}

	// Determine which model to use for query formulation
	queryModel := s.Config.QueryModel
	if queryModel == "" {
		queryModel = requestModel
	}

	// Try LLM-based query formulation
	if s.LLMReg != nil && s.LLMRouter != nil {
		formulated := s.llmFormulateQuery(ctx, lastUserMsg, queryModel)
		if formulated != "" {
			return formulated
		}
	}

	// Fallback: use the last user message directly (truncated)
	if len(lastUserMsg) > 200 {
		lastUserMsg = lastUserMsg[:200]
	}

	return lastUserMsg
}

// llmFormulateQuery makes a lightweight LLM call to formulate a search query.
func (s *Service) llmFormulateQuery(ctx *gofr.Context, userMessage, model string) string {
	p, modelName, err := s.LLMReg.ResolveProvider(model)
	if err != nil {
		ctx.Errorf("web search: cannot resolve model %q for query formulation: %v", model, err)
		return ""
	}

	queryReq := models.ChatCompletionRequest{
		Model: modelName,
		Messages: []models.Message{
			{
				Role: "system",
				Content: `You are a search query formulator. Given the user's message, output a concise web search query (1-8 words) that would find the most relevant information.
If the user's message is about coding, math, or something that doesn't need web search, respond with exactly "NONE".
Output ONLY the search query, nothing else.`,
			},
			{
				Role:    "user",
				Content: userMessage,
			},
		},
	}

	resp, err := s.LLMRouter.ChatCompletion(ctx, p, modelName, queryReq)
	if err != nil {
		ctx.Errorf("web search: query formulation LLM call failed: %v", err)
		return ""
	}

	if len(resp.Choices) > 0 {
		return resp.Choices[0].Message.Content
	}

	return ""
}

// SearchDirect performs a web search for the given query, using the cache.
// It is exported for use by handlers that need direct search access (e.g. Responses API).
func (s *Service) SearchDirect(ctx *gofr.Context, query string, maxResults int) []SearchResult {
	if s.Cache != nil {
		if cached, found := s.Cache.Get(ctx, query); found {
			return cached
		}
	}

	client, err := s.Registry.Get(s.Config.Provider)
	if err != nil {
		ctx.Errorf("web search direct: %v", err)
		return nil
	}

	n := maxResults
	if n <= 0 {
		n = s.Config.MaxResults
	}

	results, err := client.Search(ctx, query, n)
	if err != nil {
		ctx.Errorf("web search direct: %v", err)
		return nil
	}

	if s.Cache != nil && len(results) > 0 {
		s.Cache.Set(ctx, query, results)
	}

	return results
}

// search checks the cache first, then calls the search backend.
func (s *Service) search(ctx *gofr.Context, query string) []SearchResult {
	// Check cache
	if s.Cache != nil {
		if cached, found := s.Cache.Get(ctx, query); found {
			return cached
		}
	}

	// Get search client
	client, err := s.Registry.Get(s.Config.Provider)
	if err != nil {
		ctx.Errorf("web search: %v", err)
		return nil
	}

	// Perform search
	results, err := client.Search(ctx, query, s.Config.MaxResults)
	if err != nil {
		ctx.Errorf("web search: %v", err)
		return nil
	}

	// Cache results
	if s.Cache != nil && len(results) > 0 {
		s.Cache.Set(ctx, query, results)
	}

	return results
}

// injectContext prepends search results as a system message.
func (s *Service) injectContext(req *models.ChatCompletionRequest, results []SearchResult, contextSize string) {
	searchMsg := formatAsSystemMessage(results, contextSize)
	if searchMsg.Content == "" {
		return
	}

	// Prepend to existing system message or add new one
	for i, m := range req.Messages {
		if m.Role == "system" {
			req.Messages[i].Content = searchMsg.Content + "\n\n" + m.Content
			return
		}
	}

	req.Messages = append([]models.Message{searchMsg}, req.Messages...)
}
