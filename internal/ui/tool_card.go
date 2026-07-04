package ui

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// cardBody represents the rendered body of a tool card.
type cardBody struct {
	header  string
	content string
	footer  string
	diffAdd int
	diffDel int
}

// toolBodyRenderer renders a tool call result into a structured card body.
type toolBodyRenderer interface {
	renderToolBody(req toolBodyRequest) cardBody
}

// toolBodyRendererFunc adapts a function to the toolBodyRenderer interface.
type toolBodyRendererFunc func(toolBodyRequest) cardBody

func (f toolBodyRendererFunc) renderToolBody(req toolBodyRequest) cardBody {
	return f(req)
}

// toolBodyRequest contains the data needed to render a tool result.
type toolBodyRequest struct {
	name       string
	arg        string
	detail     string // The full tool result text
	hint       string
	path       string
	argsJSON   map[string]interface{} // Parsed JSON arguments
}

// toolBodyRegistry maps tool names to their specific renderers.
type toolBodyRegistry struct {
	renderers map[string]toolBodyRenderer
	fallback  toolBodyRenderer
}

// newDefaultToolBodyRegistry creates the default registry with all tool renderers.
func newDefaultToolBodyRegistry() *toolBodyRegistry {
	r := &toolBodyRegistry{
		renderers: make(map[string]toolBodyRenderer),
		fallback:  toolBodyRendererFunc(renderFallbackToolBody),
	}

	// Diff-first renderers for file editing tools
	diffRenderer := toolBodyRendererFunc(renderDiffToolBody)
	r.renderers["edit_file"] = diffRenderer
	r.renderers["apply_patch"] = diffRenderer
	r.renderers["write_file"] = diffRenderer
	r.renderers["file_edit"] = diffRenderer
	r.renderers["file_write"] = diffRenderer
	r.registerBatch([]string{"str_replace", "write"}, diffRenderer)

	// Explore renderers for file reading/listing tools
	exploreRenderer := toolBodyRendererFunc(renderExploreToolBody)
	r.renderers["read_file"] = exploreRenderer
	r.renderers["file_read"] = exploreRenderer
	r.registerBatch([]string{
		"list_directory", "glob", "grep", "file_list",
		"read_subtree",
	}, exploreRenderer)

	// Bash/Shell renderer
	bashRenderer := toolBodyRendererFunc(renderBashToolBody)
	r.renderers["bash"] = bashRenderer
	r.renderers["shell"] = bashRenderer
	r.renderers["run_terminal_command"] = bashRenderer

	// Web tool renderers
	webRenderer := toolBodyRendererFunc(renderWebToolBody)
	r.renderers["web_search"] = webRenderer
	r.renderers["web_fetch"] = webRenderer
	r.renderers["web_search"] = webRenderer
	r.renderers["read_url"] = webRenderer

	// System/plan tools - collapsed
	r.renderers["update_plan"] = toolBodyRendererFunc(renderPlanSummaryToolBody)
	r.renderers["todo_write"] = toolBodyRendererFunc(renderPlanSummaryToolBody)
	r.renderers["tool_search"] = toolBodyRendererFunc(renderHiddenToolBody)
	r.renderers["skill"] = toolBodyRendererFunc(renderHiddenToolBody)
	r.renderers["discover_skills"] = toolBodyRendererFunc(renderHiddenToolBody)
	r.renderers["enter_plan_mode"] = toolBodyRendererFunc(renderHiddenToolBody)
	r.renderers["exit_plan_mode"] = toolBodyRendererFunc(renderHiddenToolBody)

	return r
}

func (r *toolBodyRegistry) registerBatch(names []string, renderer toolBodyRenderer) {
	for _, n := range names {
		r.renderers[n] = renderer
	}
}

// render returns the rendered card body for the given tool request.
func (r *toolBodyRegistry) render(req toolBodyRequest) cardBody {
	renderer, ok := r.renderers[req.name]
	if !ok {
		renderer = r.fallback
	}
	return renderer.renderToolBody(req)
}

