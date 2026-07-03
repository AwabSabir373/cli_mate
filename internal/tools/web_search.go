package tools

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type WebSearchTool struct {
	HTTPClient *http.Client
}

func NewWebSearchTool() *WebSearchTool {
	return &WebSearchTool{
		HTTPClient: &http.Client{Timeout: 15 * time.Second},
	}
}

func (t *WebSearchTool) Name() string {
	return "web_search"
}

func (t *WebSearchTool) Definition() Definition {
	return Definition{
		Name:        t.Name(),
		Description: "Search the web using DuckDuckGo. Returns search results with titles, URLs, and snippets.",
		Schema: map[string]any{
			"type":     "object",
			"required": []string{"query"},
			"properties": map[string]any{
				"query": map[string]any{
					"type":        "string",
					"description": "Search query string (minimum 2 characters)",
				},
				"max_results": map[string]any{
					"type":        "integer",
					"description": "Maximum number of results to return (default: 5, max: 10)",
				},
			},
			"description": "Search the web using DuckDuckGo. Returns search results with titles, URLs, and snippets.",
		},
	}
}

type searchResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
}

func (t *WebSearchTool) Execute(ctx context.Context, call Call) (Result, error) {
	query, _ := call.Argument["query"].(string)
	if strings.TrimSpace(query) == "" {
		return Result{Error: "query is required"}, fmt.Errorf("query is required")
	}
	if len(strings.TrimSpace(query)) < 2 {
		return Result{Error: "query must be at least 2 characters"}, fmt.Errorf("query must be at least 2 characters")
	}

	maxResults := 5
	if mr, ok := call.Argument["max_results"].(float64); ok {
		maxResults = int(mr)
		if maxResults < 1 {
			maxResults = 1
		}
		if maxResults > 10 {
			maxResults = 10
		}
	}

	results, err := t.searchDuckDuckGo(ctx, query, maxResults)
	if err != nil {
		return Result{Error: fmt.Sprintf("search failed: %v", err)}, err
	}

	if len(results) == 0 {
		return Result{Content: "No search results found for: " + query}, nil
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Search results for: %s\n\n", query)
	for i, r := range results {
		fmt.Fprintf(&b, "%d. %s\n   %s\n   %s\n\n", i+1, r.Title, r.URL, r.Snippet)
	}

	return Result{Content: b.String()}, nil
}

func (t *WebSearchTool) searchDuckDuckGo(ctx context.Context, query string, maxResults int) ([]searchResult, error) {
	searchURL := "https://html.duckduckgo.com/html/?q=" + url.QueryEscape(query)

	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

	resp, err := t.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	return parseDuckDuckGoHTML(string(body), maxResults), nil
}

func parseDuckDuckGoHTML(html string, maxResults int) []searchResult {
	var results []searchResult

	parts := strings.Split(html, "class=\"result__body\"")
	if len(parts) < 2 {
		parts = strings.Split(html, "class=\"result ")
	}

	for i := 1; i < len(parts) && len(results) < maxResults; i++ {
		block := parts[i]

		titleStart := strings.Index(block, "class=\"result__a\"")
		if titleStart == -1 {
			titleStart = strings.Index(block, "result__title")
		}
		if titleStart == -1 {
			continue
		}
		titleBlock := block[titleStart:]
		title := extractTextBetween(titleBlock, ">", "</a>")
		title = strings.TrimSpace(title)
		if title == "" {
			continue
		}

		urlStart := strings.Index(block, "class=\"result__url\"")
		if urlStart == -1 {
			urlStart = strings.Index(block, "result__url")
		}
		var resultURL string
		if urlStart != -1 {
			urlBlock := block[urlStart:]
			resultURL = extractTextBetween(urlBlock, ">", "</a>")
			resultURL = strings.TrimSpace(resultURL)
			if resultURL == "" {
				hrefStart := strings.Index(urlBlock, "href=\"")
				if hrefStart != -1 {
					hrefBlock := urlBlock[hrefStart+6:]
					hrefEnd := strings.Index(hrefBlock, "\"")
					if hrefEnd != -1 {
						resultURL = hrefBlock[:hrefEnd]
					}
				}
			}
		}

		snippetStart := strings.Index(block, "class=\"result__snippet\"")
		if snippetStart == -1 {
			snippetStart = strings.Index(block, "result__snippet")
		}
		snippet := ""
		if snippetStart != -1 {
			snippetBlock := block[snippetStart:]
			snippet = extractTextBetween(snippetBlock, ">", "</a>")
			if snippet == "" {
				snippet = extractTextBetween(snippetBlock, ">", "</td>")
			}
			snippet = strings.TrimSpace(snippet)
		}

		if title != "" {
			results = append(results, searchResult{
				Title:   title,
				URL:     resultURL,
				Snippet: snippet,
			})
		}
	}

	return results
}

func extractTextBetween(s, startDelim, endDelim string) string {
	start := strings.Index(s, startDelim)
	if start == -1 {
		return ""
	}
	s = s[start+len(startDelim):]
	end := strings.Index(s, endDelim)
	if end == -1 {
		return s
	}
	return s[:end]
}
