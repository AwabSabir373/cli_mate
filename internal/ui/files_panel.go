package ui

import (
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

	// Find the position of this entry
	idx := fb.selection

	// Check if already expanded (next entry is a child)
	alreadyExpanded := false
	if idx+1 < len(fb.files) {
		next := fb.files[idx+1]
		// Check if next entry is a child (higher indent and path starts with current path)
		if next.Indent > selected.Indent && strings.HasPrefix(next.Path, selected.Path) {
			alreadyExpanded = true
		}
	}

	if alreadyExpanded {
		// Collapse: remove all entries with higher indent
		fb.collapseAt(idx)
	} else {
		// Expand: add children
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

	// Insert after the parent
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
		// Don't auto-expand subdirectories; let the user expand them
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

	return strings.Join(lines, "\n")
}
