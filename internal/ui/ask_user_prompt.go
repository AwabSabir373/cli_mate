package ui

import (
	"fmt"
	"strings"
)

// askUserChoice represents a single option in an ask-user prompt.
type askUserChoice struct {
	Label       string
	Description string
}

// askUserQuestion represents a single question in the prompt.
type askUserQuestion struct {
	Question    string
	Header      string
	Options     []askUserChoice
	MultiSelect bool
}

// askUserState manages the interactive ask-user prompt state.
type askUserState struct {
	questions    []askUserQuestion
	cursor       int      // Which question we're on
	answerCursor int      // Which option is selected within the question
	answers      []string // Committed answers (option labels or typed text)
	typing       bool     // Whether user is typing a custom answer
	typedText    string   // Custom typed answer text
	active       bool
	finished     bool
}

// newAskUserState creates a new ask-user prompt state.
func newAskUserState(questions []askUserQuestion) *askUserState {
	state := &askUserState{
		questions:    questions,
		answers:      make([]string, len(questions)),
		answerCursor: 0,
		cursor:       0,
		active:       true,
	}

	// Start with custom input for multi-select questions
	if len(questions) > 0 && questions[0].MultiSelect {
		state.typing = true
	}

	return state
}

// handleKey processes a keypress for the ask-user prompt.
func (as *askUserState) handleKey(key string) (string, bool) {
	if !as.active {
		return "", true
	}

	if as.cursor >= len(as.questions) {
		// On the confirm tab
		if key == "enter" || key == "y" || key == "Y" {
			as.active = false
			as.finished = true
			return strings.Join(as.answers, "\n"), true
		}
		if key == "n" || key == "N" || key == "esc" {
			as.active = false
			return "", true
		}
		return "", false
	}

	q := as.questions[as.cursor]

	switch key {
	case "up", "shift+tab":
		if as.cursor > 0 {
			as.cursor--
			as.answerCursor = 0
			as.typing = false
		}
		return "", false
	case "down", "tab":
		if as.cursor < len(as.questions) {
			as.cursor++
			as.answerCursor = 0
			as.typing = false
		}
		return "", false
	case "enter":
		if as.typing && as.typedText != "" {
			as.answers[as.cursor] = as.typedText
			as.typedText = ""
			as.cursor++
			as.answerCursor = 0
			as.typing = false
			return "", false
		}
		if len(q.Options) > 0 {
			if as.answerCursor < len(q.Options) {
				if q.MultiSelect {
					// Toggle selection
					selected := q.Options[as.answerCursor].Label
					if as.answers[as.cursor] == selected {
						as.answers[as.cursor] = ""
					} else {
						as.answers[as.cursor] = selected
					}
				} else {
					as.answers[as.cursor] = q.Options[as.answerCursor].Label
					as.cursor++
					as.answerCursor = 0
				}
			} else {
				// "Type your own answer" option
				as.typing = true
				as.typedText = ""
			}
		}
		return "", false
	case "esc":
		as.active = false
		return "", true
	default:
		if as.typing && len(key) == 1 {
			as.typedText += key
		}
	}
	return "", false
}

// render renders the ask-user prompt UI.
func (as *askUserState) render(styles appStyles, _ int) string {
	if !as.active {
		return ""
	}

	var b strings.Builder
	b.WriteString(styles.pill.Render("❓ Question"))
	b.WriteString("\n\n")

	for i, q := range as.questions {
		// Question header
		prefix := " "
		if i == as.cursor {
			prefix = "▸"
		}
		b.WriteString(styles.accent.Render(fmt.Sprintf("%s %s: %s", prefix, q.Header, q.Question)))
		b.WriteString("\n")

		if i != as.cursor {
			// Show committed answer for other questions
			if as.answers[i] != "" {
				b.WriteString(fmt.Sprintf("    Answer: %s\n", as.answers[i]))
			} else {
				b.WriteString(styles.muted.Render("    (no answer yet)\n"))
			}
			b.WriteString("\n")
			continue
		}

		// Show options for active question
		if as.typing {
			b.WriteString("    Type your answer: ")
			if as.typedText == "" {
				b.WriteString(styles.cursor.Render(" "))
			} else {
				b.WriteString(as.typedText)
				b.WriteString(styles.cursor.Render(" "))
			}
			b.WriteString("\n")
			b.WriteString(styles.muted.Render("    Press Enter to submit\n"))
		} else {
			for j, opt := range q.Options {
				optPrefix := "  "
				if j == as.answerCursor {
					optPrefix = "▸ "
				}
				mark := " "
				if as.answers[i] == opt.Label {
					mark = "✓"
				}
				if j == as.answerCursor {
					b.WriteString(styles.selected.Render(fmt.Sprintf("%s%s %s", optPrefix, mark, opt.Label)))
				} else {
					b.WriteString(fmt.Sprintf("  %s %s", mark, opt.Label))
				}
				if opt.Description != "" {
					b.WriteString(fmt.Sprintf(" — %s", opt.Description))
				}
				b.WriteString("\n")
			}
			// "Type your own answer" option
			typePrefix := "  "
			if as.answerCursor == len(q.Options) {
				typePrefix = "▸ "
			}
			if as.answerCursor == len(q.Options) {
				b.WriteString(styles.selected.Render(typePrefix + "✏ Type your own answer"))
			} else {
				b.WriteString("  ✏ Type your own answer")
			}
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	// Confirm tab
	if as.cursor >= len(as.questions) {
		b.WriteString(styles.selected.Render("  ✓ Confirm answers (Enter)"))
		b.WriteString("\n")
	} else {
		b.WriteString("    Confirm answers")
		b.WriteString("\n")
	}

	b.WriteString(styles.muted.Render("  ↑/↓ navigate · Tab next · Enter select · Esc cancel"))
	return b.String()
}
