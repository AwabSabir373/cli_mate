package ui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"

	"cli_mate/internal/agent"
	"cli_mate/internal/config"
	"cli_mate/internal/providers"
	"cli_mate/internal/storage"
	"cli_mate/internal/tools"
	"cli_mate/internal/usercommands"
)

type logEntry struct {
	Kind         string
	Text         string
	Time         time.Time
	renderedText string // cached rendered markdown output
	renderWidth  int    // width used for the cached render
}

type suggestion struct {
	Value       string
	Label       string
	Description string
}

type loadingTickMsg struct{}
type syncTickMsg struct{}
type filesSyncedMsg []string
type chatStreamMsg struct {
	token string
	c     chan tea.Msg
}
type chatLoadingStepMsg struct {
	text string
	c    chan tea.Msg
}
type chatApprovalRequestMsg struct {
	call     tools.Call
	response chan bool
}
type chatEditSnapshotMsg struct {
	record editRecord
	c      chan tea.Msg
}
type chatToolCallMsg struct {
	toolName string
	args     string
	c        chan tea.Msg
}
type chatToolResultMsg struct {
	toolName string
	result   string
	c        chan tea.Msg
}

func syncTick() tea.Cmd {
	return tea.Tick(30*time.Second, func(time.Time) tea.Msg {
		return syncTickMsg{}
	})
}

func syncFilesCmd(root string, limit int) tea.Cmd {
	return func() tea.Msg {
		return filesSyncedMsg(workspaceFiles(root, limit))
	}
}

// App is the Bubble Tea model. Keep fields copy-safe because Bubble Tea passes
// the model by value through Update.
type App struct {
	cfg               *config.Config
	store             storage.SessionStore
	sessionID         string
	program           *tea.Program
	input             string
	cursorPos         int
	inputMode         string
	renderer          *Renderer
	styles            appStyles
	err               error
	width             int
	height            int
	log               []logEntry
	files             []string
	selected          int
	workspaceName     string
	theme             string
	provider          providers.Provider
	connected         config.Profile
	messages          []providers.Message
	loading           bool
	loadingFrame      int
	loadingSteps      []string
	currentStepText   string
	completedStepText string
	streamBuffer      string
	workspaceRoot     string
	instructions      string
	scrollOffset      int
	tokensUsed        int
	pasteBuffer       string
	isPasting         bool
	history           []string
	historyIndex      int
	editHistory       []editRecord
	pendingApproval   *approvalRequest
	compactPending    bool
	userCommands      []usercommands.Command
	responseStyle     agent.ResponseStyle

	// New enhanced UI fields
	toolCardRegistry *toolBodyRegistry
	streamFade       *streamingFadeState
	streamingTool    *streamingToolCall
	sidebar          *Sidebar
	planPanel        *PlanPanel
	permissionPrompt *permissionPrompt
	askUserState     *askUserState
	renderCache      *renderCache
	// Unwired component fields (created but need integration)
	viewport          *viewport
	flushBatcher      *flushBatcher
	hoverManager      *hoverManager
	bgTerminalManager *bgTerminalManager
	planStepDetail    *planStepDetail
	// New major feature components
	onboarding    *onboardingState
	composer      *composerState
	sessionPicker *sessionPicker
	mcpManager    *mcpManager
	// Additional feature components
	transcriptSelection *transcriptSelection
	prStatus            *PRDisplay
	specMode            *specMode
	subchatManager      *subchatManager
	// Remaining feature components (phase 2)
	autocomplete     *autocompleteState
	picker           *genericPicker
	sessionCtrls     *sessionControls
	sessionTitle     *sessionTitleGenerator
	commandOutput    *commandOutputView
	startup          *startupState
	imageAttach      *imageAttachState
	doctor           *doctorView
}

// SetProgram wires the Bubble Tea program back into the app so background
// goroutines (e.g. tool approval) can deliver messages via program.Send
// without competing with the streaming channel.
func (a *App) SetProgram(p *tea.Program) {
	a.program = p
}

// RequestApproval blocks until the user answers the approval prompt, or the
// context is cancelled. It uses program.Send so the request cannot deadlock
// behind pending stream tokens on the chat channel.
func (a *App) RequestApproval(ctx context.Context, call tools.Call) bool {
	response := make(chan bool, 1)
	if a.program == nil {
		return true
	}
	a.program.Send(chatApprovalRequestMsg{call: call, response: response})
	select {
	case allowed := <-response:
		return allowed
	case <-ctx.Done():
		return false
	}
}

