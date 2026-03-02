package llm

import (
	"fmt"

	"github.com/tristanmatthias/llmdoc/internal/config"
)

// NewProvider creates an LLM Provider based on the configuration.
// Returns an error if the provider is unknown or the API key is missing.
func NewProvider(cfg *config.Config) (Provider, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("no API key configured for provider %q — set api_key in .llmdoc.yaml or use the ANTHROPIC_API_KEY / OPENAI_API_KEY environment variable", cfg.Provider)
	}

	switch cfg.Provider {
	case "anthropic":
		return NewAnthropicProvider(cfg.APIKey, cfg.Model), nil
	case "openai":
		return NewOpenAIProvider(cfg.APIKey, cfg.Model), nil
	default:
		return nil, fmt.Errorf("unknown provider %q — supported providers: anthropic, openai", cfg.Provider)
	}
}
