package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var renameCmd = &cobra.Command{
	Use:   "rename <old> <new>",
	Short: "Rename a saved layout",
	Args:  cobra.ExactArgs(2),
	RunE:  runRename,
}

func init() {
	renameCmd.ValidArgsFunction = completeLayoutNames
	rootCmd.AddCommand(renameCmd)
}

func runRename(cmd *cobra.Command, args []string) error {
	oldName, newName := args[0], args[1]
	store, err := newStore()
	if err != nil {
		return err
	}

	if err := store.Rename(oldName, newName); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "Renamed %q → %q\n", oldName, newName)
	return nil
}
