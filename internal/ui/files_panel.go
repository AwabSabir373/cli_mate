package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FileBrowser displays a file tree for navigating the workspace.
type FileBrowser struct {
	visible    bool
	root       string
	files      []FileEntry
	selection  int
	scrollOff  int
}

// FileEntry represents a file or directory in the browser.
type FileEntry struct {
	Name     string
	Path     string
	IsDir    bool
	Indent   int
	GitStatus string // "modified", "added", "deleted", "untracked", ""
}

// NewFileBrowser creates a new file browser.
func NewFileBrowser(root string) *FileBrowser {
	return &FileBrowser{
		visible: false,
		root:    root,
	}
}

// Toggle shows/hides the file browser.
func (fb *FileBrowser) Toggle() {
	fb.visible = !fb.visible
	if fb.visible && len(fb.files) == 0 {
		fb.Refresh()
	}
}

// SetVisible sets the visibility of the file browser.
func (fb *FileBrowser) SetVisible(visible bool) {
	fb.visible = visible
	if visible && len(fb.files) == 0 {
		fb.Refresh()
	}
}

// IsVisible returns whether the file browser is visible.
func (fb *FileBrowser) IsVisible() bool {
	return fb.visible
}

// Refresh reloads the file list.
func (fb *FileBrowser) Refresh() {
	fb.files = nil
	fb.scanDir(fb.root, 0)
	fb.selection = 0
	fb.scrollOff = 0
}

// MoveUp moves the selection up.
func (fb *FileBrowser) MoveUp() {
	if fb.selection > 0 {
		fb.selection--
	}
	fb.adjustScroll()
}

// MoveDown moves the selection down.
func (fb *FileBrowser) MoveDown() {
	if fb.selection < len(fb.files)-1 {
		fb.selection++
	}
	fb.adjustScroll()
}

// GetSelected returns the currently selected file entry.
func (fb *FileBrowser) GetSelected() *FileEntry {
	if fb.selection >= 0 && fb.selection < len(fb.files) {
		return &fb.files[fb.selection]
	}
	return nil
}

// Expand toggles expansion of the selected directory.
func (fb *FileBrowser) Expand() {
	selected := fb.GetSelected()
	if selected == nil || !selected.IsDir {
		return
	}

	idx := fb.selection
	alreadyExpanded := false
	if idx+1 < len(fb.files) {
		next := fb.files[idx+1]
		if next.Indent > selected.Indent && strings.HasPrefix(next.Path, selected.Path) {
			alreadyExpanded = true
		}
	}

	if alreadyExpanded {
		fb.collapseAt(idx)
	} else {
		fb.expandAt(idx)
	}
}

func (fb *FileBrowser) collapseAt(idx int) {
	parentIndent := fb.files[idx].Indent
	end := idx + 1
	for end < len(fb.files) && fb.files[end].Indent > parentIndent {
		end++
	}
	fb.files = append(fb.files[:idx+1], fb.files[end:]...)
}

func (fb *FileBrowser) expandAt(idx int) {
	entry := fb.files[idx]
	entries, _ := os.ReadDir(entry.Path)
	var newFiles []FileEntry
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		if name == "node_modules" || name == "vendor" || name == ".git" {
			continue
		}
		path := filepath.Join(entry.Path, name)
		newFiles = append(newFiles, FileEntry{
			Name:   name,
			Path:   path,
			IsDir:  e.IsDir(),
			Indent: entry.Indent + 1,
		})
	}

	fb.files = append(fb.files[:idx+1], append(newFiles, fb.files[idx+1:]...)...)
}

func (fb *FileBrowser) scanDir(dir string, indent int) {
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		if name == "node_modules" || name == "vendor" || name == ".git" {
			continue
		}
		path := filepath.Join(dir, name)
		fb.files = append(fb.files, FileEntry{
			Name:   name,
			Path:   path,
			IsDir:  e.IsDir(),
			Indent: indent,
		})
	}
}

