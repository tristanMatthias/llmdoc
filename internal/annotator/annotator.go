package annotator

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/tristanmatthias/llmdoc/internal/comment"
	"github.com/tristanmatthias/llmdoc/internal/config"
	"github.com/tristanmatthias/llmdoc/internal/hasher"
	"github.com/tristanmatthias/llmdoc/internal/index"
	"github.com/tristanmatthias/llmdoc/internal/llm"
	"github.com/tristanmatthias/llmdoc/internal/scanner"
)

// Status describes the outcome of annotating a single file.
type Status int

const (
	StatusUnchanged Status = iota
	StatusCreated
	StatusUpdated
	StatusSkipped
	StatusError
	StatusCleaned  // inline block stripped and data moved to index (inline → index migration)
	StatusMigrated // inline block written from index data without LLM (index → inline migration)
)

func (s Status) String() string {
	switch s {
	case StatusUnchanged:
		return "unchanged"
	case StatusCreated:
		return "created"
	case StatusUpdated:
		return "updated"
	case StatusSkipped:
		return "skipped"
	case StatusError:
		return "error"
	case StatusCleaned:
		return "cleaned"
	case StatusMigrated:
		return "migrated"
	default:
		return "unknown"
	}
}

// Result is the outcome of attempting to annotate a single file.
type Result struct {
	File       scanner.FileInfo
	Status     Status
	TokensUsed llm.TokenUsage
	Err        error
}

// Options controls annotate behavior.
type Options struct {
	DryRun bool
}

// Run annotates all files found by the scanner, using the provided LLM provider.
// It returns the total file count and a channel that streams Results as they complete.
// The channel is closed once all files have been processed.
func Run(ctx context.Context, root string, cfg *config.Config, provider llm.Provider, opts Options) (total int, results <-chan Result, err error) {
	files, scanErr := scanner.Walk(root, cfg)
	if scanErr != nil {
		return 0, nil, fmt.Errorf("scanning %s: %w", root, scanErr)
	}

	ch := make(chan Result, len(files))

	// Index mode: load the shared index once; goroutines update it; save after all finish.
	var (
		idx   *index.Index
		idxMu sync.Mutex
	)
	if cfg.Mode == "index" {
		idx, err = index.Load(cfg.IndexFile)
		if err != nil {
			return 0, nil, fmt.Errorf("loading index: %w", err)
		}
	}

	// Inline mode: load the index as a read-only migration source so we can reuse
	// existing summaries when switching from index → inline without calling the LLM.
	var migrationIdx *index.Index
	if cfg.Mode != "index" {
		if mi, loadErr := index.Load(cfg.IndexFile); loadErr == nil && len(mi.Files) > 0 {
			migrationIdx = mi
		}
	}

	go func() {
		sem := make(chan struct{}, cfg.Concurrency)
		var wg sync.WaitGroup
		for _, f := range files {
			wg.Add(1)
			sem <- struct{}{}
			go func(file scanner.FileInfo) {
				defer wg.Done()
				defer func() { <-sem }()

				// Look up any pre-existing annotation for this file.
				var existing *existingAnnotation
				if idx != nil {
					// Index mode: from the mutable shared index.
					idxMu.Lock()
					existing = fromIndexEntry(idx.Files[file.RelPath])
					idxMu.Unlock()
				} else if migrationIdx != nil {
					// Inline mode: from the previous index (read-only migration source).
					existing = fromIndexEntry(migrationIdx.Files[file.RelPath])
				}

				result, entry := processFile(ctx, file, cfg, provider, opts, existing)

				if idx != nil && entry != nil {
					idxMu.Lock()
					idx.Files[file.RelPath] = entry
					idxMu.Unlock()
				}

				ch <- result
			}(f)
		}
		wg.Wait()

		// Save the index after all files are processed.
		if idx != nil && !opts.DryRun {
			if saveErr := index.Save(cfg.IndexFile, idx); saveErr != nil {
				ch <- Result{Status: StatusError, Err: fmt.Errorf("saving index: %w", saveErr)}
			}
		}

		close(ch)
	}()

	return len(files), ch, nil
}

// existingAnnotation carries the prior summary and hash, regardless of storage mode.
type existingAnnotation struct {
	summary string
	hash    string
}

// annotationMeta holds the fields shared by both index.Entry and comment.Block.
// Use newMeta to construct one, then call toEntry or toBlock to convert.
type annotationMeta struct {
	summary     string
	hash        string
	model       string
	generatedAt time.Time
}

func newMeta(summary, hash, model string) annotationMeta {
	return annotationMeta{summary: summary, hash: hash, model: model, generatedAt: time.Now().UTC()}
}

