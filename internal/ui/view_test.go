package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"cli_mate/internal/config"
	"cli_mate/internal/providers"
)

func TestGridSidebarDoesNotDuplicateHeader(t *testing.T) {
	styles := buildStyles(themeFor("midnight"))
	app := App{
		styles: styles,
		width:  120,
		height: 30,
	}
	grid := computeGridLayout(app.width, app.height, true, false, 0, 0, false)

	got := app.renderChatColumnForGrid(grid)
	if strings.Contains(got, "cli_mate") {
		t.Fatalf("expected the body column not to repeat the global header, got %q", got)
	}
}

func TestHiddenSidebarDoesNotReserveChatWidth(t *testing.T) {
	app := App{
		width:   120,
		height:  30,
		sidebar: NewSidebar(NewPlanPanel()),
	}
	app.sidebar.SetSessionInfo(SessionInfo{Provider: "openai"})

	grid := app.computeCurrentGrid()
	if grid.ShowSidebar || grid.SidebarWidth != 0 {
		t.Fatalf("hidden sidebar reserved layout space: %+v", grid)
	}
}

func TestMainViewFillsAvailableTerminalHeight(t *testing.T) {
	styles := buildStyles(themeFor("midnight"))
	app := App{
		styles:   styles,
		width:    80,
		height:   24,
		viewport: newViewport(),
		messages: []providers.Message{{Role: "user", Content: "hello"}},
		log:      []logEntry{{Kind: "system", Text: "hello"}},
	}

	got := app.View().Content
	if height := lipgloss.Height(got); height != app.height-2 {
		t.Fatalf("expected panel height %d, got %d", app.height-2, height)
	}
}

func TestVisibleSidebarStaysWithinTerminal(t *testing.T) {
	styles := buildStyles(themeFor("midnight"))
	sidebar := NewSidebar(NewPlanPanel())
	sidebar.SetVisible(true)
	sidebar.SetSessionInfo(SessionInfo{Provider: "openai", Model: "gpt-4.1", Branch: "main"})
	app := App{
		styles:   styles,
		width:    100,
		height:   24,
		viewport: newViewport(),
		sidebar:  sidebar,
		messages: []providers.Message{{Role: "user", Content: "hello"}},
		log:      []logEntry{{Kind: "system", Text: "hello"}},
	}

	got := app.View().Content
	if width := lipgloss.Width(got); width > app.width {
		t.Fatalf("sidebar view overflowed width: %d > %d", width, app.width)
	}
	if height := lipgloss.Height(got); height > app.height {
		t.Fatalf("sidebar view overflowed height: %d > %d\n%s", height, app.height, got)
	}
}

func TestTinyTerminalDoesNotForceWideComposer(t *testing.T) {
	styles := buildStyles(themeFor("midnight"))
	app := App{styles: styles, width: 24, height: 12}

	got := app.View().Content
	if width := lipgloss.Width(got); width > app.width {
		t.Fatalf("view overflowed tiny terminal: width=%d, terminal=%d\n%s", width, app.width, got)
	}
	if height := lipgloss.Height(got); height > app.height {
		t.Fatalf("view overflowed tiny terminal: height=%d, terminal=%d\n%s", height, app.height, got)
	}
}

func TestWelcomeShowsConnectionAndWorkspace(t *testing.T) {
	styles := buildStyles(themeFor("midnight"))
	app := App{
		styles:        styles,
		width:         100,
		height:        30,
		workspaceRoot: `D:\projects\cli_mate`,
		cfg: &config.Config{
			ActiveProfile: "default",
			Profiles: map[string]config.Profile{
				"default": {Provider: "openrouter", Model: "glm-5.2"},
			},
		},
	}

	got := app.View().Content
	for _, want := range []string{"██████", "Your AI Coding Agent", "glm-5.2", "openrouter", `D:\projects\cli_mate`, "/help", "/model", "/exit"} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected welcome screen to contain %q, got:\n%s", want, got)
		}
	}
	if width := lipgloss.Width(got); width > app.width {
		t.Fatalf("welcome overflowed width: %d > %d", width, app.width)
	}
	if height := lipgloss.Height(got); height > app.height {
		t.Fatalf("welcome overflowed height: %d > %d", height, app.height)
	}
}

