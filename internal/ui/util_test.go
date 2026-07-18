package ui

import "testing"

func TestTruncateStringHandlesUnicodeAndInvalidWidths(t *testing.T) {
	if got := truncateString("🔧 tools", 4); got != "🔧..." {
		t.Fatalf("unexpected Unicode truncation: %q", got)
	}
	if got := truncateString("value", -3); got != "" {
		t.Fatalf("negative width should produce an empty string, got %q", got)
	}
}
