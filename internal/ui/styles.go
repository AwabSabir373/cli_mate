package ui

import "github.com/charmbracelet/lipgloss"

type themeColors struct {
	bg        lipgloss.Color
	fg        lipgloss.Color
	accent    lipgloss.Color
	muted     lipgloss.Color
	panel     lipgloss.Color
	softPanel lipgloss.Color
	pill      lipgloss.Color
	selected  lipgloss.Color
	error     lipgloss.Color
	success   lipgloss.Color
	border    lipgloss.Color
}

var themes = map[string]themeColors{
	"midnight": {
		bg:        lipgloss.Color("234"),
		fg:        lipgloss.Color("252"),
		accent:    lipgloss.Color("39"),
		muted:     lipgloss.Color("243"),
		panel:     lipgloss.Color("236"),
		softPanel: lipgloss.Color("238"),
		pill:      lipgloss.Color("240"),
		selected:  lipgloss.Color("63"),
		error:     lipgloss.Color("196"),
		success:   lipgloss.Color("42"),
		border:    lipgloss.Color("240"),
	},
	"matrix": {
		bg:        lipgloss.Color("0"),
		fg:        lipgloss.Color("46"),
		accent:    lipgloss.Color("46"),
		muted:     lipgloss.Color("22"),
		panel:     lipgloss.Color("0"),
		softPanel: lipgloss.Color("22"),
		pill:      lipgloss.Color("22"),
		selected:  lipgloss.Color("40"),
		error:     lipgloss.Color("196"),
		success:   lipgloss.Color("46"),
		border:    lipgloss.Color("22"),
	},
	"paper": {
		bg:        lipgloss.Color("230"),
		fg:        lipgloss.Color("234"),
		accent:    lipgloss.Color("25"),
		muted:     lipgloss.Color("247"),
		panel:     lipgloss.Color("255"),
		softPanel: lipgloss.Color("253"),
		pill:      lipgloss.Color("252"),
		selected:  lipgloss.Color("69"),
		error:     lipgloss.Color("160"),
		success:   lipgloss.Color("28"),
		border:    lipgloss.Color("250"),
	},
	"mono": {
		bg:        lipgloss.Color("234"),
		fg:        lipgloss.Color("252"),
		accent:    lipgloss.Color("252"),
		muted:     lipgloss.Color("245"),
		panel:     lipgloss.Color("236"),
		softPanel: lipgloss.Color("238"),
		pill:      lipgloss.Color("240"),
		selected:  lipgloss.Color("248"),
		error:     lipgloss.Color("208"),
		success:   lipgloss.Color("252"),
		border:    lipgloss.Color("240"),
	},
	"catppuccin": {
		bg:        lipgloss.Color("#1e1e2e"),
		fg:        lipgloss.Color("#cdd6f4"),
		accent:    lipgloss.Color("#89b4fa"),
		muted:     lipgloss.Color("#6c7086"),
		panel:     lipgloss.Color("#313244"),
		softPanel: lipgloss.Color("#45475a"),
		pill:      lipgloss.Color("#38384d"),
		selected:  lipgloss.Color("#89b4fa"),
		error:     lipgloss.Color("#f38ba8"),
		success:   lipgloss.Color("#a6e3a1"),
		border:    lipgloss.Color("#6c7086"),
	},
	"dracula": {
		bg:        lipgloss.Color("#282a36"),
		fg:        lipgloss.Color("#f8f8f2"),
		accent:    lipgloss.Color("#bd93f9"),
		muted:     lipgloss.Color("#6272a4"),
		panel:     lipgloss.Color("#3a3c4e"),
		softPanel: lipgloss.Color("#44475a"),
		pill:      lipgloss.Color("#3d3f52"),
		selected:  lipgloss.Color("#bd93f9"),
		error:     lipgloss.Color("#ff5555"),
		success:   lipgloss.Color("#50fa7b"),
		border:    lipgloss.Color("#6272a4"),
	},
	"nord": {
		bg:        lipgloss.Color("#2e3440"),
		fg:        lipgloss.Color("#d8dee9"),
		accent:    lipgloss.Color("#88c0d0"),
		muted:     lipgloss.Color("#4c566a"),
		panel:     lipgloss.Color("#3b4252"),
		softPanel: lipgloss.Color("#434c5e"),
		pill:      lipgloss.Color("#3e4656"),
		selected:  lipgloss.Color("#88c0d0"),
		error:     lipgloss.Color("#bf616a"),
		success:   lipgloss.Color("#a3be8c"),
		border:    lipgloss.Color("#4c566a"),
	},
	"gruvbox": {
		bg:        lipgloss.Color("#282828"),
		fg:        lipgloss.Color("#ebdbb2"),
		accent:    lipgloss.Color("#83a598"),
		muted:     lipgloss.Color("#928374"),
		panel:     lipgloss.Color("#3c3836"),
		softPanel: lipgloss.Color("#504945"),
		pill:      lipgloss.Color("#43403c"),
		selected:  lipgloss.Color("#83a598"),
		error:     lipgloss.Color("#fb4934"),
		success:   lipgloss.Color("#b8bb26"),
		border:    lipgloss.Color("#928374"),
	},
	"tokyonight": {
		bg:        lipgloss.Color("#1a1b26"),
		fg:        lipgloss.Color("#c0caf5"),
		accent:    lipgloss.Color("#7aa2f7"),
		muted:     lipgloss.Color("#565f89"),
		panel:     lipgloss.Color("#24283b"),
		softPanel: lipgloss.Color("#414868"),
		pill:      lipgloss.Color("#2a2e3f"),
		selected:  lipgloss.Color("#7aa2f7"),
		error:     lipgloss.Color("#f7768e"),
		success:   lipgloss.Color("#9ece6a"),
		border:    lipgloss.Color("#565f89"),
	},
	"rosepine": {
		bg:        lipgloss.Color("#191724"),
		fg:        lipgloss.Color("#e0def4"),
		accent:    lipgloss.Color("#9ccfd8"),
		muted:     lipgloss.Color("#6e6a86"),
		panel:     lipgloss.Color("#26233a"),
		softPanel: lipgloss.Color("#393552"),
		pill:      lipgloss.Color("#2a273f"),
		selected:  lipgloss.Color("#9ccfd8"),
		error:     lipgloss.Color("#eb6f92"),
		success:   lipgloss.Color("#31748f"),
		border:    lipgloss.Color("#6e6a86"),
	},
	"solarized": {
		bg:        lipgloss.Color("#002b36"),
		fg:        lipgloss.Color("#839496"),
		accent:    lipgloss.Color("#268bd2"),
		muted:     lipgloss.Color("#586e75"),
		panel:     lipgloss.Color("#073642"),
		softPanel: lipgloss.Color("#0a4050"),
		pill:      lipgloss.Color("#053540"),
		selected:  lipgloss.Color("#268bd2"),
		error:     lipgloss.Color("#dc322f"),
		success:   lipgloss.Color("#859900"),
		border:    lipgloss.Color("#586e75"),
	},
	"onedark": {
		bg:        lipgloss.Color("#282c34"),
		fg:        lipgloss.Color("#abb2bf"),
		accent:    lipgloss.Color("#61afef"),
		muted:     lipgloss.Color("#5c6370"),
		panel:     lipgloss.Color("#31353f"),
		softPanel: lipgloss.Color("#3e4451"),
		pill:      lipgloss.Color("#353b45"),
		selected:  lipgloss.Color("#61afef"),
		error:     lipgloss.Color("#e06c75"),
		success:   lipgloss.Color("#98c379"),
		border:    lipgloss.Color("#5c6370"),
	},
}

