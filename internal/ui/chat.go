package ui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"cli_mate/internal/agent"
	"cli_mate/internal/config"
	"cli_mate/internal/providers"
	"cli_mate/internal/providers/registry"
	"cli_mate/internal/tools"
	"cli_mate/pkg/httpclient"
	"cli_mate/pkg/tokenizer"
)

type chatStepMsg struct {
	entry logEntry
	c     chan tea.Msg
}

type chatContextMsg struct {
	tokens int
	c      chan tea.Msg
}

type chatDoneMsg struct {
	messages []providers.Message
	err      error
}

const (
	streamFlushInterval = 100 * time.Millisecond
	streamFlushBytes    = 512
	chatIdleNotice      = 30 * time.Second
)

func waitForChatMsg(c chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-c
		if !ok {
			return nil
		}
		return msg
	}
}

// Provider connections are cached until the selected profile changes. This
// keeps ordinary chatting fast while still reconnecting after setup edits.
func (a *App) connect() {
	profile := a.activeProfile()
	if err := registry.Validate(profile); err != nil {
		a.appendLog("error", err.Error())
		return
	}

	client := httpclient.New(a.cfg.HTTP.Timeout, a.cfg.HTTP.Retries)
	provider, err := registry.New(profile, client)
	if err != nil {
		a.appendLog("error", err.Error())
		return
	}

	a.provider = provider
	a.connected = profile
	a.messages = nil
	a.appendLog("system", fmt.Sprintf("Connected %s with model %s.", provider.Name(), profile.Model))
}

func (a *App) disconnect() {
	a.provider = nil
	a.connected = config.Profile{}
	a.messages = nil
}

func (a *App) startChat(text string) tea.Cmd {
	profile := a.activeProfile()

	if a.provider == nil || !sameConnection(profile, a.connected) {
		a.connect()
		if a.provider == nil {
			return nil
		}
		profile = a.connected
	}

	ctx, cancel := context.WithCancel(context.Background())
	a.beginRun(cancel)
	a.loading = true
	a.loadingFrame = 0
	a.loadingSteps = loadingSteps(text, profile, a.workspaceName)
	provider := a.provider
	history := append([]providers.Message{}, a.messages...)
	workspaceRoot := a.workspaceRoot
	instructions := a.instructions
	mcpConfigs := a.cfg.MCP

	c := make(chan tea.Msg, 64)
	app := *a
	go runChatAsync(ctx, &app, profile, provider, history, text, workspaceRoot, instructions, mcpConfigs, c)
	return tea.Batch(waitForChatMsg(c), loadingTick())
}

