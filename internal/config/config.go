package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

type Config struct {
	LLM      LLMConfig      `toml:"llm"`
	Search   SearchConfig   `toml:"search"`
	Defaults DefaultsConfig `toml:"defaults"`
}

type LLMConfig struct {
	LocalProvider string `toml:"local_provider"`
	LocalModel    string `toml:"local_model"`
	LocalEndpoint string `toml:"local_endpoint"`
	CloudProvider string `toml:"cloud_provider"`
	CloudModel    string `toml:"cloud_model"`
	CloudAPIKey   string `toml:"cloud_api_key"`
	ForceProvider string `toml:"force_provider"`
}

type SearchConfig struct {
	Provider        string `toml:"provider"`
	BraveAPIKey     string `toml:"brave_api_key"`
	SearXNGEndpoint string `toml:"searxng_endpoint"`
	TavilyAPIKey    string `toml:"tavily_api_key"`
}

type DefaultsConfig struct {
	Resolution string `toml:"resolution"`
}

func DefaultConfig() Config {
	return Config{
		LLM: LLMConfig{
			LocalProvider: "ollama",
			LocalModel:    "llama3",
			LocalEndpoint: "http://localhost:11434",
			CloudProvider: "anthropic",
			CloudModel:    "claude-sonnet-4-20250514",
		},
		Search: SearchConfig{
			Provider: "brave",
		},
		Defaults: DefaultsConfig{
			Resolution: "brief",
		},
	}
}

func Dir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("unable to determine home directory: %w", err)
	}
	return filepath.Join(home, ".jeeves"), nil
}

func Path() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.toml"), nil
}

func Load() (Config, error) {
	cfg := DefaultConfig()

	path, err := Path()
	if err != nil {
		return cfg, err
	}

	// Override cloud API key from env if not set in config
	defer func() {
		if cfg.LLM.CloudAPIKey == "" {
			cfg.LLM.CloudAPIKey = os.Getenv("ANTHROPIC_API_KEY")
		}
		if cfg.Search.BraveAPIKey == "" {
			cfg.Search.BraveAPIKey = os.Getenv("BRAVE_API_KEY")
		}
		if cfg.Search.TavilyAPIKey == "" {
			cfg.Search.TavilyAPIKey = os.Getenv("TAVILY_API_KEY")
		}
	}()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, fmt.Errorf("reading config: %w", err)
	}

	if err := toml.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parsing config: %w", err)
	}

	return cfg, nil
}

const defaultConfigTemplate = `# Jeeves configuration
# See: jeeves config show

[llm]
# local_provider = "ollama"
# local_model = "llama3"
# local_endpoint = "http://localhost:11434"
# cloud_provider = "anthropic"
# cloud_model = "claude-sonnet-4-20250514"
# cloud_api_key = ""              # or set ANTHROPIC_API_KEY env var
# force_provider = ""             # "local", "cloud", or "" for auto

[search]
# provider = "brave"              # "brave", "searxng", "tavily"
# brave_api_key = ""              # or set BRAVE_API_KEY env var
# searxng_endpoint = ""
# tavily_api_key = ""             # or set TAVILY_API_KEY env var

[defaults]
# resolution = "brief"
`

func EnsureDir() (string, bool, error) {
	dir, err := Dir()
	if err != nil {
		return "", false, err
	}

	if _, err := os.Stat(dir); err == nil {
		return dir, false, nil
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", false, fmt.Errorf("creating jeeves directory: %w", err)
	}

	configPath := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(configPath, []byte(defaultConfigTemplate), 0644); err != nil {
		return "", false, fmt.Errorf("writing default config: %w", err)
	}

	return dir, true, nil
}
