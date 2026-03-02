// Package pricing maps LLM model names to their per-token costs and provides
// token-count estimation for dry-run cost projections.
package pricing

import "strings"

// ModelPrice holds the USD cost per million tokens for a model.
type ModelPrice struct {
	InputPerMTok  float64
	OutputPerMTok float64
	PriceURL      string // canonical pricing page for this provider
}

// Estimate returns the projected USD cost for the given token counts.
func (p ModelPrice) Estimate(inputTokens, outputTokens int) float64 {
	return float64(inputTokens)/1e6*p.InputPerMTok +
		float64(outputTokens)/1e6*p.OutputPerMTok
}

// PromptOverheadTokens is the approximate number of input tokens added to every
// LLM request beyond the raw file content: system prompt + file header + delimiters.
const PromptOverheadTokens = 250

// SummaryOutputTokens is the estimated number of output tokens per generated summary.
const SummaryOutputTokens = 150

// known maps model-name prefixes to pricing data.
// Prices are in USD per million tokens (MTok).
// Anthropic prices verified March 2026 via https://docs.anthropic.com/en/docs/about-claude/models/overview
// OpenAI prices verified March 2026 via https://platform.openai.com/docs/pricing
var known = map[string]ModelPrice{
	// ── Anthropic ──────────────────────────────────────────────────────────────
	// Opus 4.x pricing changed across minor versions — longer prefixes take
	// priority so minor-version-specific entries shadow the catch-all:
	//   4.0 ($15/$75) and 4.1 ($15/$75) are legacy premium releases.
	//   4.5 ($5/$25) and 4.6 ($5/$25) are the current lower-priced releases.
	// The "claude-opus-4" catch-all covers 4.0 and any unrecognised minor versions.
	"claude-opus-4-6":   {5.00, 25.00, "https://docs.anthropic.com/en/docs/about-claude/pricing"},
	"claude-opus-4-5":   {5.00, 25.00, "https://docs.anthropic.com/en/docs/about-claude/pricing"},
	"claude-opus-4-1":   {15.00, 75.00, "https://docs.anthropic.com/en/docs/about-claude/pricing"},
	"claude-opus-4":     {15.00, 75.00, "https://docs.anthropic.com/en/docs/about-claude/pricing"},
	"claude-sonnet-4":   {3.00, 15.00, "https://docs.anthropic.com/en/docs/about-claude/pricing"},
	"claude-haiku-4":    {1.00, 5.00, "https://docs.anthropic.com/en/docs/about-claude/pricing"},
	"claude-3-5-sonnet": {3.00, 15.00, "https://docs.anthropic.com/en/docs/about-claude/pricing"},
	"claude-3-5-haiku":  {0.80, 4.00, "https://docs.anthropic.com/en/docs/about-claude/pricing"},
	"claude-3-opus":     {15.00, 75.00, "https://docs.anthropic.com/en/docs/about-claude/pricing"},
	"claude-3-haiku":    {0.25, 1.25, "https://docs.anthropic.com/en/docs/about-claude/pricing"},

	// ── OpenAI ─────────────────────────────────────────────────────────────────
	// Longer prefixes take priority: "gpt-5-mini" and "gpt-5-nano" shadow "gpt-5".
	"gpt-5.2":      {1.75, 14.00, "https://platform.openai.com/docs/pricing"},
	"gpt-5-mini":   {0.25, 2.00, "https://platform.openai.com/docs/pricing"},
	"gpt-5-nano":   {0.05, 0.40, "https://platform.openai.com/docs/pricing"},
	"gpt-5":        {1.25, 10.00, "https://platform.openai.com/docs/pricing"},
	"gpt-4.1-mini": {0.40, 1.60, "https://platform.openai.com/docs/pricing"},
	"gpt-4.1-nano": {0.10, 0.40, "https://platform.openai.com/docs/pricing"},
	"gpt-4.1":      {2.00, 8.00, "https://platform.openai.com/docs/pricing"},
	"gpt-4o-mini":  {0.15, 0.60, "https://platform.openai.com/docs/pricing"},
	"gpt-4o":       {2.50, 10.00, "https://platform.openai.com/docs/pricing"},
	"o4-mini":      {1.10, 4.40, "https://platform.openai.com/docs/pricing"},
	"o3-mini":      {1.10, 4.40, "https://platform.openai.com/docs/pricing"},
	"o3":           {0.40, 1.60, "https://platform.openai.com/docs/pricing"},
	"o1-mini":      {3.00, 12.00, "https://platform.openai.com/docs/pricing"},
	"o1":           {15.00, 60.00, "https://platform.openai.com/docs/pricing"},
}

// ForModel returns pricing for the given model, matched by longest prefix.
// Returns (ModelPrice{}, false) when the model is not recognised.
func ForModel(model string) (ModelPrice, bool) {
	var best string
	var price ModelPrice
	for prefix, p := range known {
		if strings.HasPrefix(model, prefix) && len(prefix) > len(best) {
			best = prefix
			price = p
		}
	}
	return price, best != ""
}

// EstimateInputTokens returns the approximate number of input tokens for a
// single LLM request given the stripped file content. It adds PromptOverheadTokens
// to account for the system prompt, file header, and delimiters.
// Code averages ~4 characters per token.
func EstimateInputTokens(strippedContent []byte) int {
	return (len(strippedContent)+3)/4 + PromptOverheadTokens
}
