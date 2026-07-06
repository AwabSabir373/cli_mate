package ui

import (
	"image/color"
	"os"
	"strconv"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
)

const (
	streamingFadeSteps    = 12
	streamingFadeDuration = 1200 * time.Millisecond
)

// streamingFadeState tracks the age of streaming text tokens for fade effects.
type streamingFadeState struct {
	tokens     []fadeToken
	enabled    bool
	stepStyles []lipgloss.Style
}

// fadeToken represents a chunk of streaming text with its timestamp.
type fadeToken struct {
	text      string
	timestamp time.Time
}

// newStreamingFade creates a new streaming fade state.
func newStreamingFade(accent color.Color, muted color.Color) *streamingFadeState {
	enabled := !reducedMotionEnabled()

	sf := &streamingFadeState{
		enabled:    enabled,
		stepStyles: make([]lipgloss.Style, streamingFadeSteps),
	}

	// Pre-compute step styles using style modification
	if enabled {
		for i := 0; i < streamingFadeSteps; i++ {
			col := accent
			if i > streamingFadeSteps/2 {
				col = muted
			}
			sf.stepStyles[i] = lipgloss.NewStyle().Foreground(col)
		}
	}

	return sf
}

// reducedMotionEnabled checks if reduced motion is preferred.
func reducedMotionEnabled() bool {
	val := os.Getenv("CLI_MATE_REDUCED_MOTION")
	if val != "" {
		disabled, err := strconv.ParseBool(val)
		if err == nil && disabled {
			return true
		}
	}
	return !isTerminal()
}

// addToken adds a new streaming text token.
func (sf *streamingFadeState) addToken(text string) {
	if !sf.enabled {
		return
	}
	sf.tokens = append(sf.tokens, fadeToken{
		text:      text,
		timestamp: time.Now(),
	})
}

// render applies the fade effect to all tokens and returns the styled string.
func (sf *streamingFadeState) render() string {
	if !sf.enabled || len(sf.tokens) == 0 {
		var b strings.Builder
		for _, t := range sf.tokens {
			b.WriteString(t.text)
		}
		return b.String()
	}

	now := time.Now()
	var b strings.Builder
	for _, token := range sf.tokens {
		age := now.Sub(token.timestamp)
		step := int(age.Nanoseconds() * int64(streamingFadeSteps) / streamingFadeDuration.Nanoseconds())
		if step >= streamingFadeSteps {
			step = streamingFadeSteps - 1
		}
		b.WriteString(sf.stylesForStep(step).Render(token.text))
	}
	return b.String()
}

// stylesForStep returns the style for a given fade step.
func (sf *streamingFadeState) stylesForStep(step int) lipgloss.Style {
	if step < 0 {
		step = 0
	}
	if step >= len(sf.stepStyles) {
		step = len(sf.stepStyles) - 1
	}
	return sf.stepStyles[step]
}

// clear removes all tokens.
func (sf *streamingFadeState) clear() {
	sf.tokens = nil
}

// isTerminal checks if stdout is a terminal.
func isTerminal() bool {
	info, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (info.Mode() & os.ModeCharDevice) != 0
}
