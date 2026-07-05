package tools

import (
	"context"
	"strings"
	"testing"
)

func TestApplyPatchReturnsErrorWhenNoHunksApply(t *testing.T) {
	tool := NewApplyPatchTool(t.TempDir())
	result, err := tool.Execute(context.Background(), Call{
		Name: "apply_patch",
		Argument: map[string]any{
			"patch": "--- a/missing.txt\n+++ b/missing.txt\n@@ -1,1 +1,1 @@\n-old\n+new\n",
		},
	})

	if err == nil {
		t.Fatal("expected error when patch applies no hunks")
	}
	if result.Error == "" {
		t.Fatal("expected result error when patch applies no hunks")
	}
	if !strings.Contains(result.Content, "SKIP missing.txt") {
		t.Fatalf("expected skip detail in result content, got %q", result.Content)
	}
}
