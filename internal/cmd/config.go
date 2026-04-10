package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/alexfsong/jeeves/internal/config"
	"github.com/alexfsong/jeeves/internal/render"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage Jeeves configuration",
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		r := render.New(jsonOutput)
		if r.IsJSON() {
			r.JSON(cfg)
			return nil
		}

		r.Title("Configuration")
		r.Divider()

		// Mask API keys
		r.Header("[llm]")
		r.Body(fmt.Sprintf("  local_provider = %q", cfg.LLM.LocalProvider))
		r.Body(fmt.Sprintf("  local_model    = %q", cfg.LLM.LocalModel))
		r.Body(fmt.Sprintf("  local_endpoint = %q", cfg.LLM.LocalEndpoint))
		r.Body(fmt.Sprintf("  cloud_provider = %q", cfg.LLM.CloudProvider))
		r.Body(fmt.Sprintf("  cloud_model    = %q", cfg.LLM.CloudModel))
		r.Body(fmt.Sprintf("  cloud_api_key  = %s", maskKey(cfg.LLM.CloudAPIKey)))
		if cfg.LLM.ForceProvider != "" {
			r.Body(fmt.Sprintf("  force_provider = %q", cfg.LLM.ForceProvider))
		}

		r.Header("[search]")
		r.Body(fmt.Sprintf("  provider        = %q", cfg.Search.Provider))
		r.Body(fmt.Sprintf("  brave_api_key   = %s", maskKey(cfg.Search.BraveAPIKey)))
		r.Body(fmt.Sprintf("  searxng_endpoint= %q", cfg.Search.SearXNGEndpoint))
		r.Body(fmt.Sprintf("  tavily_api_key  = %s", maskKey(cfg.Search.TavilyAPIKey)))

		r.Header("[defaults]")
		r.Body(fmt.Sprintf("  resolution = %q", cfg.Defaults.Resolution))
		return nil
	},
}

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a configuration value",
	Long:  "Set a configuration value. Keys use dot notation: search.brave_api_key, llm.cloud_model, defaults.resolution",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		key, value := args[0], args[1]

		cfg, err := config.Load()
		if err != nil {
			return err
		}

		switch key {
		case "llm.local_provider":
			cfg.LLM.LocalProvider = value
		case "llm.local_model":
			cfg.LLM.LocalModel = value
		case "llm.local_endpoint":
			cfg.LLM.LocalEndpoint = value
		case "llm.cloud_provider":
			cfg.LLM.CloudProvider = value
		case "llm.cloud_model":
			cfg.LLM.CloudModel = value
		case "llm.cloud_api_key":
			cfg.LLM.CloudAPIKey = value
		case "llm.force_provider":
			cfg.LLM.ForceProvider = value
		case "search.provider":
			cfg.Search.Provider = value
		case "search.brave_api_key":
			cfg.Search.BraveAPIKey = value
		case "search.searxng_endpoint":
			cfg.Search.SearXNGEndpoint = value
		case "search.tavily_api_key":
			cfg.Search.TavilyAPIKey = value
		case "defaults.resolution":
			cfg.Defaults.Resolution = value
		default:
			return fmt.Errorf("unknown config key %q", key)
		}

		path, err := config.Path()
		if err != nil {
			return err
		}

		f, err := os.Create(path)
		if err != nil {
			return fmt.Errorf("writing config: %w", err)
		}
		defer f.Close()

		if err := toml.NewEncoder(f).Encode(cfg); err != nil {
			return fmt.Errorf("encoding config: %w", err)
		}

		r := render.New(jsonOutput)
		if r.IsJSON() {
			r.JSON(map[string]string{"status": "ok", "key": key, "value": value})
		} else {
			r.Success(fmt.Sprintf("Very good, sir. %s set to %q.", key, value))
		}
		return nil
	},
}

func init() {
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configSetCmd)
	rootCmd.AddCommand(configCmd)
}

func maskKey(key string) string {
	if key == "" {
		return "(not set)"
	}
	if len(key) <= 8 {
		return strings.Repeat("*", len(key))
	}
	return key[:4] + strings.Repeat("*", len(key)-8) + key[len(key)-4:]
}
