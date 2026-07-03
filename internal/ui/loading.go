package ui

import (
	"fmt"
	"strings"

	"cli_mate/internal/config"
)

func loadingSteps(prompt string, profile config.Profile, workspaceName string) []string {
	steps := []string{
		fmt.Sprintf("Accessing repo %s", fallback(workspaceName, "workspace")),
	}

	mentions := mentionedFiles(prompt)
	for _, mention := range mentions {
		steps = append(steps, "Reading "+mention)
	}

	lower := strings.ToLower(prompt)
	switch {
	case containsAny(lower, "edit", "change", "update", "write", "create", "fix", "patch"):
		steps = append(steps, "Planning file changes")
	case containsAny(lower, "bug", "error", "fail", "debug", "issue"):
		steps = append(steps, "Tracing repo behavior")
	default:
		steps = append(steps, "Preparing repo context")
	}

	model := profile.Model
	if model == "" {
		model = "model"
	}
	provider := profile.Provider
	if provider == "" {
		provider = "provider"
	}
	steps = append(steps,
		fmt.Sprintf("Calling %s %s", provider, model),
		"Streaming response",
	)
	return steps
}

func mentionedFiles(prompt string) []string {
	seen := map[string]bool{}
	var mentions []string
	for _, field := range strings.Fields(prompt) {
		if !strings.HasPrefix(field, "@") {
			continue
		}
		mention := strings.Trim(strings.TrimPrefix(field, "@"), ".,:;!?()[]{}\"'")
		if mention == "" || seen[mention] {
			continue
		}
		seen[mention] = true
		mentions = append(mentions, mention)
	}
	return mentions
}

func containsAny(text string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(text, needle) {
			return true
		}
	}
	return false
}
