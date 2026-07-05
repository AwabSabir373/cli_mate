package ui

import "testing"

func TestStreamFilterHidesToolDraftSplitAcrossTokens(t *testing.T) {
	var filter streamFilter

	first := filter.Push("checking`")
	if first.Visible != "checking" {
		t.Fatalf("expected visible text before possible fence, got %q", first.Visible)
	}

	second := filter.Push("``cli_mate-tool\n{\"tool\":\"file_read\",\"arguments\":{\"path\":\"src/components/ui/card.tsx\"}}")
	if second.Visible != "" {
		t.Fatalf("expected tool draft to be hidden, got visible %q", second.Visible)
	}
	if !second.ToolStarted {
		t.Fatal("expected tool draft start")
	}
	if got := streamedToolName(second.ToolDraft); got != "file_read" {
		t.Fatalf("expected file_read, got %q", got)
	}
	if got := streamingFilePath(second.ToolDraft); got != "src/components/ui/card.tsx" {
		t.Fatalf("expected streamed path, got %q", got)
	}
}

func TestStreamFilterFlushesNormalTrailingText(t *testing.T) {
	var filter streamFilter

	got := filter.Push("hello `")
	if got.Visible != "hello " {
		t.Fatalf("expected visible prefix, got %q", got.Visible)
	}
	if tail := filter.Flush(); tail != "`" {
		t.Fatalf("expected trailing text on flush, got %q", tail)
	}
}
