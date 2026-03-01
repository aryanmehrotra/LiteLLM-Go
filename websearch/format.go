package websearch

import (
	"fmt"
	"strings"

	"aryanmehrotra/litellm-go/models"
)

// formatAsSystemMessage converts search results into a system message for injection.
// contextSize controls verbosity: "low", "medium" (default), "high".
func formatAsSystemMessage(results []SearchResult, contextSize string) models.Message {
	if len(results) == 0 {
		return models.Message{}
	}

	var sb strings.Builder

	sb.WriteString("[Web Search Results]\n")
	sb.WriteString("The following web search results are relevant to the user's query.\n\n")

	for i, r := range results {
		switch contextSize {
		case "low":
			// Title + URL only (~50 tokens/result)
			fmt.Fprintf(&sb, "%d. %s (%s)\n\n", i+1, r.Title, r.URL)
		case "high":
			// Title + URL + full snippet (~300 tokens/result)
			fmt.Fprintf(&sb, "%d. **%s** (%s)\n", i+1, r.Title, r.URL)
			if r.Snippet != "" {
				fmt.Fprintf(&sb, "   %s\n\n", r.Snippet)
			} else {
				sb.WriteString("\n")
			}
		default: // "medium"
			// Title + URL + truncated snippet (~150 tokens/result)
			fmt.Fprintf(&sb, "%d. **%s** (%s)\n", i+1, r.Title, r.URL)
			snippet := r.Snippet
			if len(snippet) > 200 {
				snippet = snippet[:200] + "..."
			}
			if snippet != "" {
				fmt.Fprintf(&sb, "   %s\n\n", snippet)
			} else {
				sb.WriteString("\n")
			}
		}
	}

	sb.WriteString("Use the above information to inform your response. Cite sources using [N](URL) format when referencing specific information.")

	return models.Message{
		Role:    "system",
		Content: sb.String(),
	}
}

// buildAnnotations creates citation annotations from search results.
func buildAnnotations(results []SearchResult) []models.Annotation {
	annotations := make([]models.Annotation, 0, len(results))

	for _, r := range results {
		annotations = append(annotations, models.Annotation{
			Type: "url_citation",
			URLCitation: &models.URLCitation{
				URL:   r.URL,
				Title: r.Title,
			},
		})
	}

	return annotations
}
