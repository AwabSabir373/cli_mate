package ui

import (
	"net/url"
	"regexp"
	"sort"
	"strings"

	"cli_mate/internal/config"
)

type MCPViewState struct {
	Servers     []MCPServerView
	Tools       []MCPToolView
	Permissions MCPPermissionSummary
}

type MCPServerView struct {
	Name      string
	Transport string
	State     string
	Target    string
	Auth      string
	ToolCount int
}

type MCPToolView struct {
	ServerName   string
	Name         string
	RegistryName string
	SideEffect   string
	Permission   string
	Description  string
}

type MCPPermissionSummary struct {
	Mode        string
	PromptCount int
	DeniedCount int
	GrantCount  int
}

type MCPStateOptions struct {
	Config         []config.MCPConfig
	PermissionMode string
	PromptCount    int
	DeniedCount    int
}

const mcpDisplayRedacted = "[REDACTED]"

var mcpStateUnsafeToolNameChars = regexp.MustCompile(`[^A-Za-z0-9_]+`)

func BuildMCPViewState(options MCPStateOptions) MCPViewState {
	servers := buildMCPServerViews(options.Config)
	return MCPViewState{
		Servers:     servers,
		Tools:       nil,
		Permissions: buildMCPPermissionSummary(options),
	}
}

func buildMCPServerViews(cfgs []config.MCPConfig) []MCPServerView {
	views := make([]MCPServerView, 0, len(cfgs))
	for _, cfg := range cfgs {
		transport := "stdio"
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(cfg.Command)), "http") ||
			strings.Contains(strings.ToLower(strings.TrimSpace(cfg.Command)), "://") {
			transport = "http"
		}
		views = append(views, MCPServerView{
			Name:      cfg.Name,
			Transport: transport,
			State:     "configured",
			Target:    mcpServerTarget(cfg),
			ToolCount: 0,
		})
	}
	return views
}

func buildMCPPermissionSummary(options MCPStateOptions) MCPPermissionSummary {
	return MCPPermissionSummary{
		Mode:        strings.TrimSpace(options.PermissionMode),
		PromptCount: options.PromptCount,
		DeniedCount: options.DeniedCount,
	}
}

func mcpServerTarget(cfg config.MCPConfig) string {
	parts := []string{}
	if command := strings.TrimSpace(cfg.Command); command != "" {
		parts = append(parts, command)
	}
	parts = append(parts, redactedCommandArgs(cfg.Args)...)
	if env := redactedStringMap(cfg.Env); env != "" {
		parts = append(parts, "env", env)
	}
	return strings.Join(parts, " ")
}

func redactedStringMap(values map[string]string) string {
	if len(values) == 0 {
		return ""
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		if key = strings.TrimSpace(key); key != "" {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+mcpDisplayRedacted)
	}
	return strings.Join(parts, " ")
}

func redactedCommandArgs(values []string) []string {
	trimmed := make([]string, 0, len(values))
	redactNext := false
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			if redactNext {
				if looksLikeMCPDisplayURLValue(value) {
					trimmed = append(trimmed, redactMCPDisplayURL(value))
				} else {
					trimmed = append(trimmed, mcpDisplayRedacted)
				}
				redactNext = false
				continue
			}
			if key, rest, ok := strings.Cut(value, "="); ok {
				switch {
				case isSensitiveMCPDisplayKey(key):
					trimmed = append(trimmed, key+"="+mcpDisplayRedacted)
					continue
				case looksLikeMCPDisplayURLValue(rest):
					trimmed = append(trimmed, key+"="+redactMCPDisplayURL(rest))
					continue
				}
			}
			if isSensitiveMCPDisplayFlag(value) {
				trimmed = append(trimmed, value)
				redactNext = true
				continue
			}
			if looksLikeMCPDisplayURLValue(value) {
				trimmed = append(trimmed, redactMCPDisplayURL(value))
				continue
			}
			trimmed = append(trimmed, value)
		}
	}
	return trimmed
}

func redactMCPDisplayURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return fallbackRedactMCPDisplayURL(raw)
	}
	if parsed.User != nil {
		parsed.User = nil
	}
	if parsed.RawQuery != "" {
		parsed.RawQuery = redactMCPDisplayRawQuery(parsed.RawQuery)
	}
	if parsed.Fragment != "" {
		parsed.Fragment = redactMCPDisplayRawQuery(parsed.Fragment)
	}
	out := parsed.String()
	if strings.TrimSpace(out) == "" {
		return fallbackRedactMCPDisplayURL(raw)
	}
	return strings.ReplaceAll(out, "%5BREDACTED%5D", mcpDisplayRedacted)
}

func redactMCPDisplayRawQuery(rawQuery string) string {
	parts := strings.Split(rawQuery, "&")
	for index, part := range parts {
		if part == "" {
			continue
		}
		key, _, hasValue := strings.Cut(part, "=")
		decodedKey, err := url.QueryUnescape(key)
		if err != nil {
			decodedKey = key
		}
		if !isSensitiveMCPDisplayKey(decodedKey) {
			continue
		}
		if hasValue {
			parts[index] = key + "=" + mcpDisplayRedacted
		} else {
			parts[index] = key
		}
	}
	return strings.Join(parts, "&")
}

func looksLikeMCPDisplayURLValue(value string) bool {
	value = strings.TrimSpace(value)
	lower := strings.ToLower(value)
	return strings.Contains(value, "://") ||
		strings.HasPrefix(lower, "http:") ||
		strings.HasPrefix(lower, "https:") ||
		strings.Contains(value, "?") ||
		strings.Contains(value, "#")
}

func fallbackRedactMCPDisplayURL(raw string) string {
	out := strings.TrimSpace(raw)
	if out == "" {
		return ""
	}
	if schemeIndex := strings.Index(out, "://"); schemeIndex >= 0 {
		authorityStart := schemeIndex + len("://")
		authorityEnd := len(out)
		for _, marker := range []string{"/", "?", "#"} {
			if index := strings.Index(out[authorityStart:], marker); index >= 0 && authorityStart+index < authorityEnd {
				authorityEnd = authorityStart + index
			}
		}
		if at := strings.LastIndex(out[authorityStart:authorityEnd], "@"); at >= 0 {
			out = out[:authorityStart] + out[authorityStart+at+1:]
		}
	}
	if head, fragment, ok := strings.Cut(out, "#"); ok {
		fragment = redactMCPDisplayRawQuery(fragment)
		out = head + "#" + fragment
	}
	if head, query, ok := strings.Cut(out, "?"); ok {
		query = redactMCPDisplayRawQuery(query)
		out = head + "?" + query
	}
	return out
}

func isSensitiveMCPDisplayFlag(value string) bool {
	value = strings.TrimLeft(strings.ToLower(strings.TrimSpace(value)), "-")
	if key, _, ok := strings.Cut(value, "="); ok {
		value = key
	}
	return isSensitiveMCPDisplayKey(value)
}

func isSensitiveMCPDisplayKey(key string) bool {
	key = strings.ToLower(strings.TrimSpace(strings.TrimLeft(key, "-")))
	key = strings.ReplaceAll(key, "-", "_")
	if key == "key" {
		return true
	}
	for _, token := range []string{"token", "secret", "password", "passwd", "api_key", "apikey", "access_key", "auth", "credential"} {
		if strings.Contains(key, token) {
			return true
		}
	}
	return false
}