type editRecord struct {
	path    string
	content string
	existed bool
}

type approvalRequest struct {
	call     tools.Call
	response chan bool
}

func NewApp(cfg *config.Config, store storage.SessionStore) App {
	renderer, err := NewRenderer(100)
	workspace, _ := os.Getwd()
	instructions, instructionErr := agent.LoadInstructions(context.Background(), workspace)
	if err == nil && instructionErr != nil {
		err = instructionErr
	}

	themeName := "midnight"
	if cfg.ActiveProfile != "" {
		if p, e := cfg.Active(); e == nil && p.Provider != "" {
			// Theme could come from profile in future
		}
	}

	sessionID := uuid.New().String()
	if store != nil {
		if createErr := store.CreateSession(context.Background(), storage.SessionRecord{
			ID:    sessionID,
			Title: "cli_mate session",
		}); createErr != nil {
			if err == nil {
				err = createErr
			}
		}
	}

	styles := buildStyles(themeFor(themeName))
	planPanel := NewPlanPanel()
	sidebar := NewSidebar(planPanel)

	return App{
		cfg:           cfg,
		store:         store,
		sessionID:     sessionID,
		renderer:      renderer,
		styles:        styles,
		err:           err,
		width:         100,
		height:        30,
		files:         workspaceFiles(workspace, 300),
		workspaceName: filepath.Base(workspace),
		workspaceRoot: workspace,
		instructions:  instructions,
		theme:         themeName,
		userCommands:  loadUserCommands(workspace),
		log: []logEntry{
			{Kind: "system", Text: "Welcome to cli_mate. Press / to open commands, choose /provider, then follow the setup.", Time: time.Now()},
		},

		// New enhanced UI
		toolCardRegistry: newDefaultToolBodyRegistry(),
		streamFade:       newStreamingFade(styles.accent.GetForeground(), styles.muted.GetForeground()), // lipgloss.TerminalColor is compatible
		planPanel:        planPanel,
		sidebar:          sidebar,
		renderCache:      newRenderCache(30*time.Second, 200),
		// New wired components
		viewport:          newViewport(),
		flushBatcher:      newFlushBatcher(),
		hoverManager:      newHoverManager(),
		bgTerminalManager: newBGTerminalManager(),
		planStepDetail:    newPlanStepDetail(),
		// New major feature components
		onboarding:    newOnboardingState(),
		composer:      newComposerState(),
		sessionPicker: newSessionPicker(),
		mcpManager:    newMCPManager(),
		// Additional feature components
		transcriptSelection: newTranscriptSelection(),
		prStatus:            newPRDisplay(),
		specMode:            newSpecMode(),
		subchatManager:      newSubchatManager(),
		// Remaining feature components (phase 2)
		autocomplete:  newAutocompleteState(),
		picker:        newGenericPicker(),
		sessionCtrls:  newSessionControls(),
		sessionTitle:  newSessionTitleGenerator(),
		commandOutput: newCommandOutputView(),
		startup:       newStartupState(),
		imageAttach:   newImageAttachState(),
		doctor:        newDoctorView(),
	}
}

// loadUserCommands discovers user-defined slash commands from the workspace.
func loadUserCommands(workspace string) []usercommands.Command {
	configDir, _ := os.UserConfigDir()
	paths := usercommands.DefaultPaths(workspace, configDir)
	return usercommands.Load(paths)
}