// --- Individual Renderers ---

// renderDiffToolBody renders a diff-focused card for file edit tools.
func renderDiffToolBody(req toolBodyRequest) cardBody {
	detail := req.detail
	if detail == "" {
		detail = req.arg
	}

	// Extract structured info from JSON args
	path := extractArgString(req.argsJSON, "path", "file_path", "file")
	if path == "" {
		path = req.path
	}
	body := cardBody{
		header: fmt.Sprintf("✎ %s", path),
	}

	// Check if content looks like a diff
	lines := strings.Split(detail, "\n")
	var added, removed int
	var contentLines []string

	maxLines := 30
	for _, line := range lines {
		if len(contentLines) >= maxLines {
			contentLines = append(contentLines, fmt.Sprintf("  ... +%d more lines", len(lines)-maxLines))
			break
		}
		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			contentLines = append(contentLines, lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Render("┃ "+line))
			added++
		} else if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
			contentLines = append(contentLines, lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render("┃ "+line))
			removed++
		} else if strings.HasPrefix(line, "@@") {
			contentLines = append(contentLines, lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Render("┃ "+line))
		} else if strings.HasPrefix(line, " ") {
			contentLines = append(contentLines, "┃ "+line)
		} else if strings.TrimSpace(line) != "" {
			contentLines = append(contentLines, "  "+line)
		}
	}

	body.content = strings.Join(contentLines, "\n")
	body.diffAdd = added
	body.diffDel = removed

	if len(lines) > maxLines {
		body.footer = fmt.Sprintf("  (%d lines total)", len(lines))
	} else {
		body.footer = fmt.Sprintf("  +%d -%d", added, removed)
	}

	return body
}

// renderExploreToolBody renders a compact card for file reading/listing.
func renderExploreToolBody(req toolBodyRequest) cardBody {
	path := extractArgString(req.argsJSON, "pattern", "path", "file_path")
	if path == "" {
		path = req.path
	}
	body := cardBody{
		header: fmt.Sprintf("📄 %s", path),
	}

	detail := req.detail
	if detail == "" {
		detail = req.arg
	}

	lines := strings.Split(detail, "\n")
	const maxPreviewLines = 15

	if len(lines) <= maxPreviewLines+1 {
		body.content = detail
		body.footer = fmt.Sprintf("%d lines", len(lines))
	} else {
		preview := strings.Join(lines[:maxPreviewLines], "\n")
		body.content = preview
		body.footer = fmt.Sprintf("%d lines (+%d collapsed)", len(lines), len(lines)-maxPreviewLines)
	}

	return body
}

// renderBashToolBody renders a card for bash/shell command output.
func renderBashToolBody(req toolBodyRequest) cardBody {
	cmd := extractArgString(req.argsJSON, "command", "cmd", "script")
	if cmd == "" {
		cmd = req.arg
	}
	body := cardBody{
		header: fmt.Sprintf("$ %s", truncateString(cmd, 60)),
	}

	detail := req.detail
	if detail == "" {
		detail = req.arg
	}

	const maxOutputLines = 20
	lines := strings.Split(detail, "\n")

	if len(lines) <= maxOutputLines+1 {
		body.content = detail
		body.footer = fmt.Sprintf("exit: %d lines", len(lines))
	} else {
		// Show first and last few lines
		head := lines[:8]
		tail := lines[len(lines)-8:]
		body.content = strings.Join(head, "\n") +
			fmt.Sprintf("\n  ... %d lines omitted ...\n", len(lines)-16) +
			strings.Join(tail, "\n")
		body.footer = fmt.Sprintf("%d total lines", len(lines))
	}

	return body
}