func (m annotationMeta) toEntry() *index.Entry {
	return &index.Entry{Summary: m.summary, Hash: m.hash, Model: m.model, GeneratedAt: m.generatedAt, Version: 1}
}

func (m annotationMeta) toBlock() comment.Block {
	return comment.Block{Summary: m.summary, ContentHash: m.hash, Model: m.model, GeneratedAt: m.generatedAt, Version: 1}
}

func writeInlineBlock(path string, block comment.Block, syntax comment.CommentSyntax, stripped []byte) error {
	return os.WriteFile(path, []byte(comment.Render(block, syntax)+string(stripped)), 0644)
}

// fromIndexEntry converts an index entry to the existingAnnotation type used
// by processFile. Returns nil when e is nil (missing entry).
func fromIndexEntry(e *index.Entry) *existingAnnotation {
	if e == nil {
		return nil
	}
	return &existingAnnotation{summary: e.Summary, hash: e.Hash}
}

// errResult is a convenience constructor for error Results, matching the
// (Result, *index.Entry) signature returned by processFile.
func errResult(file scanner.FileInfo, msg string, err error) (Result, *index.Entry) {
	return Result{File: file, Status: StatusError, Err: fmt.Errorf("%s: %w", msg, err)}, nil
}

func processFile(ctx context.Context, file scanner.FileInfo, cfg *config.Config, provider llm.Provider, opts Options, existing *existingAnnotation) (Result, *index.Entry) {
	content, err := os.ReadFile(file.AbsPath)
	if err != nil {
		return errResult(file, "reading file", err)
	}

	// Strip any inline block (needed for hash computation and LLM input).
	stripped := hasher.StripBlock(content)
	hasInlineBlock := len(stripped) != len(content)

	// If the file has an inline block, read it as the source of truth:
	//   index mode (no entry yet): promotes the block to the index — LLM-free migration.
	//   inline mode (always): overrides any migration-source entry to prevent stale hashes.
	if hasInlineBlock && (cfg.Mode != "index" || existing == nil) {
		if block, parseErr := comment.Parse(string(content), file.Syntax); parseErr == nil && block != nil {
			existing = &existingAnnotation{summary: block.Summary, hash: block.ContentHash}
		}
	}

	currentHash := hasher.ComputeHash(content)

	// Decide whether to annotate.
	if !cfg.Force && existing != nil && existing.hash == currentHash {
		// Index mode: strip any stale inline block and write the index entry (zero LLM calls).
		if cfg.Mode == "index" && hasInlineBlock {
			if !opts.DryRun {
				if writeErr := os.WriteFile(file.AbsPath, stripped, 0644); writeErr != nil {
					return errResult(file, "removing inline block", writeErr)
				}
			}
			return Result{File: file, Status: StatusCleaned}, newMeta(existing.summary, currentHash, cfg.Model).toEntry()
		}

		// Inline mode: write the inline block from the index migration source (zero LLM calls).
		if cfg.Mode != "index" && !hasInlineBlock {
			if !opts.DryRun {
				block := newMeta(existing.summary, currentHash, cfg.Model).toBlock()
				if writeErr := writeInlineBlock(file.AbsPath, block, file.Syntax, stripped); writeErr != nil {
					return errResult(file, "writing inline block", writeErr)
				}
			}
			return Result{File: file, Status: StatusMigrated}, nil
		}

		return Result{File: file, Status: StatusUnchanged}, nil
	}

	isUpdate := existing != nil
	status := StatusCreated
	if isUpdate {
		status = StatusUpdated
	}

	if opts.DryRun {
		return Result{File: file, Status: status}, nil
	}

	req := llm.SummaryRequest{
		FilePath:    file.RelPath,
		FileContent: string(stripped),
		Language:    file.Language,
	}
	if isUpdate {
		req.PreviousSummary = existing.summary
	}

	summary, usage, err := provider.Summarize(ctx, req)
	if err != nil {
		return errResult(file, "LLM error", err)
	}

	if cfg.Mode == "index" {
		// Strip any leftover inline block from the source file.
		if hasInlineBlock {
			if writeErr := os.WriteFile(file.AbsPath, stripped, 0644); writeErr != nil {
				return errResult(file, "removing inline block", writeErr)
			}
		}
		return Result{File: file, Status: status, TokensUsed: usage}, newMeta(summary, currentHash, cfg.Model).toEntry()
	}

	// Inline mode: write the block comment back into the source file.
	block := newMeta(summary, currentHash, cfg.Model).toBlock()
	if err := writeInlineBlock(file.AbsPath, block, file.Syntax, stripped); err != nil {
		return errResult(file, "writing file", err)
	}

	return Result{File: file, Status: status, TokensUsed: usage}, nil
}
