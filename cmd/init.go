package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/tristanmatthias/llmdoc/internal/config"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Write a starter .llmdoc.yaml to the current directory",
	RunE: func(cmd *cobra.Command, args []string) error {
		force, _ := cmd.Flags().GetBool("force")
		target := ".llmdoc.yaml"

		if _, err := os.Stat(target); err == nil && !force {
			return fmt.Errorf("%s already exists (use --force to overwrite)", target)
		}

		if err := os.WriteFile(target, []byte(config.StarterYAML()), 0644); err != nil {
			return fmt.Errorf("writing %s: %w", target, err)
		}

		fmt.Printf("Created %s\n", target)
		fmt.Println("Edit it to set your LLM provider, API key, and ignore patterns.")
		return nil
	},
}

func init() {
	initCmd.Flags().Bool("force", false, "overwrite existing config")
	rootCmd.AddCommand(initCmd)
}