func themeFor(name string) themeColors {
	if t, ok := themes[name]; ok {
		return t
	}
	return themes["midnight"]
}

type appStyles struct {
	logo       lipgloss.Style
	title      lipgloss.Style
	subtitle   lipgloss.Style
	panel      lipgloss.Style
	softPanel  lipgloss.Style
	pill       lipgloss.Style
	accent     lipgloss.Style
	muted      lipgloss.Style
	divider    lipgloss.Style
	prompt     lipgloss.Style
	input      lipgloss.Style
	inputPanel lipgloss.Style
	selected   lipgloss.Style
	error      lipgloss.Style
	success    lipgloss.Style
	heroBorder lipgloss.Style
	pillRow    lipgloss.Style
	logTime    lipgloss.Style
	logPrefix  lipgloss.Style
	roleTool   lipgloss.Style
	roleFile   lipgloss.Style
	roleSystem lipgloss.Style
	roleAssist lipgloss.Style
	diffAdd    lipgloss.Style
	diffRemove lipgloss.Style
	tokenCount lipgloss.Style
	cursor     lipgloss.Style
	// New styles for enhanced UI
	card         lipgloss.Style
	cardHeader   lipgloss.Style
	cardBody     lipgloss.Style
	cardFooter   lipgloss.Style
	progressBar  lipgloss.Style
	progressFill lipgloss.Style
	progressBg   lipgloss.Style
	permission   lipgloss.Style
	sidebarTitle lipgloss.Style
	sidebarItem  lipgloss.Style
	hover        lipgloss.Style
	badge        lipgloss.Style
	badgeAdd     lipgloss.Style
	badgeDel     lipgloss.Style
	code         lipgloss.Style
	codePanel    lipgloss.Style
	spinner      lipgloss.Style
	fadeFresh    lipgloss.Style
	fadeInk      lipgloss.Style
	planHeader   lipgloss.Style
	planStep     lipgloss.Style
	planProgress lipgloss.Style
	info         lipgloss.Style
	scrollHint   lipgloss.Style
}

