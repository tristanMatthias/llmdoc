package llm

import (
	"context"
	"fmt"
	"net/http"
)

const openaiAPIURL = "https://api.openai.com/v1/chat/completions"

// OpenAIProvider implements Provider using the OpenAI Chat Completions API.
type OpenAIProvider struct {
	baseClient
}

func NewOpenAIProvider(apiKey, model string) *OpenAIProvider {
	return &OpenAIProvider{newBaseClient(apiKey, model)}
}

type openaiRequest struct {
	Model     string          `json:"model"`
	MaxTokens int             `json:"max_tokens"`
	Messages  []openaiMessage `json:"messages"`
}

type openaiMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openaiResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

func (p *OpenAIProvider) Summarize(ctx context.Context, req SummaryRequest) (string, TokenUsage, error) {
	return withRetry(ctx, func() (string, TokenUsage, error) {
		return p.summarizeOnce(ctx, req)
	})
}

func (p *OpenAIProvider) summarizeOnce(ctx context.Context, req SummaryRequest) (string, TokenUsage, error) {
	payload := openaiRequest{
		Model:     p.model,
		MaxTokens: 512,
		Messages: []openaiMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: buildPrompt(req)},
		},
	}

	var result openaiResponse
	rawBody, status, headers, err := p.postJSON(ctx, openaiAPIURL, payload, &result, func(r *http.Request) {
		r.Header.Set("Authorization", "Bearer "+p.apiKey)
	})
	if err != nil {
		return "", TokenUsage{}, err
	}

	if result.Error != nil {
		apiErr := fmt.Errorf("openai API error (%s): %s", result.Error.Type, result.Error.Message)
		if result.Error.Type == "rate_limit_exceeded" {
			return "", TokenUsage{}, retryableError{cause: apiErr, wait: parseRetryAfter(headers)}
		}
		return "", TokenUsage{}, apiErr
	}
	if status == http.StatusTooManyRequests {
		return "", TokenUsage{}, retryableError{
			cause: fmt.Errorf("openai API returned HTTP %d: %s", status, rawBody),
			wait:  parseRetryAfter(headers),
		}
	}
	if status != http.StatusOK {
		return "", TokenUsage{}, fmt.Errorf("openai API returned HTTP %d: %s", status, rawBody)
	}
	if len(result.Choices) == 0 {
		return "", TokenUsage{}, fmt.Errorf("no choices in response")
	}

	usage := TokenUsage{InputTokens: result.Usage.PromptTokens, OutputTokens: result.Usage.CompletionTokens}
	return result.Choices[0].Message.Content, usage, nil
}