func TestActiveToolStaysInsideTranscriptViewport(t *testing.T) {
	styles := buildStyles(themeFor("midnight"))
	app := App{
		styles:          styles,
		width:           90,
		height:          22,
		loading:         true,
		currentStepText: "Preparing tool call",
		streamingTool: &streamingToolCall{
			name:    "write_file",
			path:    "internal/ui/view.go",
			content: strings.Repeat("fmt.Println(\"a very long line that still has to stay inside the viewport\")\n", 80),
		},
		viewport: newViewport(),
	}
	got := app.View().Content

	toolIndex := strings.Index(got, "Editing")
	if toolIndex < 0 || !strings.Contains(got, "internal/ui/view.go") {
		t.Fatalf("expected live tool status inside transcript, got %q", got)
	}
	promptIndex := strings.Index(got, "esc to interrupt")
	if promptIndex < 0 {
		t.Fatalf("expected prompt to remain visible, got %q", got)
	}
	if toolIndex > promptIndex {
		t.Fatalf("expected live tool output above prompt, got %q", got)
	}
	if height := lipgloss.Height(got); height > app.height {
		t.Fatalf("expected rendered panel height within terminal, got %d:\n%s", height, got)
	}
}

func TestStreamingToolCallViewHonorsBounds(t *testing.T) {
	styles := buildStyles(themeFor("midnight"))
	tc := &streamingToolCall{
		name:    "write_file",
		path:    "internal/ui/a/very/long/path/that/should/not/stretch/the/window/view.go",
		content: strings.Repeat("this is a long streaming content line that must be clipped cleanly\n", 30),
	}

	got := streamingToolCallView(tc, styles, 36, 4)

	if height := lipgloss.Height(got); height > 4 {
		t.Fatalf("expected height <= 4, got %d:\n%s", height, got)
	}
	if width := lipgloss.Width(got); width > 36 {
		t.Fatalf("expected width <= 36, got %d:\n%s", width, got)
	}
}

func TestActiveRunConversationPresentation(t *testing.T) {
	styles := buildStyles(themeFor("midnight"))
	app := App{styles: styles, loadingFrame: 2}

	user := app.renderConversationEntry(logEntry{Kind: "user", Text: "add JWT middleware"}, 60, 0)
	if !strings.Contains(user, "> ") || !strings.Contains(user, "add JWT middleware") {
		t.Fatalf("submitted prompt is not presented as a conversation row: %q", user)
	}

	assistant := app.renderConversationEntry(logEntry{Kind: liveAssistantLogKind, Text: "I'll add middleware and validate the token."}, 60, 0)
	if !strings.Contains(assistant, "Assistant") || !strings.Contains(assistant, "validate the token") || !strings.Contains(assistant, "█") {
		t.Fatalf("streaming assistant card is incomplete: %q", assistant)
	}
	if lipgloss.Width(assistant) > 60 {
		t.Fatalf("assistant card overflowed: %d", lipgloss.Width(assistant))
	}
}

func TestCompletedEditRendersStatusAndInlineDiff(t *testing.T) {
	styles := buildStyles(themeFor("midnight"))
	app := App{styles: styles}
	entry := logEntry{Kind: "tool", Text: "file_edit middleware/auth.go: edited middleware/auth.go\n```diff\n@@ auth.go:12 @@\n-return next\n+return middleware(next)\n```"}

	got := app.renderConversationEntry(entry, 64, 0)
	for _, want := range []string{"Edited", "middleware/auth.go", "@@ auth.go:12 @@", "-return next", "+return middleware(next)"} {
		if !strings.Contains(got, want) {
			t.Fatalf("edit presentation missing %q:\n%s", want, got)
		}
	}
	if lipgloss.Width(got) > 64 {
		t.Fatalf("inline diff overflowed: %d", lipgloss.Width(got))
	}
}

