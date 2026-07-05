package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"cli_mate/internal/providers"
)

const (
	recapTimeout        = 30 * time.Second
	recapMaxAnswerChars = 1600
)

const recapSystemPrompt = `You are a summarizer. Return exactly one sentence in plain English (no markdown, no formatting) 
summarizing what the assistant just did in the chat. Focus on actions taken and results achieved. 
If the answer is conversational, just say "The assistant responded to the user."`

// recapResult carries the generated recap.
type recapResult struct {
	recap string
	err   error
}

// generateRecap generates a one-sentence recap of the assistant's answer.
func generateRecap(ctx context.Context, provider providers.Provider, model string, answer string) string {
	if len(answer) > recapMaxAnswerChars {
		answer = answer[:recapMaxAnswerChars]
	}

	ctx, cancel := context.WithTimeout(ctx, recapTimeout)
	defer cancel()

	req := providers.ChatRequest{
		Model: model,
		Messages: []providers.Message{
			{Role: "user", Content: "Summarize what the assistant just did in one sentence:\n\n" + answer},
		},
		Temperature: 0.3,
		MaxTokens:   60,
	}

	stream, err := provider.StreamChat(ctx, req)
	if err != nil {
		return ""
	}

	var recap strings.Builder
	for event := range stream {
		if event.Err != nil {
			break
		}
		recap.WriteString(event.Delta)
		if event.Done {
			break
		}
	}

	result := cleanGeneratedRecap(recap.String())
	return result
}

// cleanGeneratedRecap cleans up the generated recap text.
func cleanGeneratedRecap(recap string) string {
	recap = strings.TrimSpace(recap)
	recap = strings.TrimPrefix(recap, "**")
	recap = strings.TrimSuffix(recap, "**")
	recap = strings.TrimPrefix(recap, "*")
	recap = strings.TrimSuffix(recap, "*")
	recap = strings.Trim(recap, "\"'")
	recap = strings.TrimSpace(recap)
	return recap
}

// generateRecapCmd returns a Bubble Tea command to generate a recap asynchronously.
func generateRecapCmd(provider providers.Provider, model string, answer string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		recap := generateRecap(ctx, provider, model, answer)
		return recapResult{recap: recap}
	}
}

// formatRecap formats the recap for display in the transcript footer.
func formatRecap(recap string, styles appStyles) string {
	if recap == "" {
		return ""
	}
	return styles.muted.Render(fmt.Sprintf("  ↳ %s", recap))
}
