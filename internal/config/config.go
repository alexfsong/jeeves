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
	Verify   VerifyConfig   `toml:"verify"`
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

// VerifyConfig controls the optional post-synthesis verification pass that
// invokes Anthropic's native web_search / web_fetch tools to verify claims
// and fill gaps in the synthesis. Cloud-only, opt-in, strictly budgeted.
type VerifyConfig struct {
	Enabled        bool     `toml:"enabled"`
	MaxSearchUses  int      `toml:"max_search_uses"`
	MaxFetchUses   int      `toml:"max_fetch_uses"`
	MaxNewSources  int      `toml:"max_new_sources"`
	MaxTokens      int      `toml:"max_tokens"`
	AllowedDomains []string `toml:"allowed_domains"`
	BlockedDomains []string `toml:"blocked_domains"`
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
		Verify: VerifyConfig{
			Enabled:       false,
			MaxSearchUses: 3,
			MaxFetchUses:  3,
			MaxNewSources: 5,
			MaxTokens:     2048,
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
		if !cfg.Verify.Enabled {
			if v := os.Getenv("JEEVES_VERIFY"); v == "1" || v == "true" {
				cfg.Verify.Enabled = true
			}
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

[verify]
# Opt-in post-synthesis verification pass using Anthropic's native
# web_search + web_fetch tools. Requires cloud LLM (Anthropic).
# enabled = false                 # or set JEEVES_VERIFY=1 env var
# max_search_uses = 3             # cap on web_search tool invocations
# max_fetch_uses = 3              # cap on web_fetch tool invocations
# max_new_sources = 5             # cap on verification-discovered sources persisted
# max_tokens = 2048
# allowed_domains = []            # optional whitelist (web_search + web_fetch)
# blocked_domains = []            # optional blocklist (mutually exclusive with allowed_domains)
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
