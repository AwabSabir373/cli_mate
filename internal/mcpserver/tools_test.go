package mcpserver

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func inWorkspace(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	workspace := filepath.Join(root, "project")
	if err := os.Mkdir(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(workspace); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(previous) })
	return root
}

func TestResolveWorkspacePathRejectsSiblingPrefix(t *testing.T) {
	root := inWorkspace(t)
	sibling := filepath.Join(root, "project-evil")
	if err := os.Mkdir(sibling, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := resolveWorkspacePath(sibling); err == nil {
		t.Fatal("expected sibling path with matching prefix to be rejected")
	}
}

func TestSearchCodeHonorsCompactOutputBudget(t *testing.T) {
	inWorkspace(t)
	content := strings.Repeat("needle "+strings.Repeat("x", 100)+"\n", 30)
	if err := os.WriteFile("sample.go", []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := handleSearchCode(context.Background(), map[string]any{
		"query":       "needle",
		"max_results": 3,
		"max_chars":   300,
	})
	if err != nil {
		t.Fatal(err)
	}
	text := result.(string)
	if len(text) > 360 { // allows the short truncation hint
		t.Fatalf("compact search exceeded budget: %d bytes\n%s", len(text), text)
	}
	if strings.Count(text, "sample.go:") != 2 {
		t.Fatalf("expected budgeted exact anchors, got:\n%s", text)
	}
}

func TestReadFileReturnsExactLineWindow(t *testing.T) {
	inWorkspace(t)
	if err := os.WriteFile("sample.go", []byte("one\ntwo\nthree\nfour\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := handleReadFile(context.Background(), map[string]any{
		"path":       "sample.go",
		"line_start": 2,
		"line_end":   3,
	})
	if err != nil {
		t.Fatal(err)
	}
	text := result.(string)
	if !strings.Contains(text, "2|two\n3|three") || strings.Contains(text, "1|one") || strings.Contains(text, "4|four") {
		t.Fatalf("unexpected line window:\n%s", text)
	}
}