func runChatAsync(ctx context.Context, app *App, profile config.Profile, provider providers.Provider, history []providers.Message, text string, workspaceRoot string, instructions string, mcpConfigs []config.MCPConfig, c chan tea.Msg) {
	defer func() {
		if r := recover(); r != nil {
			c <- chatDoneMsg{messages: history, err: fmt.Errorf("chat panic: %v", r)}
		}
	}()
	activity := make(chan struct{}, 1)
	done := make(chan struct{})
	defer close(done)
	defer close(c)
	go func() {
		timer := time.NewTimer(chatIdleNotice)
		defer timer.Stop()
		waited := chatIdleNotice
		for {
			select {
			case <-activity:
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				waited = chatIdleNotice
				timer.Reset(chatIdleNotice)
			case <-timer.C:
				msg := chatLoadingStepMsg{text: fmt.Sprintf("Waiting for provider response (%s without output)", waited), c: c}
				select {
				case c <- msg:
				case <-done:
				case <-ctx.Done():
				default:
				}
				waited += chatIdleNotice
				timer.Reset(chatIdleNotice)
			case <-done:
				return
			case <-ctx.Done():
				return
			}
		}
	}()
	resetActivity := func() {
		select {
		case activity <- struct{}{}:
		default:
		}
	}
	resetActivity()
	counter := tokenizer.New(profile.Model)
	prompt, mentionEntries := promptWithMentions(workspaceRoot, text)
	casualPrompt := agent.IsConversationalPrompt(text)
	if len(history) == 0 && !casualPrompt {
		prompt = fmt.Sprintf("Act as a senior developer with 10 years of experience.\nWorkspace Context: %s\n\nUser Request:\n%s\n\nWhen you finish the task, verify the work once and explain the result in readable sections: Summary, Files changed, Verification, and Notes if needed.", filepath.Base(workspaceRoot), prompt)
	}
	for _, entry := range mentionEntries {
		c <- chatStepMsg{entry: entry, c: c}
	}

	c <- chatLoadingStepMsg{text: fmt.Sprintf("Calling %s %s", profile.Provider, profile.Model), c: c}
	var pendingStream strings.Builder
	var filter streamFilter
	lastToolDraftName := ""
	lastToolDraftPath := ""
	lastStreamFlush := time.Now()
	flushStream := func(force bool) {
		if pendingStream.Len() == 0 {
			return
		}
		if !force && pendingStream.Len() < streamFlushBytes && time.Since(lastStreamFlush) < streamFlushInterval {
			return
		}
		c <- chatStreamMsg{token: pendingStream.String(), c: c}
		pendingStream.Reset()
		lastStreamFlush = time.Now()
	}

	// Initialize background session manager
	bgManager := tools.NewBackgroundManager(workspaceRoot)

	// Initialize plan mode
	planMode := tools.NewPlanMode()

	toolset := []tools.Tool{
		// Core file tools
		tools.NewFileReadTool(workspaceRoot),
		tools.NewFileEditTool(workspaceRoot),
		tools.NewFileWriteTool(workspaceRoot),
		tools.NewApplyPatchTool(workspaceRoot),
		tools.NewShellTool(workspaceRoot, 45*time.Second),
		tools.NewGlobTool(workspaceRoot),
		tools.NewGrepTool(workspaceRoot),
		tools.NewFileListTool(workspaceRoot),
		tools.NewReadSubtreeTool(workspaceRoot),
		// Web tools
		tools.NewWebSearchTool(),
		tools.NewWebFetchTool(),
		// Task management
		tools.NewTodoWriteTool(),
		// Plan mode
		tools.NewEnterPlanModeTool(planMode),
		tools.NewExitPlanModeTool(planMode),
		tools.NewVerifyPlanExecutionTool(planMode),
		// Skills
		tools.NewSkillTool(workspaceRoot),
		tools.NewDiscoverSkillsTool(workspaceRoot),
		// Code review & diff
		tools.NewReviewTool(workspaceRoot),
		tools.NewDiffTool(workspaceRoot),
		tools.NewCommitTool(workspaceRoot),
		tools.NewCompactTool(workspaceRoot),
		// Security
		tools.NewSecretScanTool(workspaceRoot),
		// Model escalation
		tools.NewEscalateModelTool(nil),
		// Git worktrees
		tools.NewWorktreeCreateTool(workspaceRoot),
		tools.NewWorktreeListTool(workspaceRoot),
		tools.NewWorktreeCleanupTool(workspaceRoot),
		// Background sessions
		tools.NewBGRunTool(bgManager),
		tools.NewBGStatusTool(bgManager),
		tools.NewBGLogsTool(bgManager),
		tools.NewBGKillTool(bgManager),
	}

	// Connect MCP servers
	var mcpClients []*tools.MCPClient
	for _, mc := range mcpConfigs {
		resetActivity()
		client := tools.NewMCPClient(mc.Command, mc.Args)
		if err := client.Connect(ctx); err != nil {
			c <- chatStepMsg{entry: logEntry{Kind: "error", Text: fmt.Sprintf("MCP server %q failed: %v", mc.Name, err), Time: time.Now()}, c: c}
			continue
		}
		mcpClients = append(mcpClients, client)
	}
	defer func() {
		for _, c := range mcpClients {
			c.Close()
		}
	}()

	runner := agent.NewCodingRunner(provider, instructions, toolset, workspaceRoot)
	runner.EnableSpecialists()
	runner.Style = app.responseStyle
	if profile.MaxToolIterations > 0 {
		runner.MaxIterations = profile.MaxToolIterations
	}
	selfCorrector := agent.NewSelfCorrector(workspaceRoot)
	result, err := runner.Run(ctx, agent.RunOptions{
		Model:         profile.Model,
		History:       history,
		Prompt:        prompt,
		MaxTokens:     profile.MaxTokens,
		ReserveTokens: profile.ReserveTokens,
		Temperature:   profile.Temperature,
		Counter:       counter,
		OnStep: func(step agent.Step) {
			resetActivity()
			entry := logEntry{Kind: step.Kind, Text: step.Text, Time: time.Now()}
			c <- chatStepMsg{entry: entry, c: c}
			c <- chatLoadingStepMsg{text: step.Text, c: c}

			// Send tool result message for streaming UI
			if step.Kind == "tool" {
				toolName := parseToolName(step.Text)
				if toolName != "" && isFileWritingTool(toolName) {
					c <- chatToolResultMsg{toolName: toolName, result: step.Text, c: c}
				}
			}
		},
		OnContext: func(tokens int) {
			resetActivity()
			c <- chatContextMsg{tokens: tokens, c: c}
		},
		OnToken: func(token string) {
			resetActivity()
			filtered := filter.Push(token)
			if filtered.ToolStarted {
				pendingStream.Reset()
				c <- chatStreamMsg{clear: true, c: c}
				c <- chatLoadingStepMsg{text: "Preparing tool call", c: c}
			}
			if filtered.ToolDraft != "" {
				name := streamedToolName(filtered.ToolDraft)
				path := streamingFilePath(filtered.ToolDraft)
				if filtered.ToolStarted || name != lastToolDraftName || path != lastToolDraftPath {
					lastToolDraftName = name
					lastToolDraftPath = path
					c <- chatToolCallMsg{toolName: name, args: filtered.ToolDraft, c: c}
				}
			}
			if filtered.Visible != "" {
				pendingStream.WriteString(filtered.Visible)
				flushStream(false)
			}
		},
		OnUsage: func(u providers.Usage) {
			// Token usage from compaction summarizer calls.
			_ = u
		},
		OnActivity:    resetActivity,
		DisableTools:  casualPrompt,
		SelfCorrector: selfCorrector,
		ApproveTool: func(call tools.Call) bool {
			resetActivity()
			currentProfile := app.activeProfile()
			path, _ := call.Argument["path"].(string)

			// Send tool call message for approval
			if isFileWritingTool(call.Name) {
				pathArg := path
				if pathArg == "" {
					pathArg, _ = call.Argument["file_path"].(string)
				}
				args := fmt.Sprintf("%s path=%s", call.Name, pathArg)
				c <- chatToolCallMsg{toolName: call.Name, args: args, c: c}
			}

			if !currentProfile.IsAllowed(call.Name, path) && !app.RequestApproval(ctx, call) {
				return false
			}
			if record, ok := editSnapshot(workspaceRoot, call); ok {
				c <- chatEditSnapshotMsg{record: record, c: c}
			}
			return true
		},
	})
	if visible := filter.Flush(); visible != "" {
		pendingStream.WriteString(visible)
	}
	flushStream(true)
	if err != nil {
		c <- chatStepMsg{entry: logEntry{Kind: "error", Text: err.Error(), Time: time.Now()}, c: c}
		c <- chatDoneMsg{messages: history, err: err}
		return
	}
	c <- chatStepMsg{entry: logEntry{Kind: "assistant", Text: result.Answer, Time: time.Now()}, c: c}
	c <- chatDoneMsg{messages: result.Messages, err: nil}
}

