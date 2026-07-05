package ui

import (
	"strings"
	"testing"
)

func TestRenderChatContentShowsActivityAbovePrompt(t *testing.T) {
	styles := buildStyles(themeFor("midnight"))
	app := App{
		styles:          styles,
		width:           100,
		height:          30,
		loading:         true,
		currentStepText: "Calling openrouter deepseek",
		streamFade:      newStreamingFade(styles.accent.GetForeground(), styles.muted.GetForeground()),
	}

	got := app.renderChatContent(computeLayout(app.width, false, false))

	statusIndex := strings.Index(got, "Calling openrouter deepseek")
	if statusIndex < 0 {
		t.Fatalf("expected loading status in chat content, got %q", got)
	}
	promptIndex := strings.Index(got, ">>>")
	if promptIndex < 0 {
		t.Fatalf("expected input prompt in chat content, got %q", got)
	}
	if statusIndex > promptIndex {
		t.Fatalf("expected loading status above prompt, got %q", got)
	}
}
