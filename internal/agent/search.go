package agent

import (
	"strings"

	"cli_mate/internal/providers"
)

// SearchResult represents a search match in a message.
type SearchResult struct {
	MessageIndex int
	MessageRole  string
	MatchText    string
	Context      string
}

// SearchMessages searches through messages for matching content.
func SearchMessages(messages []providers.Message, query string) []SearchResult {
	var results []SearchResult
	query = strings.ToLower(query)

	for i, msg := range messages {
		content := strings.ToLower(msg.Content)
		if strings.Contains(content, query) {
			// Find the matching line
			lines := strings.Split(msg.Content, "\n")
			for _, line := range lines {
				if strings.Contains(strings.ToLower(line), query) {
					results = append(results, SearchResult{
						MessageIndex: i,
						MessageRole:  msg.Role,
						MatchText:    strings.TrimSpace(line),
						Context:      msg.Content,
					})
					break
				}
			}
		}
	}

	return results
}

// SearchToolCalls searches through tool calls in messages.
func SearchToolCalls(messages []providers.Message, query string) []SearchResult {
	var results []SearchResult
	query = strings.ToLower(query)

	for i, msg := range messages {
		for _, tc := range msg.ToolCalls {
			if strings.Contains(strings.ToLower(tc.Name), query) ||
				strings.Contains(strings.ToLower(tc.Arguments), query) {
				results = append(results, SearchResult{
					MessageIndex: i,
					MessageRole:  "tool_call",
					MatchText:    tc.Name + ": " + tc.Arguments,
					Context:      msg.Content,
				})
			}
		}
	}

	return results
}

// FormatSearchResult returns a human-readable representation of a search result.
func FormatSearchResult(result SearchResult, index int) string {
	role := result.MessageRole
	if role == "" {
		role = "message"
	}
	return strings.TrimSpace(result.MatchText)
}