// renderWebToolBody renders a card for web search/fetch results.
func renderWebToolBody(req toolBodyRequest) cardBody {
	body := cardBody{
		header: fmt.Sprintf("🌐 %s", req.name),
	}

	detail := req.detail
	if detail == "" {
		detail = req.arg
	}

	const maxLines = 12
	lines := strings.Split(detail, "\n")
	if len(lines) > maxLines {
		body.content = strings.Join(lines[:maxLines], "\n") +
			fmt.Sprintf("\n  ... %d more results ...", len(lines)-maxLines)
	} else {
		body.content = detail
	}

	return body
}

// renderPlanSummaryToolBody renders a compact summary for plan/skill updates.
func renderPlanSummaryToolBody(req toolBodyRequest) cardBody {
	detail := req.detail
	if detail == "" {
		detail = req.arg
	}
	lines := strings.Split(detail, "\n")
	if len(lines) > 3 {
		detail = strings.Join(lines[:3], "\n")
	}

	return cardBody{
		header:  fmt.Sprintf("📋 %s", req.name),
		content: detail,
	}
}

// renderHiddenToolBody renders minimal output for hidden plumbing tools.
func renderHiddenToolBody(req toolBodyRequest) cardBody {
	return cardBody{
		header:  fmt.Sprintf("⚙ %s", req.name),
		content: "",
	}
}

// renderFallbackToolBody renders a generic card for unregistered tools.
func renderFallbackToolBody(req toolBodyRequest) cardBody {
	body := cardBody{
		header: fmt.Sprintf("🔧 %s", req.name),
	}

	detail := req.detail
	if detail == "" {
		detail = req.arg
	}

	const maxLines = 10
	lines := strings.Split(detail, "\n")
	if len(lines) > maxLines {
		body.content = strings.Join(lines[:maxLines], "\n") +
			fmt.Sprintf("\n  ... %d more lines ...", len(lines)-maxLines)
	} else {
		body.content = detail
	}

	return body
}

// --- Card Rendering (with styles) ---

// toolCard renders a complete tool card using the registry and styles.
func (r *toolBodyRegistry) renderCard(req toolBodyRequest, styles appStyles, width int) string {
	body := r.render(req)

	// Skip rendering hidden tools entirely
	if body.content == "" && body.header == "" && body.footer == "" {
		return ""
	}

	var b strings.Builder

	// Header
	b.WriteString(styles.roleTool.Render(body.header))
	b.WriteString("\n")

	// Content
	if body.content != "" {
		cardContent := styles.softPanel.
			Width(width - 8).
			Render(body.content)
		b.WriteString(cardContent)
		b.WriteString("\n")
	}

	// Footer
	if body.footer != "" {
		diffStr := ""
		if body.diffAdd > 0 || body.diffDel > 0 {
			diffStr = fmt.Sprintf("  %s+%d %s-%d",
				styles.diffAdd.Render(""),
				body.diffAdd,
				styles.diffRemove.Render(""),
				body.diffDel,
			)
		}
		footer := styles.muted.Render(body.footer + diffStr)
		b.WriteString(footer)
		b.WriteString("\n")
	}

	return b.String()
}

// isHiddenPlumbingTool returns true if the tool should be hidden from the transcript.
func isHiddenPlumbingTool(name string) bool {
	switch name {
	case "update_plan", "tool_search", "enter_plan_mode", "exit_plan_mode",
		"discover_skills", "verify_plan_execution":
		return true
	}
	return false
}

// --- OSC 8 Hyperlinks ---

// hyperlink wraps text in an OSC 8 hyperlink escape sequence.
// This makes file paths clickable in terminals that support it (kitty, WezTerm, iTerm2, etc.).
func hyperlink(text, target string) string {
	if target == "" {
		return text
	}
	// OSC 8 sequence: <esc>]8;;<uri><bell><text><esc>]8;;<bell>
	return fmt.Sprintf("\x1b]8;;%s\x07%s\x1b]8;;\x07", target, text)
}

// hyperlinkPath wraps a file path in a file:// hyperlink.
func hyperlinkPath(path string) string {
	if path == "" {
		return ""
	}
	// Use file:// URI
	target := "file://" + path
	return hyperlink(path, target)
}

