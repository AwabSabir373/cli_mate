package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"cli_mate/internal/providers"
)

// ExportFormat represents the export format.
type ExportFormat string

const (
	ExportMarkdown ExportFormat = "markdown"
	ExportJSON     ExportFormat = "json"
)

// ExportSession exports a conversation to the specified format.
func ExportSession(messages []providers.Message, format ExportFormat, outputPath string) error {
	var content string

	switch format {
	case ExportMarkdown:
		content = exportMarkdown(messages)
	case ExportJSON:
		content = exportJSON(messages)
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}

	return os.WriteFile(outputPath, []byte(content), 0644)
}

func exportMarkdown(messages []providers.Message) string {
	var b strings.Builder
	b.WriteString("# cli_mate Session Export\n\n")
	b.WriteString(fmt.Sprintf("Exported at: %s\n\n", time.Now().Format("2006-01-02 15:04:05")))
	b.WriteString("---\n\n")

	for _, msg := range messages {
		switch msg.Role {
		case "system":
			// Skip system messages in export
		case "user":
			b.WriteString("## User\n\n")
			b.WriteString(msg.Content)
			b.WriteString("\n\n")
		case "assistant":
			b.WriteString("## Assistant\n\n")
			b.WriteString(msg.Content)
			if len(msg.ToolCalls) > 0 {
				b.WriteString("\n\n### Tool Calls\n\n")
				for _, tc := range msg.ToolCalls {
					b.WriteString(fmt.Sprintf("- **%s**: %s\n", tc.Name, tc.Arguments))
				}
			}
			b.WriteString("\n\n")
		case "tool":
			b.WriteString(fmt.Sprintf("### Tool Result: %s\n\n", msg.Name))
			b.WriteString(msg.Content)
			b.WriteString("\n\n")
		}
	}

	return b.String()
}

func exportJSON(messages []providers.Message) string {
	type ExportMessage struct {
		Role      string              `json:"role"`
		Content   string              `json:"content"`
		ToolCalls []providers.ToolCall `json:"toolCalls,omitempty"`
		Name      string              `json:"name,omitempty"`
		Timestamp time.Time           `json:"timestamp"`
	}

	export := make([]ExportMessage, len(messages))
	for i, msg := range messages {
		export[i] = ExportMessage{
			Role:      msg.Role,
			Content:   msg.Content,
			ToolCalls: msg.ToolCalls,
			Name:      msg.Name,
			Timestamp: time.Now(),
		}
	}

	data, err := json.MarshalIndent(export, "", "  ")
	if err != nil {
		return "[]"
	}
	return string(data)
}

// ExportToFile exports messages to a file with the appropriate extension.
func ExportToFile(messages []providers.Message, format ExportFormat, dir string) (string, error) {
	ext := ".md"
	if format == ExportJSON {
		ext = ".json"
	}

	filename := fmt.Sprintf("cli_mate_export_%s%s", time.Now().Format("20060102_150405"), ext)
	path := strings.Join([]string{dir, filename}, string(os.PathSeparator))

	err := ExportSession(messages, format, path)
	return path, err
}
