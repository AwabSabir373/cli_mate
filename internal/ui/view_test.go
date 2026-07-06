package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cli_mate/internal/config"
	"cli_mate/internal/providers"
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

func TestRenderChatColumnKeepsHeaderWithSidebar(t *testing.T) {
	styles := buildStyles(themeFor("midnight"))
	app := App{
		styles: styles,
		width:  120,
		height: 30,
	}

	got := app.renderChatColumn("HEADER", computeLayout(app.width, true, false))
	if !strings.Contains(got, "HEADER") {
		t.Fatalf("expected header in sidebar chat column, got %q", got)
	}
	if !strings.Contains(got, ">>>") {
		t.Fatalf("expected prompt in sidebar chat column, got %q", got)
	}
}

func TestRenderSuggestionsShowsDescriptions(t *testing.T) {
	styles := buildStyles(themeFor("midnight"))
	app := App{
		styles: styles,
		width:  100,
		input:  "/pro",
	}

	got := app.renderSuggestionsFor(80)
	if !strings.Contains(got, "/provider") {
		t.Fatalf("expected provider command suggestion, got %q", got)
	}
	if !strings.Contains(got, "choose one active provider") {
		t.Fatalf("expected suggestion description, got %q", got)
	}
}

func TestOnboardingReviewMasksShortAPIKey(t *testing.T) {
	styles := buildStyles(themeFor("midnight"))
	state := newOnboardingState()
	state.active = true
	state.stage = setupStageReview
	state.provider = "openai"
	state.apiKey = "short"
	state.model = "gpt-test"

	got := state.renderReview(styles, 80)
	if !strings.Contains(got, "*****") {
		t.Fatalf("expected short api key to be masked, got %q", got)
	}
}

func TestOnboardingFallbackModelCanAdvance(t *testing.T) {
	state := newOnboardingState()
	state.active = true
	state.stage = setupStageModel
	state.provider = "custom"

	state.handleModelKey("enter")

	if state.stage != setupStageReview {
		t.Fatalf("expected fallback model selection to advance to review, got stage %v", state.stage)
	}
	if state.model == "" {
		t.Fatal("expected fallback model to be selected")
	}
}

func TestMCPManagerAddServerAcceptsTypedInput(t *testing.T) {
	manager := newMCPManager()
	manager.show()

	manager.handleKey("enter")
	for _, key := range []string{"s", "r", "v", "enter", "n", "p", "x", "enter", "-", "y", "space", "p", "k", "g", "enter", "enter"} {
		manager.handleKey(key)
	}
	shouldSave, action := manager.handleKey("enter")

	if !shouldSave || action != "save" {
		t.Fatalf("expected save action, got save=%v action=%q", shouldSave, action)
	}
	if len(manager.servers) != 1 {
		t.Fatalf("expected one server, got %d", len(manager.servers))
	}
	server := manager.servers[0]
	if server.Name != "srv" || server.Command != "npx" || strings.Join(server.Args, " ") != "-y pkg" {
		t.Fatalf("unexpected server config: %+v", server)
	}
}

func TestMCPManagerDeleteRequestsSave(t *testing.T) {
	manager := newMCPManager()
	manager.servers = append(manager.servers, testMCPConfig("srv"))
	manager.show()

	shouldSave, action := manager.handleKey("delete")

	if !shouldSave || action != "delete" {
		t.Fatalf("expected delete save action, got save=%v action=%q", shouldSave, action)
	}
	if len(manager.servers) != 0 {
		t.Fatalf("expected server to be deleted, got %d", len(manager.servers))
	}
}

func TestAskUserCanSelectSecondOption(t *testing.T) {
	state := newAskUserState([]askUserQuestion{{
		Header:   "Mode",
		Question: "Pick one",
		Options:  []askUserChoice{{Label: "A"}, {Label: "B"}},
	}})

	state.handleKey("right")
	state.handleKey("enter")
	got, finished := state.handleKey("enter")

	if !finished || got != "B" {
		t.Fatalf("expected selected answer B, got finished=%v answer=%q", finished, got)
	}
}

func TestAskUserNoOptionsCanTypeAnswer(t *testing.T) {
	state := newAskUserState([]askUserQuestion{{
		Header:   "Name",
		Question: "What name?",
	}})

	state.handleKey("enter")
	for _, key := range []string{"c", "l", "i", "space", "m", "a", "t", "e", "enter"} {
		state.handleKey(key)
	}
	got, finished := state.handleKey("enter")

	if !finished || got != "cli mate" {
		t.Fatalf("expected typed answer, got finished=%v answer=%q", finished, got)
	}
}

func TestSessionControlsCompactReturnsAction(t *testing.T) {
	controls := newSessionControls()
	controls.show()
	controls.action = actionCompact

	action, finished := controls.handleKey("enter")

	if !finished || action != "compact" {
		t.Fatalf("expected compact action, got finished=%v action=%q", finished, action)
	}
}

func TestExportSessionWritesMarkdown(t *testing.T) {
	dir := t.TempDir()
	app := App{
		workspaceRoot: dir,
		workspaceName: "repo",
		messages: []providers.Message{
			{Role: "user", Content: "hello"},
			{Role: "assistant", Content: "hi"},
		},
	}

	app.exportSession(filepath.Join("exports", "session.md"))

	data, err := os.ReadFile(filepath.Join(dir, "exports", "session.md"))
	if err != nil {
		t.Fatalf("expected export file: %v", err)
	}
	got := string(data)
	if !strings.Contains(got, "# cli_mate session") || !strings.Contains(got, "hello") || !strings.Contains(got, "hi") {
		t.Fatalf("unexpected export content: %q", got)
	}
}

func testMCPConfig(name string) config.MCPConfig {
	return config.MCPConfig{Name: name, Command: "npx"}
}
