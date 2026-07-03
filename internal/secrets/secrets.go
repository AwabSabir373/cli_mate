// Package secrets detects leaked secrets, API keys, and credentials in
// file content and strings. It can be used as a tool or as a post-edit check.
package secrets

import (
	"regexp"
	"strings"
)

// Severity indicates how critical a detected secret is.
type Severity string

const (
	SeverityHigh   Severity = "high"
	SeverityMedium Severity = "medium"
	SeverityLow    Severity = "low"
)

// Finding represents a detected secret.
type Finding struct {
	Line       int      `json:"line"`
	Column     int      `json:"column"`
	EndColumn  int      `json:"endColumn"`
	Kind       string   `json:"kind"`
	Severity   Severity `json:"severity"`
	Match      string   `json:"match"`
	Suggestion string   `json:"suggestion"`
}

// patterns maps detection patterns to their kind and severity.
var patterns = []struct {
	*regexp.Regexp
	kind       string
	severity   Severity
	suggestion string
}{
	// High severity
	{regexp.MustCompile(`\bsk-(?:proj-)?[A-Za-z0-9._-]{12,}\b`), "OpenAI API Key", SeverityHigh, "Remove the key and rotate it immediately"},
	{regexp.MustCompile(`\bsk-ant-api\d{2}-[A-Za-z0-9._-]{12,}\b`), "Anthropic API Key", SeverityHigh, "Remove the key and rotate it immediately"},
	{regexp.MustCompile(`\bgithub_pat_[A-Za-z0-9_]{12,}\b`), "GitHub Personal Access Token", SeverityHigh, "Remove the token and revoke it on GitHub"},
	{regexp.MustCompile(`\bgh[pousr]_[A-Za-z0-9_]{12,}\b`), "GitHub Token", SeverityHigh, "Remove the token and revoke it on GitHub"},
	{regexp.MustCompile(`\bglpat-[A-Za-z0-9_-]{12,}\b`), "GitLab Personal Access Token", SeverityHigh, "Remove the token and revoke it on GitLab"},
	{regexp.MustCompile(`\bAIza[0-9A-Za-z_-]{12,}\b`), "Google API Key", SeverityHigh, "Remove the key and regenerate it in Google Cloud Console"},
	{regexp.MustCompile(`\bxox[baprs]-[A-Za-z0-9-]{12,}\b`), "Slack Token", SeverityHigh, "Remove the token and revoke it in Slack settings"},
	{regexp.MustCompile(`\b(?:AKIA|ASIA)[A-Z0-9]{16}\b`), "AWS Access Key", SeverityHigh, "Remove the key and rotate it in AWS IAM"},
	{regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}\b`), "JWT Token", SeverityHigh, "Remove the token and rotate it"},
	{regexp.MustCompile(`(?s)-----BEGIN [A-Z ]*PRIVATE KEY-----.*?-----END [A-Z ]*PRIVATE KEY-----`), "Private Key", SeverityHigh, "Remove the private key and regenerate it"},

	// Medium severity
	{regexp.MustCompile(`(?i)\b(authorization|proxy-authorization)\s*:\s*(?:bearer|basic|token)\s+[A-Za-z0-9._\-+/=]{20,}\b`), "Authorization Header", SeverityMedium, "Remove the authorization header from the code"},
	{regexp.MustCompile(`(?i)\b(x-api-key|api-key)\s*:\s*[A-Za-z0-9._\-]{20,}\b`), "API Key Header", SeverityMedium, "Remove the API key from the code"},
	{regexp.MustCompile(`(?i)\b(password|passwd|secret)\s*=\s*["'][^"']{8,}["']`), "Hardcoded Password", SeverityMedium, "Move to environment variable or secret manager"},

	// Low severity
	{regexp.MustCompile(`(?i)\b(password|secret|token)\s*[:=]\s*[A-Za-z0-9._\-]{16,}\b`), "Potential Secret", SeverityLow, "Verify this is not a real secret"},
}

// Detect scans content for leaked secrets and returns findings.
func Detect(content string) []Finding {
	var findings []Finding

	// First, check for multiline patterns on the full content
	multilinePatterns := []struct {
		*regexp.Regexp
		kind       string
		severity   Severity
		suggestion string
	}{
		{regexp.MustCompile(`(?s)-----BEGIN [A-Z ]*PRIVATE KEY-----.*?-----END [A-Z ]*PRIVATE KEY-----`), "Private Key", SeverityHigh, "Remove the private key and regenerate it"},
	}

	for _, p := range multilinePatterns {
		loc := p.Regexp.FindStringIndex(content)
		if loc != nil {
			// Find which line this starts on
			lineNum := strings.Count(content[:loc[0]], "\n") + 1
			match := content[loc[0]:loc[1]]
			if !isTestValue(match) {
				findings = append(findings, Finding{
					Line:       lineNum,
					Column:     1,
					EndColumn:  1,
					Kind:       p.kind,
					Severity:   p.severity,
					Match:      redactMatch(match),
					Suggestion: p.suggestion,
				})
			}
		}
	}

	// Then check line-by-line patterns
	lines := strings.Split(content, "\n")
	for lineNum, line := range lines {
		// Track which positions have already been matched by higher-severity patterns
		type matchedRange struct {
			start, end int
		}
		var matchedRanges []matchedRange

		for _, p := range patterns {
			// Skip the private key pattern since we already handled it
			if p.kind == "Private Key" {
				continue
			}
			matches := p.FindAllStringIndex(line, -1)
			for _, match := range matches {
				// Skip if this position overlaps with an existing match
				overlaps := false
				for _, r := range matchedRanges {
					if match[0] < r.end && match[1] > r.start {
						overlaps = true
						break
					}
				}
				if overlaps {
					continue
				}

				value := line[match[0]:match[1]]
				if isTestValue(value) {
					continue
				}

				// Mark this range as matched
				matchedRanges = append(matchedRanges, matchedRange{start: match[0], end: match[1]})

				findings = append(findings, Finding{
					Line:       lineNum + 1,
					Column:     match[0] + 1,
					EndColumn:  match[1] + 1,
					Kind:       p.kind,
					Severity:   p.severity,
					Match:      redactMatch(value),
					Suggestion: p.suggestion,
				})
			}
		}
	}

	return findings
}

// DetectLine scans a single line for secrets.
func DetectLine(line string, lineNum int) []Finding {
	var findings []Finding

	for _, p := range patterns {
		matches := p.FindAllStringIndex(line, -1)
		for _, match := range matches {
			value := line[match[0]:match[1]]
			if isTestValue(value) {
				continue
			}

			findings = append(findings, Finding{
				Line:       lineNum,
				Column:     match[0] + 1,
				EndColumn:  match[1] + 1,
				Kind:       p.kind,
				Severity:   p.severity,
				Match:      redactMatch(value),
				Suggestion: p.suggestion,
			})
		}
	}

	return findings
}

// HasSecrets reports whether content contains any detected secrets.
func HasSecrets(content string) bool {
	return len(Detect(content)) > 0
}

// FormatFindings returns a human-readable summary of findings.
func FormatFindings(findings []Finding) string {
	if len(findings) == 0 {
		return "No secrets detected."
	}

	var b strings.Builder
	b.WriteString(strings.Repeat("=", 60) + "\n")
	b.WriteString("SECRET DETECTION REPORT\n")
	b.WriteString(strings.Repeat("=", 60) + "\n\n")

	for _, f := range findings {
		b.WriteString(f.SeverityEmoji() + " " + f.Kind + "\n")
		b.WriteString("  Line: " + string(rune('0'+f.Line)) + "\n")
		b.WriteString("  Match: " + f.Match + "\n")
		b.WriteString("  Fix: " + f.Suggestion + "\n\n")
	}

	b.WriteString(strings.Repeat("-", 60) + "\n")
	b.WriteString("Total: " + string(rune('0'+len(findings))) + " secret(s) found\n")

	return b.String()
}

// SeverityEmoji returns an emoji indicator for the severity level.
func (f Finding) SeverityEmoji() string {
	switch f.Severity {
	case SeverityHigh:
		return "🔴 HIGH"
	case SeverityMedium:
		return "🟡 MEDIUM"
	case SeverityLow:
		return "🟢 LOW"
	default:
		return "⚪ UNKNOWN"
	}
}

func redactMatch(value string) string {
	if len(value) <= 8 {
		return "***"
	}
	// Show first 4 and last 4 characters
	prefix := value[:4]
	suffix := value[len(value)-4:]
	middle := strings.Repeat("*", len(value)-8)
	return prefix + middle + suffix
}

func isTestValue(value string) bool {
	lower := strings.ToLower(value)
	// Only match values that are clearly test/placeholder values
	testIndicators := []string{
		"test-key", "test_key", "testsecret", "test-",
		"example-key", "example_key", "example-",
		"placeholder", "dummy", "mock", "fake", "sample",
		"xxx", "yyy", "zzz",
		"your-api-key", "your_api_key", "your-api-key-here",
		"my-api-key", "my_api_key",
		"insert-your", "replace-with",
	}
	for _, indicator := range testIndicators {
		if strings.Contains(lower, indicator) {
			return true
		}
	}
	return false
}
