package ui

import (
	"fmt"
	"testing"

	"charm.land/lipgloss/v2"

	"cli_mate/internal/providers"
)

func TestDiagPanelDimensions(t *testing.T) {
	styles := buildStyles(themeFor("midnight"))
	for _, h := range []int{20, 24, 30, 40} {
		for _, w := range []int{40, 50, 58, 70, 80, 100, 120, 200} {
			app := App{
				styles:    styles,
				width:     w,
				height:    h,
				sidebar:   NewSidebar(NewPlanPanel()),
				planPanel: NewPlanPanel(),
				viewport:  newViewport(),
				log:       []logEntry{{Kind: "system", Text: "hello"}},
				messages:  []providers.Message{{Role: "user", Content: "hi"}},
			}
			app.sidebar.Toggle()
			got := app.View().Content
			pw := lipgloss.Width(got)
			ph := lipgloss.Height(got)
			status := "OK"
			if pw > w {
				status = "OVERFLOW-H"
			}
			if ph > h {
				status = "OVERFLOW-V"
			}
			if ph < h-2 {
				status = fmt.Sprintf("SHORT(term=%d,panel=%d)", h, ph)
			}
			fmt.Printf("w=%d h=%d panelW=%d panelH=%d -> %s\n", w, h, pw, ph, status)
		}
	}
}