func TestLoadingViewShowsInterruptFooter(t *testing.T) {
	styles := buildStyles(themeFor("midnight"))
	app := App{
		styles:   styles,
		width:    80,
		height:   24,
		loading:  true,
		pending:  true,
		viewport: newViewport(),
		log:      []logEntry{{Kind: "user", Text: "add JWT middleware"}},
	}

	got := app.View().Content
	for _, want := range []string{"add JWT middleware", "Thinking...", "esc to interrupt"} {
		if !strings.Contains(got, want) {
			t.Fatalf("active run view missing %q:\n%s", want, got)
		}
	}
	if lipgloss.Width(got) > app.width || lipgloss.Height(got) > app.height {
		t.Fatalf("active run view overflowed %dx%d terminal: %dx%d", app.width, app.height, lipgloss.Width(got), lipgloss.Height(got))
	}
	lines := strings.Split(got, "\n")
	interruptLine := -1
	for i, line := range lines {
		if strings.Contains(line, "esc to interrupt") {
			interruptLine = i
			break
		}
	}
	// Outer panel padding and border occupy the remaining rows below the footer.
	if interruptLine < len(lines)-6 {
		t.Fatalf("interrupt footer was not locked to the bottom: line %d of %d\n%s", interruptLine, len(lines), got)
	}
}

func TestChatCompletionRestoresInput(t *testing.T) {
	styles := buildStyles(themeFor("midnight"))
	app := App{
		styles:         styles,
		width:          80,
		height:         24,
		loading:        true,
		pending:        true,
		spinnerTicking: true,
		activeRunID:    1,
		viewport:       newViewport(),
		messages:       []providers.Message{{Role: "user", Content: "hello"}},
		log:            []logEntry{{Kind: "user", Text: "hello"}},
		turnStartedAt:  time.Now(),
	}

	updated, _ := app.Update(chatDoneMsg{messages: []providers.Message{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "Hi! How can I help you today?"},
	}})
	got := updated.(App).View().Content

	if !strings.Contains(got, "Task complete") || !strings.Contains(got, ">>>") {
		t.Fatalf("expected completion details and restored input, got:\n%s", got)
	}
	if strings.Contains(got, "esc to interrupt") {
		t.Fatalf("completed run still rendered the interrupt footer:\n%s", got)
	}
}

func TestTranscriptScrollsWithinMultilineEntry(t *testing.T) {
	styles := buildStyles(themeFor("midnight"))
	var content strings.Builder
	for i := 1; i <= 20; i++ {
		fmt.Fprintf(&content, "row-%02d\n", i)
	}
	app := App{
		styles:   styles,
		width:    80,
		height:   16,
		viewport: newViewport(),
		log:      []logEntry{{Kind: "system", Text: content.String()}},
	}

	bottom := app.consoleFor(70, 8, 8)
	if !strings.Contains(bottom, "row-20") {
		t.Fatalf("expected bottom of multiline entry, got:\n%s", bottom)
	}

	app.viewport.scrollBy(mouseWheelScrollStep)
	scrolled := app.consoleFor(70, 8, 8)
	if !strings.Contains(scrolled, "row-17") || !strings.Contains(scrolled, "newer lines") {
		t.Fatalf("mouse-style scrolling did not move within the entry:\n%s", scrolled)
	}
	if strings.Contains(scrolled, "row-20") {
		t.Fatalf("scrolling up remained pinned to the entry tail:\n%s", scrolled)
	}
}

func TestCtrlCImmediatelyQuitsStuckRun(t *testing.T) {
	cancelled := false
	app := App{
		pending:     true,
		activeRunID: 1,
		runCancel:   func() { cancelled = true },
		flushRunIDs: map[int]string{1: "session-that-may-never-flush"},
	}

	cmd := app.handleCtrlC()
	if cmd == nil {
		t.Fatal("Ctrl+C did not return a quit command")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("Ctrl+C returned %T instead of tea.QuitMsg", cmd())
	}
	if !cancelled || app.pending || !app.exiting {
		t.Fatalf("Ctrl+C did not synchronously cancel and exit: %+v", app)
	}
}

