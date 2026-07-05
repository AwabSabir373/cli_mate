package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/viper"
	"go.yaml.in/yaml/v3"

	"cli_mate/pkg/crypto"
)

type Config struct {
	ActiveProfile string
	Profiles      map[string]Profile
	Log           LogConfig
	Storage       StorageConfig
	HTTP          HTTPConfig
	MCP           []MCPConfig
	path          string
}

type MCPConfig struct {
	Name    string
	Command string
	Args    []string
	Env     map[string]string
}

type LogConfig struct {
	Level string
}

type StorageConfig struct {
	Path string
}

type HTTPConfig struct {
	Timeout time.Duration
	Retries int
}

func Load(path string, profileName string) (*Config, error) {
	v := viper.New()
	v.SetConfigName("cli_mate")
	v.SetConfigType("yaml")
	v.AddConfigPath(".")
	if home, err := os.UserHomeDir(); err == nil {
		v.AddConfigPath(filepath.Join(home, ".config", "cli_mate"))
	}

	if path != "" {
		v.SetConfigFile(path)
	}

	v.SetEnvPrefix("CLI_MATE")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	setDefaults(v)
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok && !os.IsNotExist(err) {
			return nil, err
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	// Decrypt API keys on load. Plaintext legacy values pass through unchanged.
	for name, profile := range cfg.Profiles {
		if profile.APIKey != "" {
			decrypted, err := crypto.Decrypt(profile.APIKey)
			if err != nil {
				return nil, fmt.Errorf("config: decrypt api key for profile %q: %w", name, err)
			}
			profile.APIKey = decrypted
			cfg.Profiles[name] = profile
		}
	}

	if profileName == "" {
		profileName = v.GetString("activeProfile")
	}
	cfg.ActiveProfile = profileName
	ensureDefaults(&cfg)
	cfg.path = configPath(path, v.ConfigFileUsed())
	return &cfg, nil
}

func (c *Config) Save() error {
	if c == nil || c.path == "" {
		return nil
	}
	ensureDefaults(c)

	if err := os.MkdirAll(filepath.Dir(c.path), 0700); err != nil {
		return err
	}

	data, err := yaml.Marshal(persistedConfigFrom(c))
	if err != nil {
		return err
	}
	return os.WriteFile(c.path, data, 0600)
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("activeProfile", "default")
	v.SetDefault("log.level", "info")
	v.SetDefault("storage.path", filepath.Join(".cli_mate", "cli_mate.db"))
	v.SetDefault("http.timeout", "30s")
	v.SetDefault("http.retries", 3)
	v.SetDefault("profiles.default.provider", "ollama")
	v.SetDefault("profiles.default.model", "llama3.1")
	v.SetDefault("profiles.default.maxTokens", 128000)
	v.SetDefault("profiles.default.reserveTokens", 4096)
	v.SetDefault("profiles.default.temperature", 0.2)
	v.SetDefault("profiles.default.maxToolIterations", 32)
}

func ensureDefaults(cfg *Config) {
	if cfg.ActiveProfile == "" {
		cfg.ActiveProfile = "default"
	}
	if cfg.Profiles == nil {
		cfg.Profiles = map[string]Profile{}
	}

	profile := cfg.Profiles[cfg.ActiveProfile]
	if profile.Provider == "" {
		profile.Provider = "ollama"
	}
	if profile.Model == "" {
		profile.Model = "llama3.1"
	}
	if profile.Provider == "ollama" && profile.BaseURL == "" {
		profile.BaseURL = "http://localhost:11434"
	}
	if profile.MaxTokens == 0 {
		profile.MaxTokens = 128000
	}
	if profile.ReserveTokens == 0 {
		profile.ReserveTokens = 4096
	}
	if profile.Temperature == 0 {
		profile.Temperature = 0.2
	}
	if profile.MaxToolIterations <= 0 {
		profile.MaxToolIterations = 32
	}
	cfg.Profiles[cfg.ActiveProfile] = profile

	if cfg.Log.Level == "" {
		cfg.Log.Level = "info"
	}
	if cfg.Storage.Path == "" {
		cfg.Storage.Path = filepath.Join(".cli_mate", "cli_mate.db")
	}
	if cfg.HTTP.Timeout == 0 {
		cfg.HTTP.Timeout = 30 * time.Second
	}
	if cfg.HTTP.Retries == 0 {
		cfg.HTTP.Retries = 3
	}

	// Inject default built-in MCP server for project context
	hasCliMCP := false
	for _, mcp := range cfg.MCP {
		if mcp.Name == "cli_mcp" {
			hasCliMCP = true
			break
		}
	}
	if !hasCliMCP {
		// Use os.Executable to get the path of the current binary
		exe, err := os.Executable()
		if err == nil {
			cfg.MCP = append(cfg.MCP, MCPConfig{
				Name:    "cli_mcp",
				Command: exe,
				Args:    []string{"mcp-server"},
			})
		}
	}
}

func configPath(explicit string, used string) string {
	if explicit != "" {
		return explicit
	}
	if used != "" {
		return used
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "cli_mate.yaml"
	}
	return filepath.Join(home, ".config", "cli_mate", "cli_mate.yaml")
}

type persistedConfig struct {
	ActiveProfile string                      `yaml:"activeProfile"`
	Profiles      map[string]persistedProfile `yaml:"profiles"`
	Log           LogConfig                   `yaml:"log"`
	Storage       StorageConfig               `yaml:"storage"`
	HTTP          persistedHTTPConfig         `yaml:"http"`
}

type persistedProfile struct {
	Provider          string   `yaml:"provider"`
	Model             string   `yaml:"model"`
	APIKey            string   `yaml:"apiKey,omitempty"`
	BaseURL           string   `yaml:"baseURL,omitempty"`
	MaxTokens         int      `yaml:"maxTokens"`
	ReserveTokens     int      `yaml:"reserveTokens"`
	Temperature       float64  `yaml:"temperature"`
	AutoApprove       bool     `yaml:"autoApprove,omitempty"`
	AllowedTools      []string `yaml:"allowedTools,omitempty"`
	AllowedPaths      []string `yaml:"allowedPaths,omitempty"`
	MaxToolIterations int      `yaml:"maxToolIterations,omitempty"`
}

type persistedHTTPConfig struct {
	Timeout string `yaml:"timeout"`
	Retries int    `yaml:"retries"`
}

func persistedConfigFrom(cfg *Config) persistedConfig {
	profiles := make(map[string]persistedProfile, len(cfg.Profiles))
	for name, profile := range cfg.Profiles {
		apiKey := profile.APIKey
		if apiKey != "" && !crypto.IsEncrypted(apiKey) {
			if enc, err := crypto.Encrypt(apiKey); err == nil {
				apiKey = enc
			}
		}
		profiles[name] = persistedProfile{
			Provider:          profile.Provider,
			Model:             profile.Model,
			APIKey:            apiKey,
			BaseURL:           profile.BaseURL,
			MaxTokens:         profile.MaxTokens,
			ReserveTokens:     profile.ReserveTokens,
			Temperature:       profile.Temperature,
			AutoApprove:       profile.AutoApprove,
			AllowedTools:      profile.AllowedTools,
			AllowedPaths:      profile.AllowedPaths,
			MaxToolIterations: profile.MaxToolIterations,
		}
	}

	return persistedConfig{
		ActiveProfile: cfg.ActiveProfile,
		Profiles:      profiles,
		Log:           cfg.Log,
		Storage:       cfg.Storage,
		HTTP: persistedHTTPConfig{
			Timeout: cfg.HTTP.Timeout.String(),
			Retries: cfg.HTTP.Retries,
		},
	}
}
