package hasher

import (
	"strings"
	"testing"
)

func TestStripBlock_NoBlock(t *testing.T) {
	content := []byte("package main\n\nfunc main() {}\n")
	result := StripBlock(content)
	if string(result) != string(content) {
		t.Errorf("expected no change, got: %q", result)
	}
}

func TestStripBlock_GoBlockComment(t *testing.T) {
	content := `/*llmdoc:start
summary: Does stuff.
hash: abc123
model: test
generated: 2026-01-01T00:00:00Z
version: 1
llmdoc:end*/
package main

func main() {}
`
	result := StripBlock([]byte(content))
	if strings.Contains(string(result), "llmdoc:start") {
		t.Errorf("block was not stripped: %q", result)
	}
	if !strings.Contains(string(result), "package main") {
		t.Errorf("body content was removed unexpectedly: %q", result)
	}
}

func TestStripBlock_PythonLineComment(t *testing.T) {
	content := `# llmdoc:start
# summary: Does stuff.
# hash: abc123
# model: test
# generated: 2026-01-01T00:00:00Z
# version: 1
# llmdoc:end

def main():
    pass
`
	result := StripBlock([]byte(content))
	if strings.Contains(string(result), "llmdoc:start") {
		t.Errorf("block was not stripped: %q", result)
	}
	if !strings.Contains(string(result), "def main") {
		t.Errorf("body content was removed: %q", result)
	}
}

func TestHashIsStableAfterAnnotation(t *testing.T) {
	// Simulates the core race-condition fix:
	// 1. Compute hash of original file (no block)
	// 2. Add block with that hash
	// 3. Re-compute hash of annotated file — it must equal step 1
	original := []byte("package main\n\nfunc main() {}\n")
	originalHash := ComputeHash(original)

	// Simulate adding an annotation block
	annotated := []byte("/*llmdoc:start\nsummary: Does stuff.\nhash: " + originalHash + "\nmodel: test\ngenerated: 2026-01-01T00:00:00Z\nversion: 1\nllmdoc:end*/\npackage main\n\nfunc main() {}\n")

	rehash := ComputeHash(annotated)
	if rehash != originalHash {
		t.Errorf("hash changed after annotation!\n  original: %s\n  re-hash:  %s\n\nstripped:\n%s", originalHash, rehash, StripBlock(annotated))
	}
}

func TestHashChangesWhenContentChanges(t *testing.T) {
	v1 := []byte("package main\n\nfunc main() {}\n")
	v2 := []byte("package main\n\nfunc main() { println(\"hello\") }\n")

	h1 := ComputeHash(v1)
	h2 := ComputeHash(v2)

	if h1 == h2 {
		t.Error("hash should differ when content changes")
	}
}

func TestHashAnnotatedFileChangesWhenBodyChanges(t *testing.T) {
	block := "/*llmdoc:start\nsummary: Foo.\nhash: oldhash\nmodel: test\ngenerated: 2026-01-01T00:00:00Z\nversion: 1\nllmdoc:end*/\n"
	v1 := []byte(block + "package main\n\nfunc main() {}\n")
	v2 := []byte(block + "package main\n\nfunc main() { println(\"changed\") }\n")

	h1 := ComputeHash(v1)
	h2 := ComputeHash(v2)

	if h1 == h2 {
		t.Error("hash should differ when body changes, even if block is same")
	}
}

// TestStripBlock_IgnoresSentinelInStringLiteral guards against the bug where
// StripBlock falsely matched sentinel strings inside Go const/var declarations.
func TestStripBlock_IgnoresSentinelInStringLiteral(t *testing.T) {
	// This file contains the sentinel as a string literal (like hasher.go itself does).
	content := []byte(`package hasher

const (
	openSentinel  = "llmdoc:start"
	closeSentinel = "llmdoc:end"
)

func foo() {}
`)
	result := StripBlock(content)
	if string(result) != string(content) {
		t.Errorf("StripBlock incorrectly stripped content from a file that has no comment block:\ngot: %q", result)
	}
}

func TestStripBlock_RemovesAllBlocks(t *testing.T) {
	body := "package main\n\nfunc main() {}\n"
	block1 := "/*llmdoc:start\nsummary: Old.\nhash: aaa\nmodel: test\ngenerated: 2026-01-01T00:00:00Z\nversion: 1\nllmdoc:end*/\n"
	block2 := "/*llmdoc:start\nsummary: New.\nhash: bbb\nmodel: test\ngenerated: 2026-06-01T00:00:00Z\nversion: 1\nllmdoc:end*/\n"

	// Two blocks stacked (corruption scenario from switching modes).
	doubleBlock := []byte(block1 + block2 + body)
	result := StripBlock(doubleBlock)

	if strings.Contains(string(result), "llmdoc:start") {
		t.Errorf("double-block file still contains block after stripping: %q", result)
	}
	if !strings.Contains(string(result), "package main") {
		t.Errorf("body content was removed: %q", result)
	}
	// Hash of double-block file must equal hash of plain body.
	if ComputeHash(doubleBlock) != ComputeHash([]byte(body)) {
		t.Error("ComputeHash of double-block file should equal hash of plain body")
	}
}

// TestStripBlock_SentinelInSummary guards against the bug where the summary
// text itself contains the sentinel strings (e.g. for files that process llmdoc
// blocks), causing StripBlock to find a false-positive close sentinel inside
// the summary and incorrectly treat the file as having no block.
func TestStripBlock_SentinelInSummary(t *testing.T) {
	content := []byte("/*llmdoc:start\n" +
		"summary: Strips `llmdoc:start` and `llmdoc:end` sentinels from files.\n" +
		"hash: abc123\n" +
		"model: test\n" +
		"generated: 2026-01-01T00:00:00Z\n" +
		"version: 1\n" +
		"llmdoc:end*/\n" +
		"package hasher\n\nfunc foo() {}\n")
	result := StripBlock(content)
	if strings.Contains(string(result), "llmdoc:start") {
		t.Errorf("block was not stripped: %q", result)
	}
	if !strings.Contains(string(result), "package hasher") {
		t.Errorf("body content was removed: %q", result)
	}
	// Hash must be stable (stripping the block should yield the bare code)
	bare := []byte("package hasher\n\nfunc foo() {}\n")
	if ComputeHash(content) != ComputeHash(bare) {
		t.Errorf("hash of annotated file should equal hash of bare file when summary contains sentinels")
	}
}

func TestHashAnnotatedFileStableWhenOnlyBlockChanges(t *testing.T) {
	body := "package main\n\nfunc main() {}\n"
	block1 := "/*llmdoc:start\nsummary: Old summary.\nhash: aaa\nmodel: test\ngenerated: 2026-01-01T00:00:00Z\nversion: 1\nllmdoc:end*/\n"
	block2 := "/*llmdoc:start\nsummary: New summary.\nhash: bbb\nmodel: test\ngenerated: 2026-06-01T00:00:00Z\nversion: 1\nllmdoc:end*/\n"

	h1 := ComputeHash([]byte(block1 + body))
	h2 := ComputeHash([]byte(block2 + body))

	if h1 != h2 {
		t.Error("hash should be stable when only the block comment changes")
	}
}