func (a App) Init() tea.Cmd {
	return tea.Batch(syncTick(), syncFilesCmd(a.workspaceRoot, 5000))
}

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case chatStepMsg:
		a.log = append(a.log, msg.entry)
		a.completedStepText = msg.entry.Text
		// Update streaming tool state
		if msg.entry.Kind == "tool" {
			a.streamingTool = &streamingToolCall{
				name: parseToolName(msg.entry.Text),
				args: msg.entry.Text,
			}
			path := parseToolPath(msg.entry.Text)
			if path != "" {
				a.streamingTool.path = path
			}
		}
		return a, waitForChatMsg(msg.c)
	case chatToolCallMsg:
		a.streamingTool = &streamingToolCall{
			name: msg.toolName,
			args: msg.args,
		}
		path := streamingFilePath(msg.args)
		if path != "" {
			a.streamingTool.path = path
		}
		return a, waitForChatMsg(msg.c)
	case chatToolResultMsg:
		if a.streamingTool != nil && a.streamingTool.name == msg.toolName {
			a.streamingTool.completed = true
			a.streamingTool.content = msg.result
		}
		return a, waitForChatMsg(msg.c)
	case chatEditSnapshotMsg:
		a.editHistory = append(a.editHistory, msg.record)
		if len(a.editHistory) > 20 {
			a.editHistory = a.editHistory[1:]
		}
		return a, waitForChatMsg(msg.c)
	case chatApprovalRequestMsg:
		a.pendingApproval = &approvalRequest{call: msg.call, response: msg.response}
		a.permissionPrompt = newPermissionPrompt(msg.call)
		a.appendLog("system", approvalPromptOld(msg.call))
		return a, nil
	case chatStreamMsg:
		a.streamBuffer += msg.token
		a.streamFade.addToken(msg.token)
		const maxStreamPreviewBytes = 1000
		if len(a.streamBuffer) > maxStreamPreviewBytes {
			a.streamBuffer = a.streamBuffer[len(a.streamBuffer)-maxStreamPreviewBytes:]
		}
		return a, waitForChatMsg(msg.c)
	case chatLoadingStepMsg:
		a.currentStepText = msg.text
		return a, waitForChatMsg(msg.c)
	case chatContextMsg:
		a.tokensUsed = msg.tokens
		return a, waitForChatMsg(msg.c)
	case chatDoneMsg:
		previousMessageCount := len(a.messages)
		a.loading = false
		a.loadingFrame = 0
		a.loadingSteps = nil
		a.currentStepText = ""
		a.completedStepText = ""
		a.streamBuffer = ""
		a.tokensUsed = 0
		a.streamingTool = nil
		a.streamFade.clear()
		a.messages = msg.messages
		a.persistMessages(msg.messages, previousMessageCount)
		// Update sidebar
		if a.sidebar != nil {
			profile, _ := a.cfg.Active()
			a.sidebar.SetSessionInfo(SessionInfo{
				Provider: profile.Provider,
				Model:    profile.Model,
				Messages: len(msg.messages),
			})
			a.sidebar.SetTouchedFiles(touchedFilesFromLog(a.log))
		}
		return a, syncFilesCmd(a.workspaceRoot, 5000)
	case filesSyncedMsg:
		if slices.Equal(a.files, msg) {
			return a, nil
		}
		a.files = msg
		return a, nil
	case syncTickMsg:
		if a.loading {
			return a, syncTick()
		}
		return a, tea.Batch(syncTick(), syncFilesCmd(a.workspaceRoot, 5000))
	case loadingTickMsg:
		if !a.loading {
			return a, nil
		}
		a.loadingFrame++
		return a, loadingTick()
	case tea.KeyMsg:
		// Route events to overlays first (onboarding, session picker, MCP manager)
		if a.mcpManager != nil && a.mcpManager.isVisible() {
			shouldSave, action := a.mcpManager.handleKey(msg.String())
			if shouldSave {
				// Save MCP servers directly to config
				if a.cfg != nil {
					a.cfg.MCP = a.mcpManager.servers
				}
				a.saveSettings()
				a.appendLog("system", "MCP servers saved.")
			}
			_ = action
			return a, nil
		}

		if a.sessionPicker != nil && a.sessionPicker.isVisible() {
			selectedID, finished := a.sessionPicker.handleKey(msg.String())
			if finished && selectedID != "" && a.store != nil {
				ctx := context.Background()
				msgs, err := resumeSession(ctx, a.store, selectedID)
				if err == nil {
					// Convert agent messages to provider messages
					a.messages = nil
					for _, m := range msgs {
						a.messages = append(a.messages, providers.Message{
							Role:    string(m.Role),
							Content: m.Content,
						})
					}
					a.appendLog("system", fmt.Sprintf("Resumed session with %d messages.", len(msgs)))
				} else {
					a.appendLog("error", fmt.Sprintf("Could not resume session: %v", err))
				}
			}
			return a, nil
		}

		if a.onboarding != nil && a.onboarding.isActive() {
			key := msg.String()
			onboardingStage := a.onboarding.stage

			// For API key and base URL stages, forward typed characters to the input field
			if onboardingStage == setupStageAPIKey || onboardingStage == setupStageBaseURL {
				if len(key) == 1 && key != "\n" && key != "\r" {
					// Forward typed character to the input buffer
					a.input += key
					a.cursorPos = len(a.input)
					return a, nil
				}
				if key == "backspace" && len(a.input) > 0 {
					a.input = a.input[:len(a.input)-1]
					a.cursorPos = len(a.input)
					return a, nil
				}
				if key == "enter" {
					// Take the input text and feed it to the onboarding state
					switch onboardingStage {
					case setupStageAPIKey:
						a.onboarding.apiKey = a.input
						_, _ = a.onboarding.handleKey(key)
						if a.onboarding.stage != setupStageAPIKey {
							// Move past this stage
							a.input = ""
							a.cursorPos = 0
						}
					case setupStageBaseURL:
						a.onboarding.baseURL = a.input
						_, _ = a.onboarding.handleKey(key)
						if a.onboarding.stage != setupStageBaseURL {
							a.input = ""
							a.cursorPos = 0
						}
					}
					return a, nil
				}
				if key == "esc" {
					a.onboarding.handleKey(key)
					a.input = ""
					a.cursorPos = 0
					return a, nil
				}
				if key == "ctrl+v" {
					a.pasteFromClipboard()
					return a, nil
				}
				return a, nil
			}

			// For selection-based stages, route to onboarding
			shouldClose, _ := a.onboarding.handleKey(key)
			if shouldClose {
				a.onboarding.active = false
			}
			// If complete, apply config and connect
			if a.onboarding.isComplete() {
				a.onboarding.applyConfig(&a)
				a.appendLog("system", "Setup complete! Connecting to provider...")
				a.connect()
				a.onboarding.reset()
			}
			return a, nil
		}

		// Route events to spec mode overlay
		if a.specMode != nil && a.specMode.isVisible() {
			a.specMode.handleKey(msg.String())
			return a, nil
		}

		// Route events to subchat overlay
		if a.subchatManager != nil && a.subchatManager.isActive() {
			a.subchatManager.handleKey(msg.String())
			return a, nil
		}

		// Route events to PR status overlay
		if a.prStatus != nil && a.prStatus.isVisible() {
			a.prStatus.handleKey(msg.String())
			return a, nil
		}

		// Route events to session controls overlay
		if a.sessionCtrls != nil && a.sessionCtrls.isVisible() {
			a.sessionCtrls.handleKey(msg.String())
			return a, nil
		}

		// Route events to command output overlay
		if a.commandOutput != nil && a.commandOutput.isVisible() {
			a.commandOutput.handleKey(msg.String())
			return a, nil
		}

		// Route events to startup overlay
		if a.startup != nil && a.startup.isVisible() {
			if a.startup.handleKey(msg.String()) {
				return a, nil
			}
			return a, nil
		}

		// Route events to doctor overlay
		if a.doctor != nil && a.doctor.isVisible() {
			a.doctor.handleKey(msg.String())
			return a, nil
		}

		// Route events to image attach overlay
		if a.imageAttach != nil && a.imageAttach.isVisible() {
			a.imageAttach.handleKey(msg.String())
			return a, nil
		}

		// Route events to picker overlay
		if a.picker != nil && a.picker.isVisible() {
			a.picker.handleKey(msg.String())
			return a, nil
		}

		if a.pendingApproval != nil {
			if a.handleApprovalKey(msg) {
				return a, nil
			}
		}
		if a.askUserState != nil && a.askUserState.active {
			result, finished := a.askUserState.handleKey(msg.String())
			if finished {
				a.inputMode = ""
				a.askUserState = nil
				if result != "" {
					a.setInput(result)
					_ = a.submit()
				}
			}
			return a, nil
		}

		if msg.Paste {
			text := string(msg.Runes)
			// Check if onboarding is active and handling paste
			if a.onboarding != nil && a.onboarding.isActive() {
				a.onboarding.apiKey = text
				a.input = ""
				a.cursorPos = 0
				a.appendLog("system", "API key pasted. Press Enter to confirm.")
				return a, nil
			}
			a.insertText(text)
			a.selected = 0
			a.isPasting = true
			return a, nil
		}

		if a.isPasting {
			a.isPasting = false
		}

		switch msg.String() {
		case "ctrl+c":
			return a, tea.Quit
		case "esc":
			a.back()
		case "up":
			a.navigateHistory(-1)
		case "down":
			a.navigateHistory(1)
		case "alt+up":
			a.scrollUp()
		case "alt+down":
			a.scrollDown()
		case "tab":
			a.acceptSelectionOrSubmit()
		case "enter":
			if a.acceptSelectionOrSubmit() {
				return a, nil
			}
			return a, a.submit()
		case "space":
			a.insertChar(' ')
			a.selected = 0
		case "backspace":
			a.deleteCharBackward()
			a.selected = 0
		case "delete":
			a.deleteCharForward()
			a.selected = 0
		case "left":
			a.moveCursorLeft()
		case "right":
			a.moveCursorRight()
		case "home", "ctrl+a":
			a.cursorPos = 0
		case "end", "ctrl+e":
			a.cursorPos = len(a.input)
		case "alt+backspace", "ctrl+w":
			a.deleteWordBackward()
			a.selected = 0
		case "alt+delete", "ctrl+u":
			a.deleteToLineStart()
			a.selected = 0
		case "ctrl+k":
			a.deleteToLineEnd()
			a.selected = 0
		case "ctrl+p":
			if a.loading {
				return a, nil
			}
			a.inputMode = "finder"
			a.input = ""
			a.cursorPos = 0
			a.selected = 0
			return a, nil
		case "ctrl+v":
			a.pasteFromClipboard()
			a.selected = 0
		case "ctrl+b":
			// Toggle sidebar
			if a.sidebar != nil {
				a.sidebar.Toggle()
			}
		case "ctrl+d":
			// Toggle detailed transcript
			if a.sidebar != nil {
				a.sidebar.Toggle()
			}
		default:
			if len(msg.String()) == 1 {
				a.insertText(msg.String())
				a.selected = 0
			}
		}
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		renderer, err := NewRenderer(max(40, msg.Width-8))
		a.renderer = renderer
		a.err = err
		// Clear render cache on resize
		if a.renderCache != nil {
			a.renderCache.clear()
		}
	case tea.MouseMsg:
		switch msg.Button {
		case tea.MouseButtonWheelUp:
			if a.viewport != nil {
				a.viewport.scrollUp()
			} else {
				a.scrollUp()
			}
		case tea.MouseButtonWheelDown:
			if a.viewport != nil {
				a.viewport.scrollDown()
			} else {
				a.scrollDown()
			}
		default:
			if isMouseClick(msg) && a.hoverManager != nil {
				action := a.hoverManager.handleClick()
				if strings.HasPrefix(action, "plan_step_click:") {
					parts := strings.SplitN(action, ":", 2)
					if len(parts) == 2 && parts[1] != "" {
						stepIdx := 0
						fmt.Sscanf(parts[1], "%d", &stepIdx)
						if a.planPanel != nil && stepIdx < len(a.planPanel.steps) {
							works := captureWorkFromLog(a.log)
							a.planStepDetail.show(stepIdx, a.planPanel.steps[stepIdx].Title, works)
						}
					}
				}
			}
			if isMouseHover(msg) && a.hoverManager != nil && a.sidebar != nil {
				planSteps := 0
				if a.planPanel != nil {
					planSteps = len(a.planPanel.steps)
				}
				touchedFiles := 0
				a.hoverManager.updateHover(
					msg.Y,
					len(a.log),
					planSteps,
					touchedFiles,
					a.sidebar.IsVisible(),
					a.planPanel != nil && a.planPanel.IsVisible(),
				)
			}
		}
	}
	return a, nil
}

