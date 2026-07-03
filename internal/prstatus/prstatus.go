// Package prstatus provides pull request status tracking for the sidebar.
package prstatus

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

// PRStatus represents a pull request status.
type PRStatus struct {
	Number    int       `json:"number"`
	Title     string    `json:"title"`
	State     string    `json:"state"`
	Branch    string    `json:"branch"`
	Checks    string    `json:"checks"` // "passing", "failing", "pending", ""
	UpdatedAt time.Time `json:"updatedAt"`
	URL       string    `json:"url"`
}

// CheckResult is the result of a PR status check.
type CheckResult struct {
	PR    *PRStatus
	Error string
}

// githubPR is the subset of the GitHub PR API response we need.
type githubPR struct {
	Number    int    `json:"number"`
	Title     string `json:"title"`
	State     string `json:"state"`
	Head      struct {
		Ref string `json:"ref"`
		Sha string `json:"sha"`
	} `json:"head"`
	HTMLURL   string `json:"html_url"`
	UpdatedAt string `json:"updated_at"`
}

// GetCurrentPR detects the current branch and fetches PR status.
func GetCurrentPR(ctx context.Context) CheckResult {
	branch, err := getCurrentBranch()
	if err != nil {
		return CheckResult{Error: fmt.Sprintf("git: %v", err)}
	}

	if branch == "" || branch == "main" || branch == "master" || branch == "HEAD" {
		return CheckResult{}
	}

	remote, err := getGitRemote()
	if err != nil {
		return CheckResult{
			PR: &PRStatus{Branch: branch, State: "local", Title: branch},
		}
	}

	owner, repo := parseRemote(remote)
	if owner == "" || repo == "" {
		return CheckResult{
			PR: &PRStatus{Branch: branch, State: "local", Title: branch},
		}
	}

	token := getGithubToken()
	pr := fetchPRForBranch(ctx, owner, repo, branch, token)
	if pr != nil {
		return CheckResult{PR: pr}
	}

	return CheckResult{
		PR: &PRStatus{Branch: branch, State: "local", Title: branch},
	}
}

func getCurrentBranch() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func getGitRemote() (string, error) {
	for _, name := range []string{"origin", "upstream"} {
		cmd := exec.Command("git", "remote", "get-url", name)
		if out, err := cmd.Output(); err == nil {
			return strings.TrimSpace(string(out)), nil
		}
	}
	return "", fmt.Errorf("no git remote found")
}

func parseRemote(remote string) (owner, repo string) {
	remote = strings.TrimSpace(remote)
	remote = strings.TrimSuffix(remote, ".git")

	// Handle git@github.com:owner/repo
	if idx := strings.LastIndex(remote, ":"); idx > 0 && !strings.Contains(remote, "://") {
		path := remote[idx+1:]
		parts := strings.SplitN(path, "/", 2)
		if len(parts) == 2 {
			return parts[0], parts[1]
		}
		return "", ""
	}

	// Handle https://github.com/owner/repo
	parts := strings.Split(remote, "/")
	if len(parts) >= 2 {
		return parts[len(parts)-2], parts[len(parts)-1]
	}
	return "", ""
}

func getGithubToken() string {
	if t := strings.TrimSpace(os.Getenv("GITHUB_TOKEN")); t != "" {
		return t
	}
	return strings.TrimSpace(os.Getenv("GH_TOKEN"))
}

func fetchPRForBranch(ctx context.Context, owner, repo, branch, token string) *PRStatus {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls?head=%s:%s&state=open", owner, repo, owner, branch)
	body, err := githubGet(ctx, url, token)
	if err != nil {
		return nil
	}

	var prs []githubPR
	if err := json.Unmarshal(body, &prs); err != nil || len(prs) == 0 {
		return nil
	}

	pr := prs[0]
	prStatus := &PRStatus{
		Number: pr.Number,
		Title:  pr.Title,
		State:  pr.State,
		Branch: pr.Head.Ref,
		URL:    pr.HTMLURL,
	}
	if t, err := time.Parse(time.RFC3339, pr.UpdatedAt); err == nil {
		prStatus.UpdatedAt = t
	}
	if pr.Head.Sha != "" {
		prStatus.Checks = fetchCheckStatus(ctx, owner, repo, pr.Head.Sha, token)
	}
	return prStatus
}

func fetchCheckStatus(ctx context.Context, owner, repo, sha, token string) string {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/commits/%s/check-runs", owner, repo, sha)
	body, err := githubGet(ctx, url, token)
	if err != nil {
		return "unknown"
	}

	var result struct {
		CheckRuns []struct {
			Conclusion string `json:"conclusion"`
			Status     string `json:"status"`
		} `json:"check_runs"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "unknown"
	}

	if len(result.CheckRuns) == 0 {
		return "pending"
	}

	pass, fail := 0, 0
	for _, run := range result.CheckRuns {
		if run.Status != "completed" {
			continue
		}
		switch run.Conclusion {
		case "success":
			pass++
		case "failure", "timed_out", "action_required":
			fail++
		}
	}
	if fail > 0 {
		return "failing"
	}
	if pass > 0 {
		return "passing"
	}
	return "pending"
}

func githubGet(ctx context.Context, url, token string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "cli_mate")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github API: HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 1<<20))
}

// FormatStatus returns a human-readable summary for the sidebar.
func FormatStatus(pr *PRStatus) string {
	if pr == nil {
		return ""
	}
	var b strings.Builder
	if pr.Number > 0 {
		b.WriteString(fmt.Sprintf("#%d ", pr.Number))
	}
	b.WriteString(pr.Title)
	if pr.Checks != "" && pr.Checks != "unknown" {
		switch pr.Checks {
		case "passing":
			b.WriteString(" ✓")
		case "failing":
			b.WriteString(" ✗")
		case "pending":
			b.WriteString(" ○")
		}
	}
	return b.String()
}
