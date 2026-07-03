package agent

import "strings"

// IsConversationalPrompt detects simple chat turns that should not trigger
// repository exploration or tool use.
func IsConversationalPrompt(prompt string) bool {
	text := strings.TrimSpace(strings.ToLower(prompt))
	if text == "" || len(text) > 120 {
		return false
	}

	codeSignals := []string{
		"@", "/", "\\", ".go", ".js", ".ts", ".py", ".rs", ".java",
		"fix", "bug", "error", "test", "build", "run", "implement",
		"create", "edit", "change", "refactor", "review", "inspect",
		"file", "repo", "code", "function", "class", "package",
	}
	for _, signal := range codeSignals {
		if strings.Contains(text, signal) {
			return false
		}
	}

	conversational := []string{
		"hi", "hi there", "hello", "hey", "yo", "sup",
		"thanks", "thank you", "ok", "okay", "cool",
		"how are you", "what's up", "whats up",
	}
	for _, phrase := range conversational {
		if text == phrase || strings.HasPrefix(text, phrase+"!") || strings.HasPrefix(text, phrase+"?") {
			return true
		}
	}

	return false
}
