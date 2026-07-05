// Package update checks for new versions of cli_mate by querying GitHub releases.
package update

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const (
	githubReleasesURL = "https://api.github.com/repos/cli-mate/cli-mate/releases/latest"
	checkTimeout      = 5 * time.Second
)

// Version info for the current build. Set via ldflags at build time.
var (
	Version   = "dev"
	Commit    = "none"
	buildDate = "unknown"
)

// Release represents a GitHub release.
type Release struct {
	TagName string `json:"tag_name"`
	Name    string `json:"name"`
	HTMLURL string `json:"html_url"`
}

// CheckResult contains the result of an update check.
type CheckResult struct {
	Current    string
	Latest     string
	Available  bool
	ReleaseURL string
	Error      error
}

// CheckForUpdate queries GitHub for the latest release and compares it
// to the current version. Returns immediately if the check fails.
func CheckForUpdate(ctx context.Context) CheckResult {
	current := strings.TrimPrefix(Version, "v")
	if current == "dev" || current == "none" {
		return CheckResult{Current: current}
	}

	ctx, cancel := context.WithTimeout(ctx, checkTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, githubReleasesURL, nil)
	if err != nil {
		return CheckResult{Current: current, Error: err}
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return CheckResult{Current: current, Error: err}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return CheckResult{Current: current, Error: fmt.Errorf("GitHub API returned %d", resp.StatusCode)}
	}

	var release Release
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return CheckResult{Current: current, Error: err}
	}

	latest := strings.TrimPrefix(release.TagName, "v")
	available := latest != "" && latest != current

	return CheckResult{
		Current:    current,
		Latest:     latest,
		Available:  available,
		ReleaseURL: release.HTMLURL,
	}
}

// FormatCheckResult returns a user-friendly message about the update check.
func FormatCheckResult(result CheckResult) string {
	if result.Error != nil {
		return fmt.Sprintf("Update check failed: %s", result.Error)
	}
	if !result.Available {
		return fmt.Sprintf("You are running the latest version (%s).", result.Current)
	}
	return fmt.Sprintf("Update available: %s -> %s\nDownload: %s",
		result.Current, result.Latest, result.ReleaseURL)
}