func loadingTick() tea.Cmd {
	return tea.Tick(350*time.Millisecond, func(time.Time) tea.Msg {
		return loadingTickMsg{}
	})
}

func (a App) activeProfile() config.Profile {
	profile, err := a.cfg.Active()
	if err != nil {
		return config.Profile{}
	}
	return profile
}

func (a *App) appendLog(kind, text string) {
	a.log = append(a.log, logEntry{Kind: kind, Text: text, Time: time.Now()})
}

// setInput sets the input field and keeps the cursor at the end.
func (a *App) setInput(text string) {
	a.input = text
	a.cursorPos = len(text)
}

func (a *App) saveSettings() {
	if err := a.cfg.Save(); err != nil {
		a.appendLog("error", "Could not save settings: "+err.Error())
	}
}

func (a *App) scrollUp() {
	if a.scrollOffset < len(a.log)-12 {
		a.scrollOffset++
	}
}

func (a *App) scrollDown() {
	if a.scrollOffset > 0 {
		a.scrollOffset--
	}
}

func (a *App) persistMessages(messages []providers.Message, alreadyPersisted int) {
	if a.store == nil || a.sessionID == "" {
		return
	}
	if alreadyPersisted < 0 {
		alreadyPersisted = 0
	}
	if alreadyPersisted > len(messages) {
		alreadyPersisted = len(messages)
	}
	ctx := context.Background()
	for _, msg := range messages[alreadyPersisted:] {
		agentMsg := agent.Message{
			Role:    agent.Role(msg.Role),
			Content: msg.Content,
		}
		_ = a.store.AppendMessage(ctx, a.sessionID, agentMsg)
	}
}

