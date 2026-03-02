package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/tristanmatthias/llmdoc/internal/updater"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update llmdoc to the latest version",
	Long: `update checks GitHub for a newer release and, if one is found, downloads
the binary for the current platform and replaces the running executable.

Use --check to see whether an update is available without installing it.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		checkOnly, _ := cmd.Flags().GetBool("check")

		fmt.Print("Checking for updates... ")
		latest, err := updater.LatestVersion()
		if err != nil {
			return err
		}
		fmt.Println(latest)

		current := rootCmd.Version
		if !updater.IsNewer(current, latest) {
			fmt.Printf("Already up to date (%s).\n", current)
			return nil
		}

		if checkOnly {
			fmt.Printf("Update available: %s → %s\n", current, latest)
			fmt.Println("Run `llmdoc update` to install.")
			return nil
		}

		fmt.Printf("Updating %s → %s...\n", current, latest)
		if err := updater.Update(latest); err != nil {
			return err
		}
		fmt.Printf("Updated to %s.\n", latest)
		return nil
	},
}

func init() {
	updateCmd.Flags().Bool("check", false, "check for a newer version without installing")
	rootCmd.AddCommand(updateCmd)
}