// displayPath shortens a path for display while keeping it informative.
// It uses ~/ for home dirs and shows the last two segments.
func displayPath(path string) string {
	if path == "" {
		return ""
	}
	// Normalize to forward slashes
	normalized := strings.ReplaceAll(path, "\\", "/")
	parts := strings.Split(normalized, "/")

	if len(parts) <= 3 {
		return normalized
	}

	// Show last 2 segments
	tail := parts[len(parts)-2:]
	return ".../" + strings.Join(tail, "/")
}

// --- Word-Level Diff Highlighting ---

// wordDiff computes a simple word-level diff between two strings.
// Returns the old and new strings with change markers for rendering.
type wordDiffResult struct {
	OldWords  []wordSpan
	NewWords  []wordSpan
}

type wordSpan struct {
	Text   string
	IsDiff bool // true if this span differs between old/new
}

// computeWordDiff computes word-level changes between oldText and newText.
// Useful for rendering precise word-level diffs in addition to line-level.
func computeWordDiff(oldText, newText string) wordDiffResult {
	if oldText == newText {
		return wordDiffResult{
			OldWords: []wordSpan{{Text: oldText}},
			NewWords: []wordSpan{{Text: newText}},
		}
	}

	// Split into words (keep whitespace)
	oldWords := splitWords(oldText)
	newWords := splitWords(newText)

	// Simple LCS-based diff
	lcs := computeLCS(oldWords, newWords)

	var result wordDiffResult

	// Mark old words
	oidx, lidx := 0, 0
	for oidx < len(oldWords) {
		if lidx < len(lcs) && oldWords[oidx] == lcs[lidx] {
			result.OldWords = append(result.OldWords, wordSpan{Text: oldWords[oidx]})
			lidx++
		} else {
			result.OldWords = append(result.OldWords, wordSpan{Text: oldWords[oidx], IsDiff: true})
		}
		oidx++
	}

	// Mark new words
	nidx, lidx := 0, 0
	for nidx < len(newWords) {
		if lidx < len(lcs) && newWords[nidx] == lcs[lidx] {
			result.NewWords = append(result.NewWords, wordSpan{Text: newWords[nidx]})
			lidx++
		} else {
			result.NewWords = append(result.NewWords, wordSpan{Text: newWords[nidx], IsDiff: true})
		}
		nidx++
	}

	return result
}

// splitWords splits text into words including whitespace.
func splitWords(text string) []string {
	var words []string
	var current strings.Builder
	for _, r := range text {
		if r == ' ' || r == '\t' {
			if current.Len() > 0 {
				words = append(words, current.String())
				current.Reset()
			}
			words = append(words, string(r))
		} else {
			current.WriteRune(r)
		}
	}
	if current.Len() > 0 {
		words = append(words, current.String())
	}
	return words
}

// computeLCS computes the longest common subsequence of two string slices.
// Uses full DP matrix to enable backtracking.
func computeLCS(a, b []string) []string {
	m, n := len(a), len(b)
	if m == 0 || n == 0 {
		return nil
	}

	// Build full DP matrix for backtracking
	dp := make([][]int, m+1)
	for i := 0; i <= m; i++ {
		dp[i] = make([]int, n+1)
	}

	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if a[i-1] == b[j-1] {
				dp[i][j] = dp[i-1][j-1] + 1
			} else if dp[i-1][j] > dp[i][j-1] {
				dp[i][j] = dp[i-1][j]
			} else {
				dp[i][j] = dp[i][j-1]
			}
		}
	}

	// Backtrack to find LCS using the full matrix
	result := make([]string, 0, dp[m][n])
	i, j := m, n
	for i > 0 && j > 0 {
		if a[i-1] == b[j-1] {
			result = append([]string{a[i-1]}, result...)
			i--
			j--
		} else if dp[i-1][j] > dp[i][j-1] {
			i--
		} else {
			j--
		}
	}

	return result
}

// --- Reasoning Block ---

