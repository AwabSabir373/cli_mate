package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// imageAttachment represents an image attached to a message.
type imageAttachment struct {
	path     string // file path
	filename string // display name
	size     int64  // file size in bytes
	width    int    // image width (if known)
	height   int    // image height (if known)
	mimeType string // MIME type (image/png, image/jpeg, etc.)
	data     []byte // raw image data for sending to provider
}

// imageAttachState manages image attachments for multimodal models.
type imageAttachState struct {
	visible     bool
	attachments []imageAttachment
	cursor      int
	scrollOff   int
	err         string
	maxAttach   int // maximum number of attachments allowed
	maxSizeMB   int64 // maximum file size per attachment in MB
}

// newImageAttachState creates a new image attachment state.
func newImageAttachState() *imageAttachState {
	return &imageAttachState{
		maxAttach: 10,
		maxSizeMB: 20, // 20 MB per image
	}
}

// show opens the image attachment panel.
func (ias *imageAttachState) show() {
	ias.visible = true
	ias.cursor = 0
	ias.scrollOff = 0
	ias.err = ""
}

// hide closes the image attachment panel.
func (ias *imageAttachState) hide() {
	ias.visible = false
	ias.err = ""
}

// isVisible returns true if the panel is visible.
func (ias *imageAttachState) isVisible() bool {
	return ias.visible
}

// hasAttachments returns true if there are any attachments.
func (ias *imageAttachState) hasAttachments() bool {
	return len(ias.attachments) > 0
}

// attachmentCount returns the number of attachments.
func (ias *imageAttachState) attachmentCount() int {
	return len(ias.attachments)
}

// addAttachment adds an image from a file path.
func (ias *imageAttachState) addAttachment(path string) error {
	if len(ias.attachments) >= ias.maxAttach {
		return fmt.Errorf("maximum %d attachments allowed", ias.maxAttach)
	}

	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("cannot read file: %w", err)
	}

	maxBytes := ias.maxSizeMB * 1024 * 1024
	if info.Size() > maxBytes {
		return fmt.Errorf("file too large: %d MB (max %d MB)", info.Size()/(1024*1024), ias.maxSizeMB)
	}

	mimeType := detectImageMime(path)
	if mimeType == "" {
		return fmt.Errorf("unsupported image format: %s", filepath.Ext(path))
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("cannot read image: %w", err)
	}

	ias.attachments = append(ias.attachments, imageAttachment{
		path:     path,
		filename: filepath.Base(path),
		size:     info.Size(),
		mimeType: mimeType,
		data:     data,
	})

	return nil
}

// removeAttachment removes an attachment at the given index.
func (ias *imageAttachState) removeAttachment(index int) {
	if index < 0 || index >= len(ias.attachments) {
		return
	}
	ias.attachments = append(ias.attachments[:index], ias.attachments[index+1:]...)
	if ias.cursor >= len(ias.attachments) {
		ias.cursor = len(ias.attachments) - 1
	}
}

// clearAttachments removes all attachments.
func (ias *imageAttachState) clearAttachments() {
	ias.attachments = nil
	ias.cursor = 0
}

// attachmentSummary returns a human-readable summary of attached images.
func (ias *imageAttachState) attachmentSummary() string {
	if len(ias.attachments) == 0 {
		return ""
	}

	var parts []string
	for _, a := range ias.attachments {
		sizeStr := formatFileSize(a.size)
		parts = append(parts, fmt.Sprintf("%s (%s)", a.filename, sizeStr))
	}
	return strings.Join(parts, ", ")
}

