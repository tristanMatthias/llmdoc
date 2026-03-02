package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strconv"
	"time"
)

// SummaryRequest contains everything needed to generate a file summary.
type SummaryRequest struct {
	FilePath        string // relative path from project root
	FileContent     string // raw content with the llmdoc block stripped
	PreviousSummary string // empty on first annotation; prior summary for incremental updates
	Language        string // human-readable language name
}

// TokenUsage reports token consumption for a single LLM call.
type TokenUsage struct {
	InputTokens  int
	OutputTokens int
}

func (t TokenUsage) Total() int { return t.InputTokens + t.OutputTokens }

// Provider is the interface all LLM backends must implement.
type Provider interface {
	Summarize(ctx context.Context, req SummaryRequest) (string, TokenUsage, error)
}

// baseClient holds the fields shared by all HTTP-based LLM providers.
type baseClient struct {
	apiKey string
	model  string
	client *http.Client
}

func newBaseClient(apiKey, model string) baseClient {
	return baseClient{apiKey: apiKey, model: model, client: &http.Client{}}
}

// postJSON marshals payload as JSON, POSTs to url, and unmarshals the response
// body into result. setHeaders is called after Content-Type is set so callers
// can add auth and version headers. Returns the raw body, HTTP status code, and
// response headers (useful for Retry-After on 429/529 responses).
func (b *baseClient) postJSON(ctx context.Context, url string, payload, result any, setHeaders func(*http.Request)) ([]byte, int, http.Header, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, 0, nil, fmt.Errorf("marshaling request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, 0, nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	setHeaders(req)
	resp, err := b.client.Do(req)
	if err != nil {
		return nil, 0, nil, fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()
	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, resp.Header, fmt.Errorf("reading response: %w", err)
	}
	if err := json.Unmarshal(rawBody, result); err != nil {
		return rawBody, resp.StatusCode, resp.Header, fmt.Errorf("parsing response: %w", err)
	}
	return rawBody, resp.StatusCode, resp.Header, nil
}

// parseRetryAfter reads the Retry-After response header and returns the
// suggested wait duration. Returns 0 if the header is absent or non-numeric.
func parseRetryAfter(h http.Header) time.Duration {
	v := h.Get("retry-after")
	if v == "" {
		return 0
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return 0
	}
	return time.Duration(n) * time.Second
}

// retryableError wraps an error to signal that the operation may succeed if retried.
// If wait is nonzero it is used as the delay instead of exponential backoff —
// providers should populate it from the API's Retry-After response header.
type retryableError struct {
	cause error
	wait  time.Duration
}

func (e retryableError) Error() string { return e.cause.Error() }
func (e retryableError) Unwrap() error { return e.cause }

// withRetry calls fn up to 5 times, backing off whenever fn returns a
// retryableError. If the error carries an explicit wait duration (from a
// Retry-After header) that is used; otherwise exponential backoff with jitter
// is applied (1s, 2s, 4s, 8s ± up to 50%).
func withRetry(ctx context.Context, fn func() (string, TokenUsage, error)) (string, TokenUsage, error) {
	const maxAttempts = 5
	var re retryableError
	for attempt := 0; attempt < maxAttempts; attempt++ {
		s, u, err := fn()
		if err == nil {
			return s, u, nil
		}
		if !errors.As(err, &re) || attempt == maxAttempts-1 {
			return "", TokenUsage{}, err
		}
		var delay time.Duration
		if re.wait > 0 {
			delay = re.wait
		} else {
			base := time.Second << attempt // 1s, 2s, 4s, 8s
			delay = base + time.Duration(rand.Int63n(int64(base)/2))
		}
		select {
		case <-ctx.Done():
			return "", TokenUsage{}, ctx.Err()
		case <-time.After(delay):
		}
	}
	panic("unreachable")
}

const systemPrompt = `You are a code documentation assistant for a tool called llmdoc. Your job is to write concise, accurate summaries of source code files.

These summaries are stored as comment headers and are used by other LLMs to quickly understand a codebase without reading every file in full.

Rules:
- Write 2-4 sentences. Be specific about what the file DOES, not just what it IS.
- Name key types, functions, or algorithms if they are the dominant logic.
- Avoid boilerplate phrases like "This file contains..." or "This module provides...".
- Do not repeat the filename or path.
- Write in present tense.
- Output ONLY the summary text. No markdown, no preamble, no explanation.`

func buildPrompt(req SummaryRequest) string {
	header := "File: " + req.FilePath + "\nLanguage: " + req.Language + "\n\n"
	if req.PreviousSummary != "" {
		return header +
			"Previous summary (written when the file was last annotated):\n" + req.PreviousSummary +
			"\n\nThe file has changed since the previous summary. Here is the current content:\n\n---\n" +
			req.FileContent +
			"\n---\n\nUpdate the summary to reflect the current state of the file. Keep the same style and length unless the changes require more or fewer sentences.\nOutput ONLY the updated summary text."
	}
	return header + "---\n" + req.FileContent + "\n---\n\nWrite a concise summary of this file."
}
