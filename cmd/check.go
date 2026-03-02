package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/tristanmatthias/llmdoc/internal/comment"
	"github.com/tristanmatthias/llmdoc/internal/hasher"
	"github.com/tristanmatthias/llmdoc/internal/index"
	"github.com/tristanmatthias/llmdoc/internal/scanner"
)

var checkCmd = &cobra.Command{
	Use:   "check [path]",
	Short: "Validate annotation hashes without calling the LLM",
	Long: `check scans the codebase and verifies that each annotated file's stored hash
still matches the current file content. Files with stale or missing annotations
are reported. Exits with code 1 if any stale files are found — useful in CI.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		root := rootArg(args)
		quiet, _ := cmd.Flags().GetBool("quiet")

		files, err := scanner.Walk(root, cfg)
		if err != nil {
			return fmt.Errorf("scanning: %w", err)
		}

		// In index mode, load stored hashes from the index file (no per-file reads).
		// In inline mode, resolve them from each file's header in the main loop.
		var idx *index.Index
		if cfg.Mode == "index" {
			idx, err = index.Load(cfg.IndexFile)
			if err != nil {
				return fmt.Errorf("loading index: %w", err)
			}
		}

		stale, missing, current := 0, 0, 0
		for _, f := range files {
			content, err := os.ReadFile(f.AbsPath)
			if err != nil {
				if !quiet {
					fmt.Fprintf(os.Stderr, "  error     %s: %v\n", f.RelPath, err)
				}
				continue
			}

			// Resolve the stored hash for this file.
			var storedHash string
			if idx != nil {
				if e := idx.Files[f.RelPath]; e != nil {
					storedHash = e.Hash
				}
			} else {
				if block, _ := comment.Parse(string(content), f.Syntax); block != nil {
					storedHash = block.ContentHash
				}
			}

			if storedHash == "" {
				missing++
				if !quiet {
					fmt.Printf("  missing   %s\n", f.RelPath)
				}
				continue
			}

			if hasher.ComputeHash(content) != storedHash {
				stale++
				if !quiet {
					fmt.Printf("  stale     %s\n", f.RelPath)
				}
			} else {
				current++
				if !quiet {
					fmt.Printf("  ok        %s\n", f.RelPath)
				}
			}
		}

		if !quiet {
			fmt.Printf("\n%d ok, %d stale, %d missing  (%d total)\n", current, stale, missing, stale+missing+current)
		}

		if stale > 0 || missing > 0 {
			os.Exit(1)
		}
		return nil
	},
}

func init() {
	checkCmd.Flags().Bool("quiet", false, "only print stale/missing files, suppress ok status")
	rootCmd.AddCommand(checkCmd)
}