// formatFileSize formats bytes as human-readable.
func formatFileSize(bytes int64) string {
	switch {
	case bytes >= 1024*1024:
		return fmt.Sprintf("%.1f MB", float64(bytes)/(1024*1024))
	case bytes >= 1024:
		return fmt.Sprintf("%.1f KB", float64(bytes)/1024)
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

// detectImageMime detects the MIME type from the file extension.
func detectImageMime(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".bmp":
		return "image/bmp"
	default:
		return ""
	}
}

// handleKey processes key events for the image attachment panel.
func (ias *imageAttachState) handleKey(key string) string {
	if !ias.visible {
		return ""
	}

	switch key {
	case "up", "shift+tab":
		if ias.cursor > 0 {
			ias.cursor--
		}
		ias.adjustScroll()
	case "down", "tab":
		totalItems := len(ias.attachments) + 1 // +1 for "Add Image" button
		if ias.cursor < totalItems-1 {
			ias.cursor++
		}
		ias.adjustScroll()
	case "enter", " ":
		if ias.cursor >= 0 && ias.cursor < len(ias.attachments) {
			// Toggle selection (for now, just show details)
			return "select"
		}
		if ias.cursor == len(ias.attachments) {
			// "Add Image" button
			return "add"
		}
	case "delete", "backspace":
		if ias.cursor >= 0 && ias.cursor < len(ias.attachments) {
			ias.removeAttachment(ias.cursor)
			return "removed"
		}
	case "esc":
		ias.hide()
		return "close"
	}

	return ""
}

func (ias *imageAttachState) adjustScroll() {
	maxVisible := 10
	if ias.cursor < ias.scrollOff {
		ias.scrollOff = ias.cursor
	}
	if ias.cursor >= ias.scrollOff+maxVisible {
		ias.scrollOff = ias.cursor - maxVisible + 1
	}
}

// renderImageAttachments renders the image attachment panel.
func renderImageAttachments(ias *imageAttachState, styles appStyles, _ int) string {
	if !ias.visible {
		return ""
	}

	var b strings.Builder
	b.WriteString(styles.pill.Render(" Image Attachments "))
	b.WriteString("\n\n")

	if len(ias.attachments) == 0 {
		b.WriteString(styles.muted.Render("  No images attached."))
		b.WriteString("\n\n")
		b.WriteString(styles.muted.Render("  Use /attach <path> to add an image"))
		b.WriteString("\n")
		b.WriteString(styles.muted.Render("  Supported: PNG, JPEG, GIF, WebP"))
		b.WriteString("\n\n")
		b.WriteString(styles.muted.Render("  Press Esc to close"))
		b.WriteString("\n")
		return b.String()
	}

	maxVisible := 10
	start := ias.scrollOff
	end := start + maxVisible
	if end > len(ias.attachments) {
		end = len(ias.attachments)
	}

	for i := start; i < end; i++ {
		attach := ias.attachments[i]
		sizeStr := formatFileSize(attach.size)
		label := fmt.Sprintf("%s  %s  (%s)", attach.mimeType, attach.filename, sizeStr)

		if i == ias.cursor {
			b.WriteString(styles.selected.Render(fmt.Sprintf(" ▸ %s", label)))
			b.WriteString("\n")
			// Show path on next line
			b.WriteString(styles.muted.Render(fmt.Sprintf("    %s", attach.path)))
			b.WriteString("\n")
		} else {
			b.WriteString(fmt.Sprintf("   %s", label))
			b.WriteString("\n")
		}
	}

	// "Add Image" button
	addLabel := "+ Add Image"
	if ias.cursor == len(ias.attachments) {
		b.WriteString(styles.selected.Render(fmt.Sprintf(" ▸ %s", addLabel)))
		b.WriteString("\n")
	} else {
		b.WriteString(fmt.Sprintf("   %s", addLabel))
		b.WriteString("\n")
	}

	// Summary
	b.WriteString("\n")
	b.WriteString(styles.muted.Render(fmt.Sprintf("  Total: %d image(s)", len(ias.attachments))))
	b.WriteString("\n\n")

	b.WriteString(styles.muted.Render("  ↑/↓ navigate · Enter select · Del remove · Esc close"))
	b.WriteString("\n")
	return b.String()
}
