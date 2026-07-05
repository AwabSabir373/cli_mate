package ui

import "strings"

var toolFencePrefixes = []string{
	"```cli_mate-tool",
	"```goai-tool",
	"```tool",
	"```json",
}

type streamFilter struct {
	pending string
	hidden  strings.Builder
	hiding  bool
}

type streamFilterResult struct {
	Visible     string
	ToolDraft   string
	ToolStarted bool
}

func (f *streamFilter) Push(token string) streamFilterResult {
	if token == "" {
		return streamFilterResult{}
	}
	if f.hiding {
		f.hidden.WriteString(token)
		return streamFilterResult{ToolDraft: f.hidden.String()}
	}

	input := f.pending + token
	f.pending = ""
	if idx := firstToolFenceIndex(input); idx >= 0 {
		f.hiding = true
		visible := input[:idx]
		f.hidden.WriteString(input[idx:])
		return streamFilterResult{
			Visible:     visible,
			ToolDraft:   f.hidden.String(),
			ToolStarted: true,
		}
	}

	keep := toolFencePrefixSuffixLen(input)
	if keep > 0 {
		f.pending = input[len(input)-keep:]
		input = input[:len(input)-keep]
	}
	return streamFilterResult{Visible: input}
}

func (f *streamFilter) Flush() string {
	if f.hiding {
		return ""
	}
	out := f.pending
	f.pending = ""
	return out
}

func (f *streamFilter) ToolDraft() string {
	if !f.hiding {
		return ""
	}
	return f.hidden.String()
}

func firstToolFenceIndex(text string) int {
	best := -1
	for _, prefix := range toolFencePrefixes {
		idx := strings.Index(text, prefix)
		if idx >= 0 && (best < 0 || idx < best) {
			best = idx
		}
	}
	return best
}

func toolFencePrefixSuffixLen(text string) int {
	maxKeep := 0
	for _, prefix := range toolFencePrefixes {
		limit := len(prefix) - 1
		if limit > len(text) {
			limit = len(text)
		}
		for n := limit; n > 0; n-- {
			if strings.HasSuffix(text, prefix[:n]) {
				if n > maxKeep {
					maxKeep = n
				}
				break
			}
		}
	}
	return maxKeep
}

func streamedToolName(draft string) string {
	if name := decodeStreamingJSONString([]byte(draft), "tool"); name != "" {
		return name
	}
	if name := decodeStreamingJSONString([]byte(draft), "name"); name != "" {
		return name
	}
	return "tool"
}