func buildStyles(c themeColors) appStyles {
	return appStyles{
		logo: lipgloss.NewStyle().
			Background(c.accent).Foreground(lipgloss.Color("230")).
			Padding(0, 1).Bold(true),
		title: lipgloss.NewStyle().
			Foreground(c.accent).Bold(true),
		subtitle: lipgloss.NewStyle().
			Foreground(c.muted).Italic(true),
		panel: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(c.border).
			Padding(1, 2),
		softPanel: lipgloss.NewStyle().
			Background(c.softPanel).
			Padding(1, 2),
		pill: lipgloss.NewStyle().
			Background(c.pill).Foreground(c.fg).
			Padding(0, 1),
		accent: lipgloss.NewStyle().
			Foreground(c.accent).Bold(true),
		muted: lipgloss.NewStyle().
			Foreground(c.muted),
		divider: lipgloss.NewStyle().
			Foreground(c.muted),
		prompt: lipgloss.NewStyle().
			Foreground(c.accent).Bold(true),
		input: lipgloss.NewStyle().
			Foreground(c.fg),
		inputPanel: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(c.accent).
			Padding(0, 1),
		selected: lipgloss.NewStyle().
			Background(c.selected).Foreground(lipgloss.Color("230")).
			Padding(0, 1),
		error: lipgloss.NewStyle().
			Foreground(c.error).Bold(true),
		success: lipgloss.NewStyle().
			Foreground(c.success).Bold(true),
		heroBorder: lipgloss.NewStyle().
			Border(lipgloss.ThickBorder()).
			BorderForeground(c.accent).
			Padding(0, 1),
		pillRow: lipgloss.NewStyle().
			Padding(0, 1),
		logTime: lipgloss.NewStyle().
			Foreground(c.muted).Width(8),
		logPrefix: lipgloss.NewStyle().
			Foreground(c.muted).Width(10),
		roleTool: lipgloss.NewStyle().
			Foreground(c.accent).Bold(true),
		roleFile: lipgloss.NewStyle().
			Foreground(c.success),
		roleSystem: lipgloss.NewStyle().
			Foreground(c.muted).Italic(true),
		roleAssist: lipgloss.NewStyle().
			Foreground(c.fg),
		diffAdd: lipgloss.NewStyle().
			Foreground(lipgloss.Color("42")),
		diffRemove: lipgloss.NewStyle().
			Foreground(c.error),
		tokenCount: lipgloss.NewStyle().
			Foreground(c.muted).Italic(true),
		cursor: lipgloss.NewStyle().
			Background(c.fg).Foreground(c.bg),
		// New enhanced styles
		card: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(c.border).
			Padding(0, 1),
		cardHeader: lipgloss.NewStyle().
			Foreground(c.accent).Bold(true),
		cardBody: lipgloss.NewStyle().
			Foreground(c.fg),
		cardFooter: lipgloss.NewStyle().
			Foreground(c.muted).Italic(true),
		progressBar: lipgloss.NewStyle().
			Width(20),
		progressFill: lipgloss.NewStyle().
			Background(c.success).
			Foreground(c.success),
		progressBg: lipgloss.NewStyle().
			Background(c.softPanel).
			Foreground(c.softPanel),
		permission: lipgloss.NewStyle().
			Foreground(lipgloss.Color("228")).Bold(true),
		sidebarTitle: lipgloss.NewStyle().
			Foreground(c.muted).Bold(true),
		sidebarItem: lipgloss.NewStyle().
			Foreground(c.fg),
		hover: lipgloss.NewStyle().
			Background(c.selected).Foreground(lipgloss.Color("230")),
		badge: lipgloss.NewStyle().
			Background(c.pill).Foreground(c.fg).
			Padding(0, 1),
		badgeAdd: lipgloss.NewStyle().
			Foreground(lipgloss.Color("42")).Bold(true),
		badgeDel: lipgloss.NewStyle().
			Foreground(c.error).Bold(true),
		code: lipgloss.NewStyle().
			Foreground(c.fg),
		codePanel: lipgloss.NewStyle().
			Background(c.softPanel).
			Padding(0, 1),
		spinner: lipgloss.NewStyle().
			Foreground(c.accent),
		fadeFresh: lipgloss.NewStyle().
			Foreground(c.accent),
		fadeInk: lipgloss.NewStyle().
			Foreground(c.fg),
		planHeader: lipgloss.NewStyle().
			Foreground(c.accent).Bold(true),
		planStep: lipgloss.NewStyle().
			Foreground(c.fg),
		planProgress: lipgloss.NewStyle().
			Foreground(c.muted),
		info: lipgloss.NewStyle().
			Foreground(c.accent),
		scrollHint: lipgloss.NewStyle().
			Foreground(c.muted).Italic(true),
	}
}
