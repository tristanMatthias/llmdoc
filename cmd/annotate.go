package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"sync"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/tristanmatthias/llmdoc/internal/annotator"
	"github.com/tristanmatthias/llmdoc/internal/llm"
)

var annotateCmd = &cobra.Command{
	Use:   "annotate [path]",
	Short: "Annotate source files with LLM-generated summaries",
	Long: `annotate recursively scans the given path (default: current directory) and adds
or updates an llmdoc comment block at the top of each matching source file.

Files whose content hash matches the stored hash are skipped. Use --force to
re-annotate all files regardless.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		root := rootArg(args)
		dryRun, _ := cmd.Flags().GetBool("dry-run")
		verbose, _ := cmd.Flags().GetBool("verbose")
		quiet, _ := cmd.Flags().GetBool("quiet")

		var provider llm.Provider
		if !dryRun {
			var err error
			provider, err = llm.NewProvider(cfg)
			if err != nil {
				return err
			}
		}

		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer cancel()

		if !quiet && dryRun {
			fmt.Println("(dry run — no files will be modified)")
		}

		start := time.Now()
		total, resultsCh, err := annotator.Run(ctx, root, cfg, provider, annotator.Options{DryRun: dryRun})
		if err != nil {
			return err
		}

		// Collect results while driving the progress display.
		var results []annotator.Result

		var prog *progress
		if !quiet && isInteractive() {
			prog = newProgress(total)
			prog.start()
		}

		for r := range resultsCh {
			results = append(results, r)
			if prog != nil {
				prog.update(len(results), r.File.RelPath)
			}
		}

		if prog != nil {
			prog.stop()
		}

		// Sort by path for deterministic output.
		sort.Slice(results, func(i, j int) bool {
			return results[i].File.RelPath < results[j].File.RelPath
		})

		counts := map[annotator.Status]int{}
		var totalUsage llm.TokenUsage
		for _, r := range results {
			counts[r.Status]++
			totalUsage.InputTokens += r.TokensUsed.InputTokens
			totalUsage.OutputTokens += r.TokensUsed.OutputTokens

			if !shouldPrint(quiet, verbose, r.Status) {
				continue
			}
			if r.Status == annotator.StatusError {
				fmt.Fprintf(os.Stderr, "  %-10s%s: %v\n", r.Status, r.File.RelPath, r.Err)
			} else {
				fmt.Printf("  %-10s%s\n", r.Status, r.File.RelPath)
			}
		}

		// Inline mode: remove the index file if present (migration from index → inline mode).
		if cfg.Mode != "index" && !dryRun {
			if _, statErr := os.Stat(cfg.IndexFile); statErr == nil {
				if rmErr := os.Remove(cfg.IndexFile); rmErr == nil {
					os.Remove(filepath.Dir(cfg.IndexFile)) // remove dir if now empty
					if !quiet {
						fmt.Printf("  removed   %s\n", cfg.IndexFile)
					}
				}
			}
		}

		if !quiet {
			elapsed := time.Since(start).Round(time.Millisecond)
			fmt.Printf("\nSummary: %d created, %d updated, %d unchanged, %d migrated, %d cleaned, %d errors  (%s)\n",
				counts[annotator.StatusCreated],
				counts[annotator.StatusUpdated],
				counts[annotator.StatusUnchanged],
				counts[annotator.StatusMigrated],
				counts[annotator.StatusCleaned],
				counts[annotator.StatusError],
				elapsed,
			)
			if !dryRun && totalUsage.Total() > 0 {
				fmt.Printf("Tokens:  %s in / %s out  (%s total)\n",
					fmtInt(totalUsage.InputTokens),
					fmtInt(totalUsage.OutputTokens),
					fmtInt(totalUsage.Total()),
				)
			}
		}

		if counts[annotator.StatusError] > 0 {
			os.Exit(1)
		}
		return nil
	},
}

// progress drives a spinner + counter line on the terminal.
type progress struct {
	total   int
	done    int
	current string
	mu      sync.Mutex
	stopCh  chan struct{}
	doneCh  chan struct{}
}

var spinFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

func newProgress(total int) *progress {
	return &progress{
		total:  total,
		stopCh: make(chan struct{}),
		doneCh: make(chan struct{}),
	}
}

func (p *progress) start() {
	go func() {
		defer close(p.doneCh)
		ticker := time.NewTicker(80 * time.Millisecond)
		defer ticker.Stop()
		frame := 0
		for {
			select {
			case <-ticker.C:
				p.mu.Lock()
				done, total, cur := p.done, p.total, p.current
				p.mu.Unlock()

				pct := 0
				if total > 0 {
					pct = done * 100 / total
				}
				// Truncate long paths to keep the line compact.
				if len(cur) > 45 {
					cur = "…" + cur[len(cur)-44:]
				}
				fmt.Printf("\r%s %d/%d (%d%%)  %s\033[K",
					spinFrames[frame%len(spinFrames)], done, total, pct, cur)
				frame++
			case <-p.stopCh:
				return
			}
		}
	}()
}

func (p *progress) update(done int, current string) {
	p.mu.Lock()
	p.done = done
	p.current = current
	p.mu.Unlock()
}

func (p *progress) stop() {
	close(p.stopCh)
	<-p.doneCh
	fmt.Print("\r\033[K") // erase the spinner line
}

// shouldPrint reports whether a result should be printed to the user.
// In quiet mode only errors are shown; unchanged/skipped are suppressed unless verbose.
func shouldPrint(quiet, verbose bool, status annotator.Status) bool {
	if quiet {
		return status == annotator.StatusError
	}
	return verbose || (status != annotator.StatusUnchanged && status != annotator.StatusSkipped)
}

// isInteractive reports whether stdout is connected to a terminal.
func isInteractive() bool {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

// fmtInt formats an integer with comma separators (e.g. 1234567 → "1,234,567").
func fmtInt(n int) string {
	s := fmt.Sprintf("%d", n)
	out := make([]byte, 0, len(s)+len(s)/3)
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, byte(c))
	}
	return string(out)
}

func init() {
	annotateCmd.Flags().Bool("dry-run", false, "print which files would be annotated without making changes")
	annotateCmd.Flags().BoolP("verbose", "v", false, "print status for unchanged files too")
	annotateCmd.Flags().BoolP("quiet", "q", false, "suppress all output except errors")
	rootCmd.AddCommand(annotateCmd)
}
