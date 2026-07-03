package config

import (
	"fmt"
	"strings"
)

type Profile struct {
	Provider          string
	Model             string
	APIKey            string
	BaseURL           string
	MaxTokens         int
	ReserveTokens     int
	Temperature       float64
	AutoApprove       bool
	AllowedTools      []string
	AllowedPaths      []string
	MaxToolIterations int
}

func (p Profile) IsAllowed(toolName string, path string) bool {
	if p.AutoApprove {
		return true
	}
	for _, t := range p.AllowedTools {
		if t == toolName {
			return true
		}
	}
	if path != "" {
		for _, allowedPath := range p.AllowedPaths {
			// Basic prefix match. Make sure they are cleaned absolute paths
			if strings.HasPrefix(path, allowedPath) {
				return true
			}
		}
	}
	return false
}

func (c *Config) Profile(name string) (Profile, error) {
	profile, ok := c.Profiles[name]
	if !ok {
		return Profile{}, fmt.Errorf("profile %q not found", name)
	}
	return profile, nil
}

func (c *Config) Active() (Profile, error) {
	return c.Profile(c.ActiveProfile)
}

func (c *Config) UpdateActive(update func(*Profile)) error {
	if c.Profiles == nil {
		c.Profiles = map[string]Profile{}
	}

	profile := c.Profiles[c.ActiveProfile]
	update(&profile)
	c.Profiles[c.ActiveProfile] = profile
	return nil
}
