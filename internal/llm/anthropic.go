package llm

import (
	"context"
	"fmt"
	"net/http"
)

const (
	anthropicAPIURL  = "https://api.anthropic.com/v1/messages"
	anthropicVersion = "2023-06-01"
)

// AnthropicProvider implements Provider using the Anthropic Messages API.
type AnthropicProvider struct {
	baseClient
}

func NewAnthropicProvider(apiKey, model string) *AnthropicProvider {
	return &AnthropicProvider{newBaseClient(apiKey, model)}
}

type anthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	System    string             `json:"system"`
	Messages  []anthropicMessage `json:"messages"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Usage struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

func (p *AnthropicProvider) Summarize(ctx context.Context, req SummaryRequest) (string, TokenUsage, error) {
	payload := anthropicRequest{
		Model:     p.model,
		MaxTokens: 512,
		System:    systemPrompt,
		Messages:  []anthropicMessage{{Role: "user", Content: buildPrompt(req)}},
	}

	var result anthropicResponse
	rawBody, status, err := p.postJSON(ctx, anthropicAPIURL, payload, &result, func(r *http.Request) {
		r.Header.Set("x-api-key", p.apiKey)
		r.Header.Set("anthropic-version", anthropicVersion)
	})
	if err != nil {
		return "", TokenUsage{}, err
	}

	if result.Error != nil {
		return "", TokenUsage{}, fmt.Errorf("anthropic API error (%s): %s", result.Error.Type, result.Error.Message)
	}
	if status != http.StatusOK {
		return "", TokenUsage{}, fmt.Errorf("anthropic API returned HTTP %d: %s", status, rawBody)
	}

	usage := TokenUsage{InputTokens: result.Usage.InputTokens, OutputTokens: result.Usage.OutputTokens}
	for _, c := range result.Content {
		if c.Type == "text" {
			return c.Text, usage, nil
		}
	}
	return "", usage, fmt.Errorf("no text content in response")
}
