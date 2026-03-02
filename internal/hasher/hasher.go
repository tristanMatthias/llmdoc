package hasher

import (
	"crypto/sha256"
	"fmt"
	"strings"

	"github.com/tristanmatthias/llmdoc/internal/comment"
)

// StripBlock removes all llmdoc comment blocks from content using language-agnostic
// sentinel matching. The returned bytes are suitable for hashing — they represent
// the "real" file content without any annotation headers.
//
// Normally files have at most one block, but if multiple blocks are present (e.g.
// from a corrupted migration) they are all removed. If no block is present,
// content is returned unchanged.
func StripBlock(content []byte) []byte {
	for {
		next := stripOne(content)
		if len(next) == len(content) {
			return content
		}
		content = next
	}
}

// stripOne removes the first llmdoc block found in content, or returns content
// unchanged if none is present.
func stripOne(content []byte) []byte {
	s := string(content)

	startIdx := strings.Index(s, comment.OpenSentinel)
	if startIdx == -1 || !comment.IsValidSentinelLine(s, startIdx, len(comment.OpenSentinel)) {
		return content
	}

	// Find the first CloseSentinel that is on a proper comment-only line, skipping
	// false positives where the sentinel appears inside summary text.
	endIdx := -1
	for search := startIdx + len(comment.OpenSentinel); ; {
		idx := strings.Index(s[search:], comment.CloseSentinel)
		if idx == -1 {
			// No valid close sentinel found — treat as no block.
			return content
		}
		absIdx := search + idx
		if comment.IsValidSentinelLine(s, absIdx, len(comment.CloseSentinel)) {
			endIdx = absIdx
			break
		}
		search = absIdx + len(comment.CloseSentinel)
	}

	// Find the start of the line containing the open sentinel (to remove the
	// block-comment opener like "/*" that may precede the sentinel on the same line)
	lineStart := startIdx
	for lineStart > 0 && s[lineStart-1] != '\n' {
		lineStart--
	}

	// Find the end of the line containing the close sentinel (to remove the
	// block-comment closer like "*/" that follows on the same line)
	lineEnd := endIdx + len(comment.CloseSentinel)
	for lineEnd < len(s) && s[lineEnd] != '\n' {
		lineEnd++
	}
	// Consume the trailing newline too
	if lineEnd < len(s) && s[lineEnd] == '\n' {
		lineEnd++
	}

	return []byte(s[:lineStart] + s[lineEnd:])
}

// ComputeHash returns the lowercase hex-encoded SHA-256 hash of the file content
// with the llmdoc block stripped. This hash is stored in the block and compared
// on subsequent runs to detect file changes.
func ComputeHash(content []byte) string {
	stripped := StripBlock(content)
	h := sha256.Sum256(stripped)
	return fmt.Sprintf("%x", h)
}
