package ui

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const fileViewMaxLines = 4000

const (
	fileViewDiff = iota
	fileViewFull
)

type fileViewState struct {
	active             bool
	path               string
	mode               int
	parentScrollOffset int
}

func (a *App) openFileView(path string) {
	if a.fileView.active && a.fileView.path == path {
		return
	}
	if !a.fileView.active {
		a.fileView.parentScrollOffset = a.scrollOffset
	}
	a.fileView.active = true
	a.fileView.path = path
	a.fileView.mode = fileViewDiff
	a.scrollOffset = 0
}

func (a *App) exitFileView() {
	if !a.fileView.active {
		return
	}
	a.scrollOffset = a.fileView.parentScrollOffset
	a.fileView = fileViewState{}
}

func (a *App) setFileViewMode(mode int) {
	if !a.fileView.active || a.fileView.mode == mode {
		return
	}
	a.fileView.mode = mode
	a.scrollOffset = 0
}

func (a *App) renderFileView(width int) string {
	if !a.fileView.active {
		return ""
	}

	modeLabel := "diff"
	if a.fileView.mode == fileViewFull {
		modeLabel = "full"
	}

	header := fmt.Sprintf("%s  ·  %s  [d:diff f:full esc:back]", a.fileView.path, modeLabel)

	var body string
	if a.fileView.mode == fileViewFull {
		body = a.renderFileViewFull(width)
	} else {
		body = a.renderFileViewDiff(width)
	}

	return header + "\n\n" + body
}

func (a *App) renderFileViewDiff(width int) string {
	target := a.fileView.path
	if !filepath.IsAbs(target) {
		target = filepath.Join(a.workspaceRoot, target)
	}

	changed := a.fileViewChangedLines()
	file, err := os.Open(target)
	if err != nil {
		return fmt.Sprintf("Could not read file: %s", err.Error())
	}
	defer file.Close()

	var lines []string
	truncated := false
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for scanner.Scan() {
		if len(lines) == fileViewMaxLines {
			truncated = true
			break
		}
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil && len(lines) == 0 {
		return fmt.Sprintf("Could not read file: %s", err.Error())
	}

	var b strings.Builder
	gutterW := len(fmt.Sprintf("%d", len(lines)))
	textBudget := width - gutterW - 3
	if textBudget < 8 {
		textBudget = 8
	}
	for i, line := range lines {
		if i > 0 {
			b.WriteString("\n")
		}
		marker := " "
		if changed[strings.TrimSpace(line)] {
			marker = "▎"
		}
		b.WriteString(fmt.Sprintf("%*d %s%s", gutterW, i+1, marker, line))
	}
	if truncated {
		b.WriteString(fmt.Sprintf("\n… more lines (file truncated at %d for display)", len(lines)))
	}
	return b.String()
}

func (a *App) renderFileViewFull(width int) string {
	target := a.fileView.path
	if !filepath.IsAbs(target) {
		target = filepath.Join(a.workspaceRoot, target)
	}

	data, err := os.ReadFile(target)
	if err != nil {
		return fmt.Sprintf("Could not read file: %s", err.Error())
	}
	lines := strings.Split(string(data), "\n")
	if len(lines) > fileViewMaxLines {
		lines = lines[:fileViewMaxLines]
	}

	var b strings.Builder
	gutterW := len(fmt.Sprintf("%d", len(lines)))
	textBudget := width - gutterW - 3
	if textBudget < 8 {
		textBudget = 8
	}
	for i, line := range lines {
		if i > 0 {
			b.WriteString("\n")
		}
		display := line
		if len(display) > textBudget {
			display = display[:textBudget]
		}
		b.WriteString(fmt.Sprintf("%*d  %s", gutterW, i+1, display))
	}
	if uint64(len(lines)) < uint64(len(strings.Split(string(data), "\n"))) {
		b.WriteString(fmt.Sprintf("\n… more lines (file truncated at %d for display)", len(lines)))
	}
	return b.String()
}

func (a *App) fileViewChangedLines() map[string]bool {
	changed := map[string]bool{}
	for _, entry := range a.log {
		if entry.Kind != "tool" {
			continue
		}
		for _, line := range strings.Split(entry.Text, "\n") {
			if !strings.HasPrefix(line, "+") || strings.HasPrefix(line, "+++") {
				continue
			}
			if text := strings.TrimSpace(strings.TrimPrefix(line, "+")); len(text) >= 4 {
				changed[text] = true
			}
		}
	}
	return changed
}