func editSnapshot(root string, call tools.Call) (editRecord, bool) {
	if call.Name != "file_edit" && call.Name != "file_write" {
		return editRecord{}, false
	}
	path, _ := call.Argument["path"].(string)
	if strings.TrimSpace(path) == "" {
		return editRecord{}, false
	}
	resolved, err := resolveUIWorkspacePath(root, path)
	if err != nil {
		return editRecord{}, false
	}
	data, err := os.ReadFile(resolved)
	if err != nil {
		if os.IsNotExist(err) && call.Name == "file_write" {
			return editRecord{path: resolved, existed: false}, true
		}
		return editRecord{}, false
	}
	return editRecord{path: resolved, content: string(data), existed: true}, true
}

func resolveUIWorkspacePath(root string, input string) (string, error) {
	if strings.TrimSpace(input) == "" {
		return "", fmt.Errorf("path is required")
	}
	if root == "" {
		var err error
		root, err = os.Getwd()
		if err != nil {
			return "", err
		}
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	path := filepath.FromSlash(input)
	if !filepath.IsAbs(path) {
		path = filepath.Join(absRoot, path)
	}
	path = filepath.Clean(path)
	rel, err := filepath.Rel(absRoot, path)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", fmt.Errorf("path is outside workspace")
	}
	return path, nil
}

func sameConnection(current config.Profile, connected config.Profile) bool {
	return current.Provider == connected.Provider &&
		current.Model == connected.Model &&
		current.APIKey == connected.APIKey &&
		current.BaseURL == connected.BaseURL
}

func promptWithMentions(root string, text string) (string, []logEntry) {
	mentions := mentionedFiles(text)
	if len(mentions) == 0 {
		return text, nil
	}

	var b strings.Builder
	b.WriteString(text)
	var entries []logEntry
	for _, mention := range mentions {
		content, err := readMention(root, mention)
		if err != nil {
			entries = append(entries, logEntry{Kind: "error", Text: fmt.Sprintf("Could not read @%s: %v", mention, err), Time: time.Now()})
			continue
		}
		entries = append(entries, logEntry{Kind: "file", Text: "Included @" + mention, Time: time.Now()})
		b.WriteString("\n\n--- file: ")
		b.WriteString(mention)
		b.WriteString(" ---\n")
		b.WriteString(content)
	}
	return b.String(), entries
}

func readMention(root string, mention string) (string, error) {
	if strings.TrimSpace(mention) == "" {
		return "", fmt.Errorf("empty file mention")
	}
	if root == "" {
		var err error
		root, err = os.Getwd()
		if err != nil {
			return "", err
		}
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	path := filepath.FromSlash(mention)
	if !filepath.IsAbs(path) {
		path = filepath.Join(absRoot, path)
	}
	path = filepath.Clean(path)
	rel, err := filepath.Rel(absRoot, path)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", fmt.Errorf("file mention is outside workspace")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	const maxMentionBytes = 24000
	if len(data) > maxMentionBytes {
		return string(data[:maxMentionBytes]) + "\n... truncated ...", nil
	}
	return string(data), nil
}
