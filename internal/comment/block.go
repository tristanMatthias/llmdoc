package comment

import (
	"fmt"
	"strings"
	"time"
)

const (
	OpenSentinel  = "llmdoc:start"
	CloseSentinel = "llmdoc:end"
)

// Block is the structured data stored in an llmdoc comment header.
type Block struct {
	Summary     string
	ContentHash string
	GeneratedAt time.Time
	Model       string
	Version     int
}

// Render serialises a Block to the appropriate comment syntax for the given language.
// The rendered string ends with a newline.
func Render(b Block, syntax CommentSyntax) string {
	// Format the summary: wrap long lines with continuation indent
	summary := formatSummary(b.Summary)

	at := b.GeneratedAt
	if at.IsZero() {
		at = time.Now().UTC()
	}
	generated := at.Format(time.RFC3339)

	version := b.Version
	if version == 0 {
		version = 1
	}

	lines := []string{
		fmt.Sprintf("summary: %s", summary),
		fmt.Sprintf("hash: %s", b.ContentHash),
		fmt.Sprintf("model: %s", b.Model),
		fmt.Sprintf("generated: %s", generated),
		fmt.Sprintf("version: %d", version),
	}

	// Determine open/close markers and per-line prefix for each form.
	var open, close, linePrefix string
	if syntax.BlockOpen != "" {
		// Sanitize: prevent "*/" from prematurely closing a block comment.
		for i, l := range lines {
			lines[i] = strings.ReplaceAll(l, "*/", "* /")
		}
		open, close = syntax.BlockOpen+OpenSentinel+"\n", CloseSentinel+syntax.BlockClose+"\n"
	} else {
		bare := strings.TrimRight(syntax.LinePrefix, " ") // sentinel lines have no trailing space
		open, close = bare+OpenSentinel+"\n", bare+CloseSentinel+"\n"
		linePrefix = syntax.LinePrefix
	}

	var sb strings.Builder
	sb.WriteString(open)
	for _, l := range lines {
		sb.WriteString(linePrefix)
		sb.WriteString(l)
		sb.WriteString("\n")
	}
	sb.WriteString(close)
	return sb.String()
}

// Parse extracts a Block from raw file content.
// Returns (nil, nil) if no llmdoc block is present.
// Returns (nil, err) if a block is found but malformed.
func Parse(content string, syntax CommentSyntax) (*Block, error) {
	startIdx := strings.Index(content, OpenSentinel)
	if startIdx == -1 {
		return nil, nil
	}

	// Find the first CloseSentinel that is on a proper comment-only line.
	// Skip false positives where the sentinel appears inside summary text
	// (e.g. "strips llmdoc:end blocks" — the backtick chars around the
	// sentinel make it an invalid sentinel line).
	endIdx := -1
	for search := startIdx + len(OpenSentinel); ; {
		idx := strings.Index(content[search:], CloseSentinel)
		if idx == -1 {
			return nil, fmt.Errorf("found %q but not %q — malformed block", OpenSentinel, CloseSentinel)
		}
		absIdx := search + idx
		if IsValidSentinelLine(content, absIdx, len(CloseSentinel)) {
			endIdx = absIdx
			break
		}
		search = absIdx + len(CloseSentinel)
	}

	// Extract the body between the sentinels
	bodyStart := strings.Index(content[startIdx:], "\n")
	if bodyStart == -1 {
		return nil, fmt.Errorf("malformed block: no newline after open sentinel")
	}
	bodyStart += startIdx + 1 // skip past the newline

	bodyEnd := endIdx
	body := content[bodyStart:bodyEnd]

	// Strip line prefixes if using line-comment form
	prefix := syntax.LinePrefix
	b := &Block{Version: 1}

	for _, rawLine := range strings.Split(body, "\n") {
		line := strings.TrimSpace(strings.TrimPrefix(rawLine, prefix))
		if line == "" {
			continue
		}
		key, val, ok := strings.Cut(line, ": ")
		if !ok {
			continue
		}
		val = strings.TrimSpace(val)
		switch key {
		case "summary":
			b.Summary = val
		case "hash":
			b.ContentHash = val
		case "model":
			b.Model = val
		case "generated":
			if t, err := time.Parse(time.RFC3339, val); err == nil {
				b.GeneratedAt = t
			}
		case "version":
			fmt.Sscanf(val, "%d", &b.Version)
		}
	}

	if b.ContentHash == "" {
		return nil, fmt.Errorf("llmdoc block missing required 'hash' field")
	}

	return b, nil
}

// IsValidSentinelLine reports whether a sentinel of sentinelLen bytes starting
// at sentinelIdx is on a comment-only line. The characters before the sentinel
// (back to the last newline) must be comment-prefix chars, and the characters
// after it (to the next newline) must be comment-close chars. This prevents
// false-positive matches when sentinel strings appear inside summary text.
func IsValidSentinelLine(s string, sentinelIdx, sentinelLen int) bool {
	for i := sentinelIdx - 1; i >= 0; i-- {
		c := s[i]
		if c == '\n' {
			break
		}
		switch c {
		case '/', '*', '#', '-', '<', '!', ' ', '\t':
		default:
			return false
		}
	}
	for i := sentinelIdx + sentinelLen; i < len(s) && s[i] != '\n'; i++ {
		switch s[i] {
		case '/', '*', '-', '>', '!', '<', ' ', '\t':
		default:
			return false
		}
	}
	return true
}

// formatSummary handles multi-line summaries with continuation indent.
// Single-line summaries are returned as-is.
func formatSummary(s string) string {
	s = strings.TrimSpace(s)
	lines := strings.Split(s, "\n")
	if len(lines) == 1 {
		return lines[0]
	}
	// First line inline, rest with continuation indent
	var sb strings.Builder
	sb.WriteString(lines[0])
	for _, l := range lines[1:] {
		sb.WriteString("\n  ")
		sb.WriteString(strings.TrimSpace(l))
	}
	return sb.String()
}
