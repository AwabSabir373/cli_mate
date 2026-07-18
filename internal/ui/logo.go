package ui

import "strings"

// renderLogo renders the large CLI MATE wordmark used on the welcome screen.
func renderLogo(styles appStyles, width int) string {
	return renderCleanLogo(styles, width)
}

// renderCleanLogo renders a terminal-safe block-letter CLI MATE logo.
func renderCleanLogo(styles appStyles, _ int) string {
	wordmark := []string{
		" ██████╗██╗     ██╗    ███╗   ███╗ █████╗ ████████╗███████╗",
		"██╔════╝██║     ██║    ████╗ ████║██╔══██╗╚══██╔══╝██╔════╝",
		"██║     ██║     ██║    ██╔████╔██║███████║   ██║   █████╗  ",
		"██║     ██║     ██║    ██║╚██╔╝██║██╔══██║   ██║   ██╔══╝  ",
		"╚██████╗███████╗██║    ██║ ╚═╝ ██║██║  ██║   ██║   ███████╗",
		" ╚═════╝╚══════╝╚═╝    ╚═╝     ╚═╝╚═╝  ╚═╝   ╚═╝   ╚══════╝",
	}
	for i := range wordmark {
		wordmark[i] = styles.accent.Render(wordmark[i])
	}
	return strings.Join(wordmark, "\n")
}

// renderLogoSmall renders a compact version of the logo for narrow terminals.
func renderLogoSmall(styles appStyles) string {
	return styles.logo.Render(" CLI MATE ") + " " + styles.subtitle.Render("Your AI Coding Agent")
}
