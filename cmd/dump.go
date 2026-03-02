package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/tristanmatthias/llmdoc/internal/dumper"
)

var dumpCmd = &cobra.Command{
	Use:   "dump [path]",
	Short: "Output an LLM-optimized summary of the codebase",
	Long: `dump collects all llmdoc annotations from the codebase and renders them
as a single document — suitable for pasting into an LLM context window.

Example:
  llmdoc dump . | pbcopy          # copy to clipboard (macOS)
  llmdoc dump . --output summary.md`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		root := rootArg(args)
		format, _ := cmd.Flags().GetString("format")
		output, _ := cmd.Flags().GetString("output")
		includeContent, _ := cmd.Flags().GetBool("include-content")
		noTree, _ := cmd.Flags().GetBool("no-tree")

		switch format {
		case "markdown", "xml", "plain":
			// ok
		default:
			return fmt.Errorf("unknown format %q — supported: markdown, xml, plain", format)
		}

		return dumper.Run(root, cfg, dumper.Options{
			Format:         format,
			IncludeContent: includeContent,
			NoTree:         noTree,
			Output:         output,
		})
	},
}

func init() {
	dumpCmd.Flags().String("format", "markdown", "output format: markdown, xml, plain")
	dumpCmd.Flags().StringP("output", "o", "", "write to file instead of stdout")
	dumpCmd.Flags().Bool("include-content", false, "include full file content in output")
	dumpCmd.Flags().Bool("no-tree", false, "omit directory tree from markdown output")
	rootCmd.AddCommand(dumpCmd)
}
