package ui

import (
	"fmt"
	"strings"
)

// DiffLine represents a single line in a diff.
type DiffLine struct {
	Type    string // "+", "-", " "
	Content string
	OldNum  int
	NewNum  int
}

// DiffHunk represents a group of related changes.
type DiffHunk struct {
	StartLine int
	Lines     []DiffLine
}

// DiffFile represents a diff for a single file.
type DiffFile struct {
	Path    string
	Hunks   []DiffHunk
	Added   int
	Removed int
}

// DiffViewer displays file diffs.
type DiffViewer struct {
	visible   bool
	files     []DiffFile
	selection int
	scrollOff int
}

// NewDiffViewer creates a new diff viewer.
func NewDiffViewer() *DiffViewer {
	return &DiffViewer{}
}

// Toggle shows/hides the diff viewer.
func (dv *DiffViewer) Toggle() {
	dv.visible = !dv.visible
}

// SetVisible sets the visibility of the diff viewer.
func (dv *DiffViewer) SetVisible(visible bool) {
	dv.visible = visible
}

// IsVisible returns whether the diff viewer is visible.
func (dv *DiffViewer) IsVisible() bool {
	return dv.visible
}

// SetFiles sets the diff files to display.
func (dv *DiffViewer) SetFiles(files []DiffFile) {
	dv.files = files
	dv.selection = 0
	dv.scrollOff = 0
}

// MoveUp moves the selection up.
func (dv *DiffViewer) MoveUp() {
	if dv.selection > 0 {
		dv.selection--
	}
	dv.adjustScroll()
}

// MoveDown moves the selection down.
func (dv *DiffViewer) MoveDown() {
	if dv.selection < len(dv.files)-1 {
		dv.selection++
	}
	dv.adjustScroll()
}

func (dv *DiffViewer) adjustScroll() {
	maxVisible := 20
	if dv.selection < dv.scrollOff {
		dv.scrollOff = dv.selection
	}
	if dv.selection >= dv.scrollOff+maxVisible {
		dv.scrollOff = dv.selection - maxVisible + 1
	}
}

// Render produces the diff viewer view.
func (dv *DiffViewer) Render(width int, styles appStyles) string {
	if !dv.visible {
		return ""
	}

	var lines []string
	lines = append(lines, styles.pill.Render("Diff Viewer"))
	lines = append(lines, "")

	if len(dv.files) == 0 {
		lines = append(lines, styles.muted.Render("No changes to display."))
		return strings.Join(lines, "\n")
	}

	// File list
	lines = append(lines, styles.accent.Render("Files:"))
	for i, file := range dv.files {
		icon := "  "
		if i == dv.selection {
			icon = "▸ "
		}
		summary := fmt.Sprintf("+%d -%d", file.Added, file.Removed)
		if i == dv.selection {
			lines = append(lines, styles.selected.Render(icon+file.Path)+styles.muted.Render(" "+summary))
		} else {
			lines = append(lines, icon+file.Path+styles.muted.Render(" "+summary))
		}
	}

	// Show selected file's diff
	if dv.selection >= 0 && dv.selection < len(dv.files) {
		file := dv.files[dv.selection]
		lines = append(lines, "")
		lines = append(lines, styles.accent.Render("Changes in "+file.Path+":"))
		lines = append(lines, "")

		for _, hunk := range file.Hunks {
			lines = append(lines, styles.muted.Render(fmt.Sprintf("@@ -%d +%d @@", hunk.StartLine, hunk.StartLine)))
			for _, line := range hunk.Lines {
				styled := dv.styleDiffLine(line, styles)
				lines = append(lines, styled)
			}
		}
	}

	return strings.Join(lines, "\n")
}

func (dv *DiffViewer) styleDiffLine(line DiffLine, styles appStyles) string {
	switch line.Type {
	case "+":
		return styles.success.Render("+ " + line.Content)
	case "-":
		return styles.error.Render("- " + line.Content)
	default:
		return "  " + line.Content
	}
}

// ParseDiff parses a unified diff string into DiffFile structures.
func ParseDiff(diff string) []DiffFile {
	var files []DiffFile
	var currentFile *DiffFile

	for _, line := range strings.Split(diff, "\n") {
		if strings.HasPrefix(line, "diff --git") {
			if currentFile != nil {
				files = append(files, *currentFile)
			}
			parts := strings.SplitN(line, " b/", 2)
			path := ""
			if len(parts) > 1 {
				path = parts[1]
			}
			currentFile = &DiffFile{Path: path}
		} else if strings.HasPrefix(line, "@@") {
			if currentFile != nil {
				var hunk DiffHunk
				fmt.Sscanf(line, "@@ -%d", &hunk.StartLine)
				currentFile.Hunks = append(currentFile.Hunks, hunk)
			}
		} else if currentFile != nil && len(currentFile.Hunks) > 0 {
			lastHunk := &currentFile.Hunks[len(currentFile.Hunks)-1]
			if strings.HasPrefix(line, "+") {
				lastHunk.Lines = append(lastHunk.Lines, DiffLine{Type: "+", Content: line[1:]})
				currentFile.Added++
			} else if strings.HasPrefix(line, "-") {
				lastHunk.Lines = append(lastHunk.Lines, DiffLine{Type: "-", Content: line[1:]})
				currentFile.Removed++
			} else if strings.HasPrefix(line, " ") {
				lastHunk.Lines = append(lastHunk.Lines, DiffLine{Type: " ", Content: line[1:]})
			}
		}
	}

	if currentFile != nil {
		files = append(files, *currentFile)
	}

	return files
}
