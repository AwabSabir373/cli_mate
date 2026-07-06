package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func (a *App) handleAddDirCommand(arg string) {
	trimmed := strings.TrimSpace(arg)
	if trimmed == "" {
		a.appendLog("system", "Usage: /add-dir <path>")
		return
	}
	abs, err := filepath.Abs(trimmed)
	if err != nil {
		a.appendLog("error", fmt.Sprintf("add-dir: %s", err.Error()))
		return
	}
	info, err := os.Stat(abs)
	if err != nil {
		a.appendLog("error", fmt.Sprintf("add-dir: %s", err.Error()))
		return
	}
	if !info.IsDir() {
		a.appendLog("error", fmt.Sprintf("add-dir: %s is not a directory", abs))
		return
	}
	a.appendLog("system", fmt.Sprintf("Added directory: %s", abs))
}