func approvalPromptOld(call tools.Call) string {
	label := call.Name
	path, _ := call.Argument["path"].(string)
	if strings.TrimSpace(path) != "" {
		label += " " + path
	}
	prompt := "Allow tool " + label + "? [y]es [n]o [t] always allow tool"
	if strings.TrimSpace(path) != "" {
		prompt += " [d] always allow directory"
	}
	return prompt
}

func (a *App) handleApprovalKey(msg tea.KeyMsg) bool {
	if a.pendingApproval == nil {
		return false
	}

	key := strings.ToLower(msg.String())
	call := a.pendingApproval.call

	// Try permission prompt first
	if a.permissionPrompt != nil && a.permissionPrompt.active {
		choice, resolved := a.permissionPrompt.handleKey(key)
		if resolved {
			a.permissionPrompt = nil
			switch choice {
			case "allow":
				a.answerApproval(true, "Tool approved.")
				return true
			case "deny":
				a.answerApproval(false, "Tool denied.")
				return true
			case "always_allow_tool":
				_ = a.cfg.UpdateActive(func(profile *config.Profile) {
					found := false
					for _, t := range profile.AllowedTools {
						if t == call.Name {
							found = true
						}
					}
					if !found {
						profile.AllowedTools = append(profile.AllowedTools, call.Name)
					}
				})
				a.saveSettings()
				a.answerApproval(true, fmt.Sprintf("Tool %q approved and whitelisted.", call.Name))
				return true
			case "always_allow_dir":
				path, _ := call.Argument["path"].(string)
				if strings.TrimSpace(path) != "" {
					dir := filepath.Dir(path)
					_ = a.cfg.UpdateActive(func(profile *config.Profile) {
						found := false
						for _, p := range profile.AllowedPaths {
							if p == dir {
								found = true
							}
						}
						if !found {
							profile.AllowedPaths = append(profile.AllowedPaths, dir)
						}
					})
					a.saveSettings()
					a.answerApproval(true, fmt.Sprintf("Tool approved. Directory %q whitelisted.", dir))
					return true
				}
				return false
			}
		}
		return true
	}

	// Fallback to old key handling
	pathStr, _ := call.Argument["path"].(string)
	switch key {
	case "y", "enter":
		a.answerApproval(true, "Tool approved.")
		return true
	case "n", "esc":
		a.answerApproval(false, "Tool denied.")
		return true
	case "t":
		_ = a.cfg.UpdateActive(func(profile *config.Profile) {
			found := false
			for _, t := range profile.AllowedTools {
				if t == call.Name {
					found = true
				}
			}
			if !found {
				profile.AllowedTools = append(profile.AllowedTools, call.Name)
			}
		})
		a.saveSettings()
		a.answerApproval(true, fmt.Sprintf("Tool %q approved and whitelisted.", call.Name))
		return true
	case "d":
		if strings.TrimSpace(pathStr) != "" {
			dir := filepath.Dir(pathStr)
			_ = a.cfg.UpdateActive(func(profile *config.Profile) {
				found := false
				for _, p := range profile.AllowedPaths {
					if p == dir {
						found = true
					}
				}
				if !found {
					profile.AllowedPaths = append(profile.AllowedPaths, dir)
				}
			})
			a.saveSettings()
			a.answerApproval(true, fmt.Sprintf("Tool approved. Directory %q whitelisted.", dir))
			return true
		}
		return false
	case "a":
		_ = a.cfg.UpdateActive(func(profile *config.Profile) {
			profile.AutoApprove = true
		})
		a.saveSettings()
		a.answerApproval(true, "Tool approved. Auto-approve enabled.")
		return true
	default:
		return true
	}
}

func (a *App) answerApproval(allowed bool, message string) {
	pending := a.pendingApproval
	a.pendingApproval = nil
	a.permissionPrompt = nil
	if pending != nil {
		pending.response <- allowed
	}
	a.appendLog("system", message)
}