func (fb *FileBrowser) adjustScroll() {
	maxVisible := 20
	if fb.selection < fb.scrollOff {
		fb.scrollOff = fb.selection
	}
	if fb.selection >= fb.scrollOff+maxVisible {
		fb.scrollOff = fb.selection - maxVisible + 1
	}
}

// Render produces the file browser view.
func (fb *FileBrowser) Render(width int, styles appStyles) string {
	if !fb.visible {
		return ""
	}

	var lines []string
	lines = append(lines, styles.pill.Render("Files"))
	lines = append(lines, "")

	maxVisible := 20
	start := fb.scrollOff
	end := start + maxVisible
	if end > len(fb.files) {
		end = len(fb.files)
	}

	for i := start; i < end; i++ {
		entry := fb.files[i]
		prefix := strings.Repeat("  ", entry.Indent)
		icon := "  "
		if entry.IsDir {
			icon = "/ "
		}
		if i == fb.selection {
			lines = append(lines, styles.selected.Render(prefix+icon+entry.Name))
		} else {
			lines = append(lines, prefix+icon+entry.Name)
		}
	}

	if len(fb.files) > maxVisible {
		lines = append(lines, "")
		lines = append(lines, styles.muted.Render("↑/↓ to scroll"))
	}

	return strings.Join(lines, "\\n")
}

// --- Touched Files for Sidebar ---

// touchedFile represents a file that was modified during the session.
type touchedFile struct {
	path    string
	added   int
	deleted int
	edits   int
	created bool
}

// touchedFiles extracts modified files from transcript log entries.
func touchedFilesFromLog(log []logEntry) []touchedFile {
	fileMap := make(map[string]*touchedFile)
	var order []string

	for _, entry := range log {
		if entry.Kind != "tool" {
			continue
		}
		// Parse tool entries for file paths and diff stats
		text := entry.Text
		var path string
		var added, deleted int

		// Simple heuristic: find paths and counts
		if strings.Contains(text, "write_file") || strings.Contains(text, "edit_file") || strings.Contains(text, "apply_patch") {
			parts := strings.Fields(text)
			for i, p := range parts {
				if p == "path:" || p == "path=" {
					if i+1 < len(parts) {
						path = strings.Trim(parts[i+1], "\"',")
					}
				}
			}
		}

		if path == "" {
			continue
		}

		relPath := path
		if existing, ok := fileMap[relPath]; ok {
			existing.edits++
			existing.added += added
			existing.deleted += deleted
		} else {
			fileMap[relPath] = &touchedFile{
				path:  relPath,
				added: added,
				edits: 1,
			}
			order = append(order, relPath)
		}
	}

	// Convert to slice, newest first
	result := make([]touchedFile, 0, len(order))
	for i := len(order) - 1; i >= 0; i-- {
		if tf, ok := fileMap[order[i]]; ok {
			result = append(result, *tf)
		}
	}

	return result
}

// renderTouchedFiles renders the touched files section for the sidebar.
func renderTouchedFiles(files []touchedFile, maxFiles int, styles appStyles) string {
	if len(files) == 0 {
		return ""
	}

	var lines []string
	lines = append(lines, styles.sidebarTitle.Render("Files"))
	lines = append(lines, "")

	shown := files
	if len(shown) > maxFiles {
		shown = shown[:maxFiles]
	}

	for _, f := range shown {
		diffStr := ""
		if f.added > 0 || f.deleted > 0 {
			diffStr = fmt.Sprintf(" %s+%d%s-%d",
				styles.badgeAdd.Render(""),
				f.added,
				styles.badgeDel.Render(""),
				f.deleted,
			)
		}
		label := filepath.Base(f.path)
		if f.created {
			label = "+ " + label
		}
		lines = append(lines, fmt.Sprintf("  %s%s", label, styles.muted.Render(diffStr)))
	}

	if len(files) > maxFiles {
		lines = append(lines, styles.muted.Render(fmt.Sprintf("  ... +%d more", len(files)-maxFiles)))
	}

	return strings.Join(lines, "\n")
}
