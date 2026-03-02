package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
	"gopkg.in/yaml.v3"
)

// Config holds all llmdoc configuration.
type Config struct {
	// LLM provider
	Provider string `yaml:"provider"`
	APIKey   string `yaml:"api_key"`
	Model    string `yaml:"model"`

	// File scanning
	Extensions []string `yaml:"extensions"`
	Ignore     []string `yaml:"ignore"`

	// Storage mode: "inline" (default) or "index"
	// inline: summaries stored as comment headers inside each source file
	// index:  summaries stored in a separate index file; source files never modified
	Mode      string `yaml:"mode"`
	IndexFile string `yaml:"index_file"` // only used when mode: index

	// Behaviour
	Concurrency int  `yaml:"concurrency"`
	Force       bool `yaml:"force"`
}

var defaults = Config{
	Provider:  "anthropic",
	Model:     "claude-sonnet-4-6",
	Mode:      "index",
	IndexFile: ".llmdoc/index.yaml",
	Extensions: []string{
		".go", ".ts", ".tsx", ".js", ".jsx",
		".py", ".rs", ".java", ".rb", ".swift", ".kt",
		".c", ".cpp", ".h", ".cs", ".php",
		".sh", ".bash", ".zsh",
		".sql", ".lua",
	},
	Ignore: []string{
		"vendor/", "node_modules/", ".git/",
		"dist/", "build/", "**/*.min.js",
		"**/*.pb.go", "**/*.generated.go",
		".llmdoc/", ".llmdoc.yaml", ".llmdoc.yml",
	},
	Concurrency: 4,
}

// Load reads configuration from the given path (or auto-discovers it), then
// applies defaults and resolves the API key from environment variables.
// A .env file in the current directory is loaded first; its values are applied
// to the environment only when the variable is not already set by the shell.
func Load(cfgPath string) (*Config, error) {
	loadDotEnv(".env")
	cfg := defaults

	paths := configPaths(cfgPath)
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("reading config %s: %w", p, err)
		}
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return nil, fmt.Errorf("parsing config %s: %w", p, err)
		}
		applyDefaults(&cfg)
		break // use first found config file
	}

	resolveAPIKey(&cfg)
	return &cfg, nil
}

// configPaths returns the ordered list of config file paths to try.
func configPaths(explicit string) []string {
	if explicit != "" {
		return []string{explicit}
	}
	paths := []string{".llmdoc.yaml", ".llmdoc.yml"}
	if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths,
			filepath.Join(home, ".config", "llmdoc", "config.yaml"),
			filepath.Join(home, ".config", "llmdoc", "config.yml"),
		)
	}
	return paths
}

// applyDefaults fills zero-valued fields in cfg from the package-level defaults.
// This is called after YAML unmarshalling so that fields explicitly set to ""
// or 0 in the config file still get a sensible value.
func applyDefaults(cfg *Config) {
	if cfg.Provider == "" {
		cfg.Provider = defaults.Provider
	}
	if cfg.Model == "" {
		cfg.Model = defaults.Model
	}
	if len(cfg.Extensions) == 0 {
		cfg.Extensions = defaults.Extensions
	}
	if len(cfg.Ignore) == 0 {
		cfg.Ignore = defaults.Ignore
	}
	if cfg.Concurrency == 0 {
		cfg.Concurrency = defaults.Concurrency
	}
	if cfg.Mode == "" {
		cfg.Mode = defaults.Mode
	}
	if cfg.IndexFile == "" {
		cfg.IndexFile = defaults.IndexFile
	}
}

// resolveAPIKey fills cfg.APIKey from environment variables when not set in config.
// Provider-specific variables take precedence over the generic LLMDOC_API_KEY fallback.
func resolveAPIKey(cfg *Config) {
	if cfg.APIKey != "" {
		return
	}
	switch cfg.Provider {
	case "anthropic":
		cfg.APIKey = os.Getenv("ANTHROPIC_API_KEY")
	case "openai":
		cfg.APIKey = os.Getenv("OPENAI_API_KEY")
	}
	if cfg.APIKey == "" {
		cfg.APIKey = os.Getenv("LLMDOC_API_KEY")
	}
}

// loadDotEnv reads a .env file and injects any variables not already set in
// the environment. Uses godotenv so existing shell variables always win.
func loadDotEnv(path string) {
	godotenv.Load(path) // silently ignores missing file
}

// StarterYAML returns a sample .llmdoc.yaml content.
func StarterYAML() string {
	return `# llmdoc configuration
# https://github.com/tristanmatthias/llmdoc

# LLM provider: "anthropic" or "openai"
provider: anthropic

# API key (recommended: use env var ANTHROPIC_API_KEY or OPENAI_API_KEY instead)
api_key: ""

# Model to use for generating summaries
model: claude-haiku-4-5

# File extensions to annotate
extensions:
  - .go
  - .ts
  - .tsx
  - .js
  - .jsx
  - .py
  - .rs
  - .java
  - .rb
  - .swift

# Ignore patterns (gitignore-style, relative to project root)
ignore:
  - vendor/
  - node_modules/
  - .git/
  - dist/
  - build/
  - "**/*.min.js"
  - "**/*.pb.go"
  - "**/*.generated.go"
  - .llmdoc/
  - .llmdoc.yaml
  - .llmdoc.yml

# Number of concurrent LLM calls (default: 4)
concurrency: 4

# Storage mode:
#   inline - summaries stored as comment headers at the top of each source file
#   index  - summaries stored in a separate index file; source files never modified
mode: index

# Index file path (only used when mode: index)
index_file: .llmdoc/index.yaml

# Force re-annotation even when file hash is unchanged
force: false
`
}
