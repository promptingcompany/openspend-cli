package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
)

var defaultBaseURL = "https://openspend.ai"
var deprecatedDefaultBaseURLs = map[string]struct{}{
	"http://192.0.0.2:5566": {},
	"http://localhost:5555": {},
}

type MarketplaceConfig struct {
	BaseURL        string `toml:"base_url"`
	WhoAmIPath     string `toml:"whoami_path"`
	PolicyInitPath string `toml:"policy_init_path"`
	AgentPath      string `toml:"agent_path"`
}

type AuthConfig struct {
	BrowserLoginPath string `toml:"browser_login_path"`
	SessionToken     string `toml:"session_token,omitempty"`
	SessionCookie    string `toml:"session_cookie"`
}

type Config struct {
	Marketplace MarketplaceConfig `toml:"marketplace"`
	Auth        AuthConfig        `toml:"auth"`
}

func defaults() Config {
	return Config{
		Marketplace: MarketplaceConfig{
			BaseURL:        defaultBaseURL,
			WhoAmIPath:     "/api/cli/whoami",
			PolicyInitPath: "/api/cli/policy/init",
			AgentPath:      "/api/cli/agent",
		},
		Auth: AuthConfig{
			BrowserLoginPath: "/api/cli/auth/login",
			SessionCookie:    "better-auth.session_token",
		},
	}
}

func configPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "openspend", "config.toml"), nil
}

func Load() (Config, error) {
	path, err := configPath()
	if err != nil {
		return Config{}, err
	}

	cfg := defaults()
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			legacyToml, legacyTomlErr := loadLegacyToml()
			if legacyTomlErr == nil {
				ApplyEnvOverrides(&legacyToml)
				applyDefaults(&legacyToml)
				_ = Save(legacyToml)
				return legacyToml, nil
			}
			legacy, legacyErr := loadLegacyJSON()
			if legacyErr == nil {
				ApplyEnvOverrides(&legacy)
				applyDefaults(&legacy)
				_ = Save(legacy)
				return legacy, nil
			}
			ApplyEnvOverrides(&cfg)
			applyDefaults(&cfg)
			return cfg, nil
		}
		return Config{}, err
	}

	if err := toml.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}
	ApplyEnvOverrides(&cfg)
	applyDefaults(&cfg)
	return cfg, nil
}

func Save(cfg Config) error {
	path, err := configPath()
	if err != nil {
		return err
	}
	applyDefaults(&cfg)

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	data, err := toml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// ApplyEnvOverrides applies runtime environment overrides to loaded config.
func ApplyEnvOverrides(cfg *Config) {
	if cfg == nil {
		return
	}
	if v := os.Getenv("OPENSPEND_MARKETPLACE_BASE_URL"); v != "" {
		cfg.Marketplace.BaseURL = v
	} else if v := os.Getenv("OPENSPEND_BASE_URL"); v != "" {
		cfg.Marketplace.BaseURL = v
	}
	if v := os.Getenv("OPENSPEND_MARKETPLACE_WHOAMI_PATH"); v != "" {
		cfg.Marketplace.WhoAmIPath = v
	}
	if v := os.Getenv("OPENSPEND_MARKETPLACE_POLICY_INIT_PATH"); v != "" {
		cfg.Marketplace.PolicyInitPath = v
	}
	if v := os.Getenv("OPENSPEND_MARKETPLACE_AGENT_PATH"); v != "" {
		cfg.Marketplace.AgentPath = v
	}
	if v := os.Getenv("OPENSPEND_AUTH_BROWSER_LOGIN_PATH"); v != "" {
		cfg.Auth.BrowserLoginPath = v
	}
	if v := os.Getenv("OPENSPEND_AUTH_SESSION_COOKIE"); v != "" {
		cfg.Auth.SessionCookie = v
	}
}

func applyDefaults(cfg *Config) {
	def := defaults()

	if cfg.Marketplace.BaseURL == "" {
		cfg.Marketplace.BaseURL = def.Marketplace.BaseURL
	} else if _, isDeprecatedDefault := deprecatedDefaultBaseURLs[cfg.Marketplace.BaseURL]; isDeprecatedDefault {
		// Legacy versions wrote local development URLs as defaults. Normalize to the public endpoint.
		cfg.Marketplace.BaseURL = def.Marketplace.BaseURL
	}
	if cfg.Marketplace.WhoAmIPath == "" {
		cfg.Marketplace.WhoAmIPath = def.Marketplace.WhoAmIPath
	}
	if cfg.Marketplace.PolicyInitPath == "" {
		cfg.Marketplace.PolicyInitPath = def.Marketplace.PolicyInitPath
	}
	if cfg.Marketplace.AgentPath == "" {
		cfg.Marketplace.AgentPath = def.Marketplace.AgentPath
	}
	if cfg.Auth.BrowserLoginPath == "" {
		cfg.Auth.BrowserLoginPath = def.Auth.BrowserLoginPath
	}
	if cfg.Auth.SessionCookie == "" {
		cfg.Auth.SessionCookie = def.Auth.SessionCookie
	}
}

func legacyTomlPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".openspend", "config.toml"), nil
}

func loadLegacyToml() (Config, error) {
	path, err := legacyTomlPath()
	if err != nil {
		return Config{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	cfg := defaults()
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func legacyConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".openspend", "config.json"), nil
}

func loadLegacyJSON() (Config, error) {
	path, err := legacyConfigPath()
	if err != nil {
		return Config{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}

	type legacy struct {
		BaseURL      string `json:"base_url"`
		SessionToken string `json:"session_token,omitempty"`
	}
	var legacyCfg legacy
	if err := json.Unmarshal(data, &legacyCfg); err != nil {
		return Config{}, err
	}

	cfg := defaults()
	if legacyCfg.BaseURL != "" {
		cfg.Marketplace.BaseURL = legacyCfg.BaseURL
	}
	cfg.Auth.SessionToken = legacyCfg.SessionToken
	return cfg, nil
}
