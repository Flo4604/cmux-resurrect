package cmd

import (
	"fmt"

	"github.com/drolosoft/cmux-resurrect/internal/orchestrate"
	"github.com/spf13/cobra"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update crex to the latest version",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("  Checking for updates...")

		result := orchestrate.RunUpdate(Version)

		if result.Err != nil && result.NewVersion == "" {
			fmt.Printf("  ✗ %v\n", result.Err)
			return
		}

		fmt.Printf("  Current: %s  Latest: %s\n", result.OldVersion, result.NewVersion)

		if result.AlreadyLatest {
			fmt.Println("  ✓ Already at the latest version.")
			return
		}

		fmt.Printf("  Detected install method: %s\n", result.Method)

		if result.Method == orchestrate.InstallManual {
			fmt.Printf("  Download the latest release at:\n  %s\n", result.ManualURL)
			return
		}

		fmt.Printf("  Updating...\n")

		if result.Err != nil {
			fmt.Printf("  ✗ Update failed: %v\n", result.Err)
			if result.Output != "" {
				fmt.Println(result.Output)
			}
			return
		}

		fmt.Printf("  ✓ Updated to %s\n", result.NewVersion)
	},
}

func init() {
	rootCmd.AddCommand(updateCmd)
}
