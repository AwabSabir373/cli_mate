package ui

import (
	"strings"
)

// renderLogo renders the CLI MATE ASCII art logo with pixel scatter effect.
// The logo matches the user's branding: scattered blue pixels on the left,
// bold blocky "CLI MATE" text.
func renderLogo(styles appStyles, width int) string {
	return renderCleanLogo(styles, width)
}

// renderCleanLogo renders a clean block-letter CLI MATE logo with pixel scatter.
func renderCleanLogo(styles appStyles, _ int) string {
	accent := styles.accent
	dim := styles.muted

	// Pixel scatter (left side) — compact
	px := []string{
		dim.Render("  ▪ ") + accent.Render("█ ") + dim.Render("▪  ") + accent.Render("█  ") + dim.Render("▪   "),
		dim.Render("▪ ") + accent.Render("█ ") + dim.Render("▪ ") + accent.Render("█ ") + dim.Render("▪ ") + accent.Render("▪  ") + dim.Render("  "),
		dim.Render("  ") + accent.Render("█ ") + dim.Render("▪ ") + accent.Render("█ ") + dim.Render("▪ ") + accent.Render("█  ") + dim.Render(" "),
		dim.Render("▪ ") + accent.Render("█ ") + dim.Render("▪  ") + dim.Render("▪ ") + accent.Render("█ ") + accent.Render("▪  ") + dim.Render(" "),
		dim.Render("  ") + accent.Render("█ ") + dim.Render("▪ ") + accent.Render("█ ") + dim.Render("▪  ") + dim.Render("   "),
		dim.Render("▪ ") + accent.Render("█ ") + dim.Render("▪  ") + accent.Render("█ ") + dim.Render("▪  ") + dim.Render("   "),
		dim.Render("  ") + accent.Render("█ ") + dim.Render("▪ ") + dim.Render("▪ ") + accent.Render("█ ") + dim.Render("▪  "),
		dim.Render("▪  ") + accent.Render("█ ") + dim.Render("▪ ") + accent.Render("█ ") + dim.Render("▪  ") + dim.Render("  "),
	}

	// Block letters for "CLI MATE"
	main := []string{
		" ██████╗██╗     ██╗    ███╗   ███╗ █████╗ ████████╗███████╗",
		"██╔════╝██║     ██║    ████╗ ████║██╔══██╗╚══██╔══╝██╔════╝",
		"██║     ██║     ██║    ██╔████╔██║███████║   ██║   █████╗  ",
		"██║     ██║     ██║    ██║╚██╔╝██║██╔══██║   ██║   ██╔══╝  ",
		"╚██████╗███████╗██║    ██║ ╚═╝ ██║██║  ██║   ██║   ███████╗",
		" ╚═════╝╚══════╝╚═╝    ╚═╝     ╚═╝╚═╝  ╚═╝   ╚═╝   ╚══════╝",
		"            Your Command Line Companion",
	}

	// Compose: pixel scatter + main text
	var lines []string
	for i := 0; i < len(main); i++ {
		pxLine := ""
		if i < len(px) {
			pxLine = px[i]
		}
		lines = append(lines, pxLine+"  "+accent.Render(main[i]))
	}

	return strings.Join(lines, "\n")
}

// renderLogoSmall renders a compact version of the logo for narrow terminals.
func renderLogoSmall(styles appStyles) string {
	return styles.logo.Render(" CLI MATE ") + " " + styles.subtitle.Render("Terminal-first AI coding agent")
}
