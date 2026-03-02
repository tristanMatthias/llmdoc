package annotator

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tristanmatthias/llmdoc/internal/comment"
	"github.com/tristanmatthias/llmdoc/internal/config"
	"github.com/tristanmatthias/llmdoc/internal/hasher"
	"github.com/tristanmatthias/llmdoc/internal/llm"
)

// mockProvider returns a fixed summary for any request.
type mockProvider struct {
	summary string
}

func (m *mockProvider) Summarize(_ context.Context, req llm.SummaryRequest) (string, llm.TokenUsage, error) {
	if m.summary != "" {
		return m.summary, llm.TokenUsage{InputTokens: 100, OutputTokens: 20}, nil
	}
	return "Mock summary for " + req.FilePath + ".", llm.TokenUsage{InputTokens: 100, OutputTokens: 20}, nil
}

// collect drains a result channel into a slice.
func collect(ch <-chan Result) []Result {
	var out []Result
	for r := range ch {
		out = append(out, r)
	}
	return out
}

func TestAnnotate_CreatesBlockOnNewFile(t *testing.T) {
	dir := t.TempDir()

	goFile := filepath.Join(dir, "main.go")
	original := "package main\n\nfunc main() {}\n"
	if err := os.WriteFile(goFile, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		Extensions:  []string{".go"},
		Ignore:      []string{},
		Concurrency: 1,
		Model:       "test-model",
	}

	provider := &mockProvider{summary: "Entry point of the test binary."}
	_, ch, err := Run(context.Background(), dir, cfg, provider, Options{})
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	results := collect(ch)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Status != StatusCreated {
		t.Errorf("expected StatusCreated, got %v", results[0].Status)
	}

	content, err := os.ReadFile(goFile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "llmdoc:start") {
		t.Error("block not present in annotated file")
	}
	if !strings.Contains(string(content), "Entry point of the test binary.") {
		t.Error("summary not in annotated file")
	}
	if !strings.Contains(string(content), "package main") {
		t.Error("original content was lost")
	}
}

func TestAnnotate_UnchangedSkipsLLM(t *testing.T) {
	dir := t.TempDir()

	pyFile := filepath.Join(dir, "app.py")
	original := "def main():\n    pass\n"
	if err := os.WriteFile(pyFile, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		Extensions:  []string{".py"},
		Ignore:      []string{},
		Concurrency: 1,
		Model:       "test-model",
	}

	callCount := 0
	provider := &countingProvider{t: t, callCount: &callCount}

	// First run — should annotate
	_, ch, err := Run(context.Background(), dir, cfg, provider, Options{})
	if err != nil {
		t.Fatalf("first Run error: %v", err)
	}
	results := collect(ch)
	if results[0].Status != StatusCreated {
		t.Errorf("expected StatusCreated, got %v", results[0].Status)
	}
	if callCount != 1 {
		t.Errorf("expected 1 LLM call, got %d", callCount)
	}

	// Second run on unchanged file — should skip
	_, ch, err = Run(context.Background(), dir, cfg, provider, Options{})
	if err != nil {
		t.Fatalf("second Run error: %v", err)
	}
	results = collect(ch)
	if results[0].Status != StatusUnchanged {
		t.Errorf("expected StatusUnchanged on second run, got %v", results[0].Status)
	}
	if callCount != 1 {
		t.Errorf("expected still 1 LLM call total, got %d", callCount)
	}
}

