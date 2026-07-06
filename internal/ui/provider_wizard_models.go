package ui

import (
	"fmt"
	"strings"
)

func formatContextWindow(ctx int) string {
	if ctx <= 0 {
		return ""
	}
	if ctx >= 1000000 {
		return fmt.Sprintf("%.0fM", float64(ctx)/1000000)
	}
	if ctx >= 1000 {
		return fmt.Sprintf("%.0fK", float64(ctx)/1000)
	}
	return fmt.Sprint(ctx)
}

func providerWizardModelMeta(contextWindow int, toolCall bool, reasoning bool, inputCost float64, outputCost float64, tags []string) string {
	parts := []string{}
	if ctx := formatContextWindow(contextWindow); ctx != "" {
		parts = append(parts, ctx+" ctx")
	}
	if toolCall {
		parts = append(parts, "tools")
	}
	if reasoning {
		parts = append(parts, "reasoning")
	}
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag != "" {
			parts = append(parts, tag)
		}
		if len(parts) >= 5 {
			break
		}
	}
	if inputCost > 0 || outputCost > 0 {
		parts = append(parts, fmt.Sprintf("$%g/%g", inputCost, outputCost))
	}
	return strings.Join(parts, " · ")
}
