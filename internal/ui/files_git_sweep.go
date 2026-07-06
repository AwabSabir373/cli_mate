package ui

import (
	"context"
	"os/exec"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
)

const gitSweepTimeout = 3 * time.Second

type gitSweepFile struct {
	path    string
	created bool
	adds    int
	dels    int
}

type gitSweepMsg struct {
	baseline bool
	ok       bool
	files    []gitSweepFile
}

func gitSweepCmd(parent context.Context, cwd string, baseline bool) tea.Cmd {
	return func() tea.Msg {
		if parent == nil {
			parent = context.Background()
		}
		ctx, cancel := context.WithTimeout(parent, gitSweepTimeout)
		defer cancel()
		status, err := runGitCmd(ctx, cwd, "status", "--porcelain", "--untracked-files=all")
		if err != nil {
			return gitSweepMsg{baseline: baseline, ok: false}
		}
		files := parseGitPorcelain(status)
		if numstat, err := runGitCmd(ctx, cwd, "diff", "HEAD", "--numstat", "-z"); err == nil {
			stats := parseGitNumstat(numstat)
			for i := range files {
				if counts, ok := stats[files[i].path]; ok {
					files[i].adds, files[i].dels = counts[0], counts[1]
				}
			}
		}
		return gitSweepMsg{baseline: baseline, ok: true, files: files}
	}
}

func runGitCmd(ctx context.Context, cwd, arg0 string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{arg0}, args...)...)
	cmd.Dir = cwd
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func parseGitPorcelain(out string) []gitSweepFile {
	var files []gitSweepFile
	for _, line := range strings.Split(out, "\n") {
		if len(line) < 4 {
			continue
		}
		code, path := line[:2], strings.TrimSpace(line[3:])
		if to, _, found := cutRename(path); found {
			path = to
		}
		path = unquoteGitPath(path)
		if path == "" {
			continue
		}
		files = append(files, gitSweepFile{
			path:    path,
			created: code == "??" || strings.Contains(code, "A"),
		})
	}
	return files
}

func cutRename(path string) (string, string, bool) {
	if idx := strings.Index(path, " -> "); idx >= 0 {
		return path[idx+4:], path[:idx], true
	}
	return "", "", false
}

func unquoteGitPath(path string) string {
	if len(path) >= 2 && strings.HasPrefix(path, `"`) && strings.HasSuffix(path, `"`) {
		return path[1 : len(path)-1]
	}
	return path
}

func parseGitNumstat(out string) map[string][2]int {
	stats := map[string][2]int{}
	fields := strings.Split(out, "\x00")
	for i := 0; i < len(fields); i++ {
		parts := strings.SplitN(fields[i], "\t", 3)
		if len(parts) != 3 {
			continue
		}
		adds, errA := strconv.Atoi(parts[0])
		dels, errD := strconv.Atoi(parts[1])
		if errA != nil || errD != nil {
			continue
		}
		path := parts[2]
		if path == "" {
			if i+2 >= len(fields) {
				break
			}
			path = fields[i+2]
			i += 2
		}
		if path != "" {
			stats[path] = [2]int{adds, dels}
		}
	}
	return stats
}

func mergeTouchedFiles(logFiles []touchedFile, gitFiles []gitSweepFile) []touchedFile {
	if len(gitFiles) == 0 {
		return logFiles
	}
	seen := make(map[string]bool, len(logFiles))
	for _, f := range logFiles {
		seen[f.path] = true
	}
	merged := make([]touchedFile, 0, len(logFiles)+len(gitFiles))
	for i := len(gitFiles) - 1; i >= 0; i-- {
		gf := gitFiles[i]
		if seen[gf.path] {
			continue
		}
		merged = append(merged, touchedFile{
			path:    gf.path,
			added:   gf.adds,
			deleted: gf.dels,
			created: gf.created,
			edits:   1,
		})
		seen[gf.path] = true
	}
	merged = append(merged, logFiles...)
	return merged
}