func TestCompletionDetailsUsesCurrentRunOnly(t *testing.T) {
	app := App{
		runLogStart:   1,
		turnStartedAt: time.Now().Add(-2 * time.Second),
		log: []logEntry{
			{Kind: "tool", Text: `shell {"command":"old command"}`},
			{Kind: "tool", Text: `file_edit {"path":"internal/ui/view.go"}`},
			{Kind: "tool", Text: `shell {"command":"go test ./..."}`},
		},
	}
	messages := []providers.Message{
		{Role: "user", Content: "old"},
		{Role: "user", Content: "add completion details"},
		{Role: "assistant", Content: "Implemented a readable completion details view for finished tasks."},
	}

	got := app.completionDetails(messages, 1, nil)

	for _, want := range []string{"Summary", "Files", "internal/ui/view.go", "Verification", "go test ./...", "Actions"} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in completion details, got %q", want, got)
		}
	}
	if strings.Contains(got, "old command") {
		t.Fatalf("expected old run command to be ignored, got %q", got)
	}
}

func TestRenderCompletionEntryShowsReadableCard(t *testing.T) {
	styles := buildStyles(themeFor("midnight"))
	app := App{styles: styles}

	got := app.renderCompletionEntry("Summary\n- Done\n\nVerification\n- go test ./...", 60, 0)

	if !strings.Contains(got, "Task complete") {
		t.Fatalf("expected completion card title, got %q", got)
	}
	if !strings.Contains(got, "Verification") {
		t.Fatalf("expected completion details, got %q", got)
	}
	if width := lipgloss.Width(got); width > 60 {
		t.Fatalf("expected width <= 60, got %d:\n%s", width, got)
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

func TestFileMentionSuggestionsShowFullForwardSlashPath(t *testing.T) {
	styles := buildStyles(themeFor("midnight"))
	app := App{
		styles: styles,
		width:  100,
		input:  "@suggest",
		files:  []string{`internal\ui\suggestions.go`},
	}

	got := app.renderSuggestionsFor(80)
	if !strings.Contains(got, "@internal/ui/suggestions.go") {
		t.Fatalf("expected complete forward-slash mention path, got %q", got)
	}
	if !strings.Contains(got, "Mention a file") {
		t.Fatalf("expected file picker title, got %q", got)
	}
}

func TestSuggestionDialogIsCenteredInBody(t *testing.T) {
	styles := buildStyles(themeFor("midnight"))
	app := App{styles: styles, input: "@", files: []string{"README.md"}}

	got := app.renderSuggestionDialog(80, 24)
	lines := strings.Split(got, "\n")
	firstContent := 0
	for firstContent < len(lines) && strings.TrimSpace(lines[firstContent]) == "" {
		firstContent++
	}
	if firstContent < 3 {
		t.Fatalf("expected dialog to be vertically centered, first content line was %d", firstContent)
	}
	if width := lipgloss.Width(got); width != 80 {
		t.Fatalf("expected dialog canvas width 80, got %d", width)
	}
}

func TestCommandSuggestionsExposeOnlyUserFacingCommands(t *testing.T) {
	for _, want := range []string{"/help", "/setup", "/model", "/permissions", "/compact", "/exit"} {
		got := commandSuggestions(strings.TrimPrefix(want, "/"), nil)
		if len(got) != 1 || got[0].Label != want {
			t.Fatalf("expected user-facing command %s", want)
		}
	}
	for _, removed := range []string{"/api-key", "/base-url", "/max-tokens", "/connect", "/review", "/diff", "/commit", "/skills"} {
		if got := commandSuggestions(strings.TrimPrefix(removed, "/"), nil); len(got) != 0 {
			t.Fatalf("deprecated command %s is still exposed", removed)
		}
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