func TestAnnotate_UpdatesWhenFileChanges(t *testing.T) {
	dir := t.TempDir()

	tsFile := filepath.Join(dir, "utils.ts")
	original := "export function add(a: number, b: number): number { return a + b; }\n"
	if err := os.WriteFile(tsFile, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		Extensions:  []string{".ts"},
		Ignore:      []string{},
		Concurrency: 1,
		Model:       "test-model",
	}

	provider := &mockProvider{summary: "Utility functions."}

	// First run — creates the block
	_, ch, _ := Run(context.Background(), dir, cfg, provider, Options{})
	collect(ch)

	// Simulate editing the file body: append to the annotated file (block is preserved)
	f, err := os.OpenFile(tsFile, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.WriteString("\nexport function sub(a: number, b: number): number { return a - b; }\n")
	f.Close()
	if err != nil {
		t.Fatal(err)
	}

	// Second run — should detect change
	_, ch, err = Run(context.Background(), dir, cfg, provider, Options{})
	if err != nil {
		t.Fatal(err)
	}
	results := collect(ch)
	if results[0].Status != StatusUpdated {
		t.Errorf("expected StatusUpdated after file change, got %v", results[0].Status)
	}
}

func TestAnnotate_HashStable(t *testing.T) {
	dir := t.TempDir()

	goFile := filepath.Join(dir, "svc.go")
	original := "package svc\n\ntype Service struct{}\n"
	if err := os.WriteFile(goFile, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		Extensions:  []string{".go"},
		Ignore:      []string{},
		Concurrency: 1,
		Model:       "test-model",
	}

	provider := &mockProvider{}
	_, ch, _ := Run(context.Background(), dir, cfg, provider, Options{})
	collect(ch)

	content, _ := os.ReadFile(goFile)
	syntax, _ := comment.ForExtension(".go")
	block, err := comment.Parse(string(content), syntax)
	if err != nil || block == nil {
		t.Fatalf("expected block in annotated file, got block=%v err=%v", block, err)
	}

	currentHash := hasher.ComputeHash(content)
	if currentHash != block.ContentHash {
		t.Errorf("hash mismatch after annotation!\n  stored:  %s\n  current: %s", block.ContentHash, currentHash)
	}
}

func TestAnnotate_DryRun(t *testing.T) {
	dir := t.TempDir()
	goFile := filepath.Join(dir, "main.go")
	original := "package main\nfunc main() {}\n"
	os.WriteFile(goFile, []byte(original), 0644)

	cfg := &config.Config{
		Extensions:  []string{".go"},
		Ignore:      []string{},
		Concurrency: 1,
	}
	provider := &mockProvider{}

	_, ch, _ := Run(context.Background(), dir, cfg, provider, Options{DryRun: true})
	results := collect(ch)
	if results[0].Status != StatusCreated {
		t.Errorf("dry run should still report would-create, got %v", results[0].Status)
	}

	content, _ := os.ReadFile(goFile)
	if strings.Contains(string(content), "llmdoc:start") {
		t.Error("dry run should not modify files")
	}
}

func TestAnnotate_TokensReported(t *testing.T) {
	dir := t.TempDir()
	goFile := filepath.Join(dir, "svc.go")
	os.WriteFile(goFile, []byte("package svc\n"), 0644)

	cfg := &config.Config{
		Extensions:  []string{".go"},
		Ignore:      []string{},
		Concurrency: 1,
		Model:       "test-model",
	}
	provider := &mockProvider{}

	_, ch, _ := Run(context.Background(), dir, cfg, provider, Options{})
	results := collect(ch)

	if results[0].TokensUsed.InputTokens != 100 {
		t.Errorf("expected 100 input tokens, got %d", results[0].TokensUsed.InputTokens)
	}
	if results[0].TokensUsed.OutputTokens != 20 {
		t.Errorf("expected 20 output tokens, got %d", results[0].TokensUsed.OutputTokens)
	}
}

func TestAnnotate_IndexMode(t *testing.T) {
	dir := t.TempDir()

	goFile := filepath.Join(dir, "svc.go")
	original := "package svc\n\ntype Service struct{}\n"
	if err := os.WriteFile(goFile, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		Extensions:  []string{".go"},
		Ignore:      []string{},
		Concurrency: 1,
		Model:       "test-model",
		Mode:        "index",
		IndexFile:   filepath.Join(dir, ".llmdoc", "index.yaml"),
	}

	provider := &mockProvider{summary: "Service layer."}
	_, ch, err := Run(context.Background(), dir, cfg, provider, Options{})
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	results := collect(ch)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Status != StatusCreated {
		t.Errorf("expected StatusCreated, got %v", results[0].Status)
	}

	// Source file must NOT be modified
	content, _ := os.ReadFile(goFile)
	if strings.Contains(string(content), "llmdoc:start") {
		t.Error("index mode must not modify source files")
	}
	if string(content) != original {
		t.Errorf("source file was modified: %q", content)
	}

	// Index file must exist and contain the entry
	idxPath := filepath.Join(dir, ".llmdoc", "index.yaml")
	if _, err := os.Stat(idxPath); err != nil {
		t.Fatalf("index file not created: %v", err)
	}
}

func TestAnnotate_IndexMode_UnchangedSkipsLLM(t *testing.T) {
	dir := t.TempDir()
	goFile := filepath.Join(dir, "main.go")
	os.WriteFile(goFile, []byte("package main\nfunc main() {}\n"), 0644)

	cfg := &config.Config{
		Extensions:  []string{".go"},
		Ignore:      []string{},
		Concurrency: 1,
		Model:       "test-model",
		Mode:        "index",
		IndexFile:   filepath.Join(dir, ".llmdoc", "index.yaml"),
	}

	calls := 0
	provider := &countingProvider{t: t, callCount: &calls}

	_, ch, _ := Run(context.Background(), dir, cfg, provider, Options{})
	results := collect(ch)
	if results[0].Status != StatusCreated {
		t.Errorf("first run: expected StatusCreated, got %v", results[0].Status)
	}
	if calls != 1 {
		t.Errorf("expected 1 LLM call, got %d", calls)
	}

	// Second run — hash unchanged, should skip
	_, ch, _ = Run(context.Background(), dir, cfg, provider, Options{})
	results = collect(ch)
	if results[0].Status != StatusUnchanged {
		t.Errorf("second run: expected StatusUnchanged, got %v", results[0].Status)
	}
	if calls != 1 {
		t.Errorf("expected no additional LLM calls, got %d total", calls)
	}
}

// TestAnnotate_MigrateInlineToIndex verifies that switching from inline to index mode
// reuses the inline block's summary — no LLM call is made.
func TestAnnotate_MigrateInlineToIndex(t *testing.T) {
	dir := t.TempDir()

	goFile := filepath.Join(dir, "svc.go")
	original := "package svc\n\ntype Service struct{}\n"
	if err := os.WriteFile(goFile, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}

	indexFile := filepath.Join(dir, ".llmdoc", "index.yaml")
	calls := 0
	provider := &countingProvider{t: t, callCount: &calls}

	// First run in inline mode — annotates the source file with a block comment.
	inlineCfg := &config.Config{
		Extensions:  []string{".go"},
		Ignore:      []string{},
		Concurrency: 1,
		Model:       "test-model",
		Mode:        "inline",
	}
	_, ch, err := Run(context.Background(), dir, inlineCfg, provider, Options{})
	if err != nil {
		t.Fatalf("inline Run error: %v", err)
	}
	collect(ch)
	if calls != 1 {
		t.Fatalf("expected 1 LLM call after inline run, got %d", calls)
	}

	contentAfterInline, _ := os.ReadFile(goFile)
	if !strings.Contains(string(contentAfterInline), "llmdoc:start") {
		t.Fatal("expected inline block in file after inline run")
	}

	// Switch to index mode — should reuse the inline block summary (StatusCleaned, zero LLM calls).
	indexCfg := &config.Config{
		Extensions:  []string{".go"},
		Ignore:      []string{},
		Concurrency: 1,
		Model:       "test-model",
		Mode:        "index",
		IndexFile:   indexFile,
	}
	_, ch, err = Run(context.Background(), dir, indexCfg, provider, Options{})
	if err != nil {
		t.Fatalf("index Run error: %v", err)
	}
	results := collect(ch)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Status != StatusCleaned {
		t.Errorf("expected StatusCleaned (zero-cost migration), got %v", results[0].Status)
	}
	if calls != 1 {
		t.Errorf("expected still 1 LLM call total (zero cost), got %d", calls)
	}

	// Source file must have the inline block stripped.
	contentAfterIndex, _ := os.ReadFile(goFile)
	if strings.Contains(string(contentAfterIndex), "llmdoc:start") {
		t.Error("inline block should have been stripped from source file")
	}
	if !strings.Contains(string(contentAfterIndex), "package svc") {
		t.Error("source content was lost after stripping")
	}

	// Subsequent index-mode run must be fully unchanged.
	_, ch, _ = Run(context.Background(), dir, indexCfg, provider, Options{})
	results = collect(ch)
	if results[0].Status != StatusUnchanged {
		t.Errorf("subsequent index run: expected StatusUnchanged, got %v", results[0].Status)
	}
	if calls != 1 {
		t.Errorf("expected still 1 LLM call total, got %d", calls)
	}
}

// TestAnnotate_MigrateIndexToInline verifies that switching from index to inline mode
// reuses the index summary — no LLM call is made.
func TestAnnotate_MigrateIndexToInline(t *testing.T) {
	dir := t.TempDir()

	goFile := filepath.Join(dir, "svc.go")
	original := "package svc\n\ntype Service struct{}\n"
	if err := os.WriteFile(goFile, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}

	indexFile := filepath.Join(dir, ".llmdoc", "index.yaml")
	calls := 0
	provider := &countingProvider{t: t, callCount: &calls}

	// First run in index mode.
	indexCfg := &config.Config{
		Extensions:  []string{".go"},
		Ignore:      []string{},
		Concurrency: 1,
		Model:       "test-model",
		Mode:        "index",
		IndexFile:   indexFile,
	}
	_, ch, err := Run(context.Background(), dir, indexCfg, provider, Options{})
	if err != nil {
		t.Fatalf("index Run error: %v", err)
	}
	collect(ch)
	if calls != 1 {
		t.Fatalf("expected 1 LLM call after index run, got %d", calls)
	}

	// Switch to inline mode — should reuse the index summary (StatusMigrated, zero LLM calls).
	inlineCfg := &config.Config{
		Extensions:  []string{".go"},
		Ignore:      []string{},
		Concurrency: 1,
		Model:       "test-model",
		Mode:        "inline",
		IndexFile:   indexFile, // needed so migrationIdx can load it
	}
	_, ch, err = Run(context.Background(), dir, inlineCfg, provider, Options{})
	if err != nil {
		t.Fatalf("inline Run error: %v", err)
	}
	results := collect(ch)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Status != StatusMigrated {
		t.Errorf("expected StatusMigrated (zero-cost migration), got %v", results[0].Status)
	}
	if calls != 1 {
		t.Errorf("expected still 1 LLM call total (zero cost), got %d", calls)
	}

	// Source file must now have the inline block.
	contentAfterInline, _ := os.ReadFile(goFile)
	if !strings.Contains(string(contentAfterInline), "llmdoc:start") {
		t.Error("inline block should have been written to source file")
	}
	if !strings.Contains(string(contentAfterInline), "package svc") {
		t.Error("source content was lost after migration")
	}

	// Subsequent inline-mode run must be fully unchanged.
	_, ch, _ = Run(context.Background(), dir, inlineCfg, provider, Options{})
	results = collect(ch)
	if results[0].Status != StatusUnchanged {
		t.Errorf("subsequent inline run: expected StatusUnchanged, got %v", results[0].Status)
	}
	if calls != 1 {
		t.Errorf("expected still 1 LLM call total, got %d", calls)
	}
}

// TestAnnotate_StatusCleaned verifies that a file with an up-to-date index entry
// but a leftover inline block gets StatusCleaned (block stripped, no LLM call).
func TestAnnotate_StatusCleaned(t *testing.T) {
	dir := t.TempDir()

	goFile := filepath.Join(dir, "svc.go")
	original := "package svc\n\ntype Service struct{}\n"
	if err := os.WriteFile(goFile, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}

	indexFile := filepath.Join(dir, ".llmdoc", "index.yaml")
	indexCfg := &config.Config{
		Extensions:  []string{".go"},
		Ignore:      []string{},
		Concurrency: 1,
		Model:       "test-model",
		Mode:        "index",
		IndexFile:   indexFile,
	}

	calls := 0
	provider := &countingProvider{t: t, callCount: &calls}

	// First index run — creates the index entry.
	_, ch, _ := Run(context.Background(), dir, indexCfg, provider, Options{})
	collect(ch)
	if calls != 1 {
		t.Fatalf("expected 1 LLM call, got %d", calls)
	}

	// Manually graft an inline block onto the source file to simulate a stale artifact.
	fileContent, _ := os.ReadFile(goFile)
	fakeBlock := "/*llmdoc:start\nsummary: Old inline summary.\nhash: deadbeef\nmodel: test\ngenerated: 2026-01-01T00:00:00Z\nversion: 1\nllmdoc:end*/\n"
	os.WriteFile(goFile, append([]byte(fakeBlock), fileContent...), 0644)

	// Second index run — index hash matches stripped content; inline block → StatusCleaned.
	_, ch, _ = Run(context.Background(), dir, indexCfg, provider, Options{})
	results := collect(ch)

	if calls != 1 {
		t.Errorf("expected no additional LLM calls, got %d total", calls)
	}
	if len(results) != 1 || results[0].Status != StatusCleaned {
		t.Errorf("expected StatusCleaned, got %v", results[0].Status)
	}

	finalContent, _ := os.ReadFile(goFile)
	if strings.Contains(string(finalContent), "llmdoc:start") {
		t.Error("inline block should have been stripped")
	}
}

// countingProvider counts Summarize calls.
type countingProvider struct {
	t         *testing.T
	callCount *int
}

func (c *countingProvider) Summarize(_ context.Context, req llm.SummaryRequest) (string, llm.TokenUsage, error) {
	*c.callCount++
	return "Summary for " + req.FilePath, llm.TokenUsage{}, nil
}
