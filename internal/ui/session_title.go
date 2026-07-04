package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"cli_mate/internal/providers"
)

// sessionTitleGenerator manages AI-generated session titles.
type sessionTitleGenerator struct {
	pending  bool
	title    string
	err      string
	started  time.Time
	digest   string
}

// newSessionTitleGenerator creates a new session title generator.
func newSessionTitleGenerator() *sessionTitleGenerator {
	return &sessionTitleGenerator{}
}

// isPending returns true if a title is being generated.
func (stg *sessionTitleGenerator) isPending() bool {
	return stg.pending
}

// titleOrPending returns the title if already generated, or a pending indicator.
func (stg *sessionTitleGenerator) titleOrPending() string {
	if stg.title != "" {
		return stg.title
	}
	if stg.pending {
		return "..."
	}
	return ""
}

// generateTitle starts asynchronous title generation.
func (stg *sessionTitleGenerator) generateTitle(messages []providers.Message, provider providers.Provider) {
	if stg.pending || len(messages) < 2 {
		return
	}

	stg.pending = true
	stg.started = time.Now()
	stg.digest = buildTitleDigest(messages, 1600)

	if stg.digest == "" {
		stg.pending = false
		stg.title = "New session"
		return
	}

	// Generate title in background using the provider
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		title := callProviderForTitle(ctx, provider, stg.digest)
		if title != "" {
			stg.title = cleanGeneratedTitle(title)
		} else {
			stg.title = generateLocalTitle(stg.digest)
		}
		stg.pending = false
	}()
}

// generateLocalTitle creates a title from the digest without a provider call.
func generateLocalTitle(digest string) string {
	lines := strings.Split(digest, "\n")
	if len(lines) == 0 {
		return "New session"
	}

	// Use the first user message to derive a title
	for _, line := range lines {
		if strings.HasPrefix(line, "user:") || strings.HasPrefix(line, "User:") {
			content := strings.TrimSpace(line[5:])
			if content == "" {
				continue
			}
			words := strings.Fields(content)
			maxWords := 6
			if len(words) > maxWords {
				words = words[:maxWords]
			}
			title := strings.Join(words, " ")
			if len(title) > 50 {
				title = title[:50]
			}
			return title
		}
	}

	return "Coding session"
}

// buildTitleDigest builds a compact digest of the conversation for title generation.
func buildTitleDigest(messages []providers.Message, maxChars int) string {
	if len(messages) == 0 {
		return ""
	}

	var parts []string
	charCount := 0

	for _, msg := range messages {
		content := strings.TrimSpace(msg.Content)
		if content == "" {
			continue
		}
		if len(content) > 320 {
			content = content[:320] + "..."
		}
		line := fmt.Sprintf("%s: %s", msg.Role, content)
		if charCount+len(line) > maxChars {
			parts = append(parts, "...")
			break
		}
		parts = append(parts, line)
		charCount += len(line)
	}

	return strings.Join(parts, "\n")
}

// cleanGeneratedTitle normalizes the model output to a clean title.
func cleanGeneratedTitle(title string) string {
	// Remove markdown formatting
	title = strings.TrimSpace(title)
	title = strings.TrimPrefix(title, "**")
	title = strings.TrimSuffix(title, "**")
	title = strings.TrimPrefix(title, "\"")
	title = strings.TrimSuffix(title, "\"")
	title = strings.TrimPrefix(title, "'")
	title = strings.TrimSuffix(title, "'")

	// Remove trailing punctuation
	title = strings.TrimRight(title, ".,;:!?")

	// Remove "Title:" prefix
	if strings.HasPrefix(strings.ToLower(title), "title:") {
		title = strings.TrimSpace(title[6:])
	}

	// Limit to 8 words
	words := strings.Fields(title)
	if len(words) > 8 {
		words = words[:8]
	}

	result := strings.Join(words, " ")
	if len(result) > 60 {
		result = result[:60]
	}

	return result
}

// callProviderForTitle sends a lightweight completion request to generate a title.
func callProviderForTitle(ctx context.Context, provider providers.Provider, digest string) string {
	if provider == nil {
		return ""
	}

	prompt := fmt.Sprintf(`Generate a concise, 3-to-6-word Title Case title describing this coding session. No quotes, no punctuation, no commentary.

Session content:
%s`, digest)

	messages := []providers.Message{
		{Role: "system", Content: "You are a helpful assistant that generates concise session titles."},
		{Role: "user", Content: prompt},
	}

	// Use StreamChat and collect the full response
	req := providers.ChatRequest{
		Model:    "", // use default model
		Messages: messages,
	}
	stream, err := provider.StreamChat(ctx, req)
	if err != nil {
		return ""
	}

	var result strings.Builder
	for event := range stream {
		if event.Err != nil {
			return ""
		}
		if event.Done {
			break
		}
		result.WriteString(event.Delta)
	}

	return strings.TrimSpace(result.String())
}

// sessionTitleDisplay renders the current session title.
func sessionTitleDisplay(stg *sessionTitleGenerator, styles appStyles) string {
	if stg.title != "" {
		return styles.accent.Render(stg.title)
	}
	if stg.pending {
		return styles.muted.Render("Generating title...")
	}
	return ""
}
