package comment

import (
	"strings"
	"testing"
	"time"
)

var goSyntax = CommentSyntax{BlockOpen: "/*", BlockClose: "*/", Language: "Go"}
var pySyntax = CommentSyntax{LinePrefix: "# ", Language: "Python"}
var sqlSyntax = CommentSyntax{LinePrefix: "-- ", Language: "SQL"}

func TestRenderParseRoundTrip_Go(t *testing.T) {
	b := Block{
		Summary:     "Implements the OAuth2 token refresh flow.",
		ContentHash: "abc123def456",
		Model:       "claude-opus-4-6",
		GeneratedAt: time.Date(2026, 3, 1, 14, 0, 0, 0, time.UTC),
		Version:     1,
	}

	rendered := Render(b, goSyntax)

	if !strings.HasPrefix(rendered, "/*llmdoc:start") {
		t.Errorf("expected block comment start, got: %q", rendered[:min(50, len(rendered))])
	}
	if !strings.HasSuffix(strings.TrimSpace(rendered), "llmdoc:end*/") {
		t.Errorf("expected block comment end, got: %q", rendered[max(0, len(rendered)-30):])
	}

	parsed, err := Parse(rendered, goSyntax)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if parsed == nil {
		t.Fatal("Parse returned nil for valid block")
	}
	if parsed.Summary != b.Summary {
		t.Errorf("summary mismatch: got %q, want %q", parsed.Summary, b.Summary)
	}
	if parsed.ContentHash != b.ContentHash {
		t.Errorf("hash mismatch: got %q, want %q", parsed.ContentHash, b.ContentHash)
	}
	if parsed.Model != b.Model {
		t.Errorf("model mismatch: got %q, want %q", parsed.Model, b.Model)
	}
}

func TestRenderParseRoundTrip_Python(t *testing.T) {
	b := Block{
		Summary:     "Provides a thin async wrapper around the Stripe billing API.",
		ContentHash: "deadbeef1234",
		Model:       "gpt-4o",
		GeneratedAt: time.Date(2026, 3, 1, 14, 0, 0, 0, time.UTC),
		Version:     1,
	}

	rendered := Render(b, pySyntax)

	if !strings.HasPrefix(rendered, "#llmdoc:start") {
		t.Errorf("expected hash line comment start, got: %q", rendered[:min(50, len(rendered))])
	}

	parsed, err := Parse(rendered, pySyntax)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if parsed.Summary != b.Summary {
		t.Errorf("summary mismatch: got %q, want %q", parsed.Summary, b.Summary)
	}
	if parsed.ContentHash != b.ContentHash {
		t.Errorf("hash mismatch: got %q, want %q", parsed.ContentHash, b.ContentHash)
	}
}

func TestParseNoBlock(t *testing.T) {
	content := "package main\n\nfunc main() {}\n"
	parsed, err := Parse(content, goSyntax)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed != nil {
		t.Error("expected nil for file with no block")
	}
}

func TestParseMalformedBlock(t *testing.T) {
	content := "/*llmdoc:start\nsummary: Foo\n// missing end\n"
	_, err := Parse(content, goSyntax)
	if err == nil {
		t.Error("expected error for malformed block (missing end sentinel)")
	}
}

func TestRenderMultiLineSummary(t *testing.T) {
	b := Block{
		Summary:     "Line one of summary.\nLine two continues here.\nLine three ends.",
		ContentHash: "abc",
		Model:       "test",
		Version:     1,
	}
	rendered := Render(b, goSyntax)
	if !strings.Contains(rendered, "summary: Line one") {
		t.Errorf("expected summary prefix, got: %q", rendered)
	}
	if !strings.Contains(rendered, "  Line two") {
		t.Errorf("expected continuation indent, got: %q", rendered)
	}
}

// TestRenderSanitizesBlockCommentTerminator guards against summaries that contain
// "*/" which would prematurely close a /* */ block comment and corrupt the file.
func TestRenderSanitizesBlockCommentTerminator(t *testing.T) {
	b := Block{
		Summary:     "Wraps the foo() function which returns */ and baz.",
		ContentHash: "abc",
		Model:       "test",
		Version:     1,
	}
	rendered := Render(b, goSyntax)
	if strings.Contains(rendered, "*/") && !strings.HasSuffix(strings.TrimSpace(rendered), "llmdoc:end*/") {
		// The only "*/" allowed is the final block comment closer.
		t.Errorf("rendered block comment contains bare */ that would terminate the block comment:\n%s", rendered)
	}
	// Verify the sanitized form is present
	if !strings.Contains(rendered, "* /") {
		t.Errorf("expected '* /' (sanitized) in rendered output, got:\n%s", rendered)
	}
}

func TestRenderSQL(t *testing.T) {
	b := Block{
		Summary:     "Migration adding the billing_events table.",
		ContentHash: "abc",
		Model:       "test",
		Version:     1,
	}
	rendered := Render(b, sqlSyntax)
	if !strings.HasPrefix(rendered, "--llmdoc:start") {
		t.Errorf("expected SQL line comment, got: %q", rendered[:min(30, len(rendered))])
	}
}

// TestParse_SentinelInSummary guards against the bug where the close sentinel
// appears inside the summary text (e.g. for files that process llmdoc blocks),
// causing Parse to extract a truncated body and miss the hash field.
func TestParse_SentinelInSummary(t *testing.T) {
	content := "/*llmdoc:start\n" +
		"summary: Strips `llmdoc:start` and `llmdoc:end` sentinels from files.\n" +
		"hash: abc123\n" +
		"model: test\n" +
		"generated: 2026-01-01T00:00:00Z\n" +
		"version: 1\n" +
		"llmdoc:end*/\n" +
		"package main\n"
	block, err := Parse(content, goSyntax)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if block == nil {
		t.Fatal("expected block, got nil")
	}
	if block.ContentHash != "abc123" {
		t.Errorf("wrong hash: got %q, want %q", block.ContentHash, "abc123")
	}
}

// TestParse_SentinelInSummary_Python tests the same scenario for line-comment syntax.
func TestParse_SentinelInSummary_Python(t *testing.T) {
	content := "#llmdoc:start\n" +
		"# summary: Strips `llmdoc:start` and `llmdoc:end` sentinels.\n" +
		"# hash: deadbeef\n" +
		"# model: test\n" +
		"# generated: 2026-01-01T00:00:00Z\n" +
		"# version: 1\n" +
		"#llmdoc:end\n" +
		"def main(): pass\n"
	block, err := Parse(content, pySyntax)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if block == nil {
		t.Fatal("expected block, got nil")
	}
	if block.ContentHash != "deadbeef" {
		t.Errorf("wrong hash: got %q, want %q", block.ContentHash, "deadbeef")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
