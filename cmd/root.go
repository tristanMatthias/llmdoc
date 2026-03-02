package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/tristanmatthias/llmdoc/internal/config"
)

var (
	cfgPath string
	cfg     *config.Config
)

var rootCmd = &cobra.Command{
	Use:   "llmdoc",
	Short: "Annotate your codebase with LLM-generated file summaries",
	Long: `llmdoc recursively scans a codebase, generates concise LLM-powered summaries
for each source file, and stores them as structured comment blocks at the top of
each file. A hash in each block tracks changes so only modified files are re-annotated.

Run 'llmdoc annotate' to add/update summaries, or 'llmdoc dump' to get a
single LLM-ready view of your entire codebase.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Skip config loading for init command
		if cmd.Name() == "init" {
			return nil
		}
		var err error
		cfg, err = config.Load(cfgPath)
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		// Apply persistent flags as overrides
		if p, _ := cmd.Flags().GetString("provider"); p != "" {
			cfg.Provider = p
		}
		if m, _ := cmd.Flags().GetString("model"); m != "" {
			cfg.Model = m
		}
		if c, _ := cmd.Flags().GetInt("concurrency"); c > 0 {
			cfg.Concurrency = c
		}
		if f, _ := cmd.Flags().GetBool("force"); f {
			cfg.Force = f
		}
		return nil
	},
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// rootArg returns the first positional argument as the scan root, defaulting to ".".
func rootArg(args []string) string {
	if len(args) > 0 {
		return args[0]
	}
	return "."
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgPath, "config", "", "path to .llmdoc.yaml (default: auto-discover)")
	rootCmd.PersistentFlags().String("provider", "", "LLM provider: anthropic, openai")
	rootCmd.PersistentFlags().String("model", "", "model identifier (e.g. claude-opus-4-6, gpt-4o)")
	rootCmd.PersistentFlags().Int("concurrency", 0, "number of concurrent LLM calls")
	rootCmd.PersistentFlags().Bool("force", false, "re-annotate even when hash unchanged")
}
