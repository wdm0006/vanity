package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "vanity",
	Short: "Sync GitHub contribution graphs across multiple accounts",
	Long: `Vanity is a CLI tool that helps you sync your GitHub contribution graph
across multiple accounts using a shared private repository.

Each collaborator runs 'vanity sync' to:
1. Export their contribution data to the shared repo
2. Import other collaborators' contribution data
3. Create mirror commits so everyone's graph shows combined activity`,
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(syncCmd)
	rootCmd.AddCommand(statusCmd)
}

func exitWithError(msg string) {
	fmt.Fprintln(os.Stderr, "Error:", msg)
	os.Exit(1)
}
