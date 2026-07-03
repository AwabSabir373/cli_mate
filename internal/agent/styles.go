package agent

// ResponseStyle controls how the agent formats its responses.
type ResponseStyle string

const (
	StyleConcise     ResponseStyle = "concise"
	StyleExplanatory ResponseStyle = "explanatory"
	StyleReview      ResponseStyle = "review"
)

// stylePrompts contains the system prompt additions for each style.
var stylePrompts = map[ResponseStyle]string{
	StyleConcise: `## Response Style: Concise
- Give short, direct answers
- Skip explanations unless explicitly asked
- Use bullet points over paragraphs
- Omit pleasantries and filler`,

	StyleExplanatory: `## Response Style: Explanatory
- Explain your reasoning and approach
- Provide context for your decisions
- Include relevant background information
- Be thorough but organized`,

	StyleReview: `## Response Style: Review
- Focus on code quality and correctness
- Identify potential bugs and edge cases
- Suggest improvements with rationale
- Reference specific lines and patterns`,
}

// StylePrompt returns the system prompt addition for the given style.
func StylePrompt(style ResponseStyle) string {
	if prompt, ok := stylePrompts[style]; ok {
		return prompt
	}
	return ""
}

// ValidStyles returns all valid response styles.
func ValidStyles() []ResponseStyle {
	return []ResponseStyle{StyleConcise, StyleExplanatory, StyleReview}
}

// IsValidStyle reports whether the given string is a valid response style.
func IsValidStyle(s string) bool {
	for _, valid := range ValidStyles() {
		if string(valid) == s {
			return true
		}
	}
	return false
}