// reasoningBlock represents a collapsible reasoning/thinking block.
type reasoningBlock struct {
	text      string
	collapsed bool
	startedAt time.Time
	elapsed   time.Duration
}

// newReasoningBlock creates a new reasoning block.
func newReasoningBlock(text string) *reasoningBlock {
	return &reasoningBlock{
		text:      text,
		collapsed: false,
		startedAt: time.Now(),
	}
}

// toggleCollapse toggles the collapsed state.
func (rb *reasoningBlock) toggleCollapse() {
	rb.collapsed = !rb.collapsed
}

// renderReasoningBlock renders the reasoning block.
func renderReasoningBlock(rb *reasoningBlock, styles appStyles, _ int) string {
	if rb.text == "" {
		return ""
	}

	rb.elapsed = time.Since(rb.startedAt).Round(time.Second)

	elapsedStr := styles.muted.Render(fmt.Sprintf("(%s)", rb.elapsed))
	var header string
	if rb.collapsed {
		header = fmt.Sprintf("%s %s %s",
			styles.roleSystem.Render("▶ reasoning"),
			styles.muted.Render(fmt.Sprintf("%d chars", len(rb.text))),
			elapsedStr,
		)
	} else {
		header = fmt.Sprintf("%s %s",
			styles.roleSystem.Render("▼ reasoning"),
			elapsedStr,
		)
	}

	var b strings.Builder
	b.WriteString(header)
	b.WriteString("\n")

	if !rb.collapsed {
		const maxReasoningLines = 20
		lines := strings.Split(rb.text, "\n")
		if len(lines) > maxReasoningLines {
			lines = lines[:maxReasoningLines]
			lines = append(lines, styles.muted.Render(fmt.Sprintf("  ... %d more lines ...", len(strings.Split(rb.text, "\n"))-maxReasoningLines)))
		}
		content := strings.Join(lines, "\n")
		b.WriteString(styles.muted.Render(content))
		b.WriteString("\n")
	}

	return b.String()
}

// truncateString is defined in util.go
// truncateString(s string, maxLen int) string

// --- Structured Argument Extraction ---

// extractArgString extracts a string value from JSON args by trying multiple keys.
func extractArgString(args map[string]interface{}, keys ...string) string {
	if args == nil {
		return ""
	}
	for _, key := range keys {
		if v, ok := args[key]; ok {
			if s, ok := v.(string); ok {
				return s
			}
		}
	}
	return ""
}

// extractArgInt extracts an integer value from JSON args.
func extractArgInt(args map[string]interface{}, keys ...string) int {
	if args == nil {
		return 0
	}
	for _, key := range keys {
		if v, ok := args[key]; ok {
			switch n := v.(type) {
			case float64:
				return int(n)
			case int:
				return n
			}
		}
	}
	return 0
}

// extractArgBool extracts a boolean value from JSON args.
func extractArgBool(args map[string]interface{}, keys ...string) bool {
	if args == nil {
		return false
	}
	for _, key := range keys {
		if v, ok := args[key]; ok {
			if b, ok := v.(bool); ok {
				return b
			}
		}
	}
	return false
}

// parseToolArgsJSON attempts to parse a JSON string into a map.
func parseToolArgsJSON(args string) map[string]interface{} {
	if args == "" {
		return nil
	}
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(args), &result); err == nil {
		return result
	}
	return nil
}

// buildToolRequest creates a toolBodyRequest from a tool call name and its arguments.
func buildToolRequest(name string, argsJSON string, detail string) toolBodyRequest {
	parsed := parseToolArgsJSON(argsJSON)
	req := toolBodyRequest{
		name:     name,
		arg:      argsJSON,
		detail:   detail,
		argsJSON: parsed,
	}
	// Extract path from JSON
	if parsed != nil {
		req.path = extractArgString(parsed, "path", "file_path", "file", "pattern")
	}
	// Fallback to parsed name-based guesses
	if req.path == "" {
		req.path = parseToolPath(detail)
	}
	return req
}
