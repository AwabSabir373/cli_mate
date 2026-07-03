package tools

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"
)

type WebFetchTool struct {
	HTTPClient *http.Client
}

func NewWebFetchTool() *WebFetchTool {
	return &WebFetchTool{
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
	}
}

func (t *WebFetchTool) Name() string {
	return "web_fetch"
}

func (t *WebFetchTool) Definition() Definition {
	return Definition{
		Name:        t.Name(),
		Description: "Fetch a URL and extract readable text content. Use this to read documentation, articles, or any web page content.",
		Schema: map[string]any{
			"type":     "object",
			"required": []string{"url"},
			"properties": map[string]any{
				"url": map[string]any{
					"type":        "string",
					"description": "The URL to fetch (must be http:// or https://)",
				},
				"max_chars": map[string]any{
					"type":        "integer",
					"description": "Maximum characters to return (default: 10000, max: 50000)",
				},
			},
			"description": "Fetch a URL and extract readable text content. Returns the main text content of the page.",
		},
	}
}

func (t *WebFetchTool) Execute(ctx context.Context, call Call) (Result, error) {
	rawURL, _ := call.Argument["url"].(string)
	if strings.TrimSpace(rawURL) == "" {
		return Result{Error: "url is required"}, fmt.Errorf("url is required")
	}

	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		return Result{Error: "url must start with http:// or https://"}, fmt.Errorf("url must start with http:// or https://")
	}

	maxChars := 10000
	if mc, ok := call.Argument["max_chars"].(float64); ok {
		maxChars = int(mc)
		if maxChars < 1000 {
			maxChars = 1000
		}
		if maxChars > 50000 {
			maxChars = 50000
		}
	}

	content, err := t.fetchURL(ctx, rawURL, maxChars)
	if err != nil {
		return Result{Error: fmt.Sprintf("fetch failed: %v", err)}, err
	}

	return Result{Content: content}, nil
}

func (t *WebFetchTool) fetchURL(ctx context.Context, rawURL string, maxChars int) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", rawURL, nil)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,text/plain;q=0.8,*/*;q=0.7")

	resp, err := t.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	limitedReader := io.LimitReader(resp.Body, int64(maxChars*3))
	body, err := io.ReadAll(limitedReader)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	contentType := resp.Header.Get("Content-Type")

	var text string
	if strings.Contains(contentType, "text/html") {
		text = extractTextFromHTML(string(body))
	} else if strings.Contains(contentType, "text/plain") || strings.Contains(contentType, "application/json") {
		text = string(body)
	} else {
		text = extractTextFromHTML(string(body))
		if text == "" {
			text = string(body)
		}
	}

	if len([]rune(text)) > maxChars {
		runes := []rune(text)
		text = string(runes[:maxChars]) + "\n\n... (truncated at " + fmt.Sprintf("%d", maxChars) + " characters)"
	}

	if !utf8.ValidString(text) {
		text = strings.ToValidUTF8(text, "?")
	}

	return text, nil
}

func extractTextFromHTML(html string) string {
	text := html
	text = removeTag(text, "script")
	text = removeTag(text, "style")
	text = removeTag(text, "noscript")
	text = removeTag(text, "iframe")
	text = removeTags(text)
	text = decodeHTMLEntities(text)

	lines := strings.Split(text, "\n")
	var cleaned []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			cleaned = append(cleaned, trimmed)
		}
	}
	return strings.Join(cleaned, "\n")
}

func removeTag(html, tag string) string {
	start := strings.ToLower(html)
	tagStart := "<" + tag
	for {
		idx := strings.Index(start, tagStart)
		if idx == -1 {
			break
		}
		closeIdx := strings.Index(html[idx:], ">")
		if closeIdx == -1 {
			break
		}
		closingTag := "</" + tag + ">"
		closingIdx := strings.Index(strings.ToLower(html[idx+closeIdx:]), closingTag)
		if closingIdx == -1 {
			html = html[:idx] + html[idx+closeIdx+1:]
		} else {
			endIdx := idx + closeIdx + 1 + closingIdx + len(closingTag)
			html = html[:idx] + html[endIdx:]
		}
		start = strings.ToLower(html)
	}
	return html
}

func removeTags(html string) string {
	var result strings.Builder
	inTag := false
	for _, r := range html {
		if r == '<' {
			inTag = true
		} else if r == '>' {
			inTag = false
		} else if !inTag {
			result.WriteRune(r)
		}
	}
	return result.String()
}

func decodeHTMLEntities(text string) string {
	text = strings.ReplaceAll(text, "&amp;", "&")
	text = strings.ReplaceAll(text, "&lt;", "<")
	text = strings.ReplaceAll(text, "&gt;", ">")
	text = strings.ReplaceAll(text, "&quot;", `"`)
	text = strings.ReplaceAll(text, "&apos;", "'")
	text = strings.ReplaceAll(text, "&#39;", "'")
	text = strings.ReplaceAll(text, "&nbsp;", " ")
	text = strings.ReplaceAll(text, "&mdash;", "\u2014")
	text = strings.ReplaceAll(text, "&ndash;", "\u2013")
	text = strings.ReplaceAll(text, "&hellip;", "...")
	text = strings.ReplaceAll(text, "&ldquo;", "\u201c")
	text = strings.ReplaceAll(text, "&rdquo;", "\u201d")
	text = strings.ReplaceAll(text, "&lsquo;", "\u2018")
	text = strings.ReplaceAll(text, "&rsquo;", "\u2019")
	return text
}
