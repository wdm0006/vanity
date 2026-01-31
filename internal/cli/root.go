package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var version = "dev"

var rootCmd = &cobra.Command{
	Use:   "vanity",
	Short: "Sync GitHub contribution graphs across multiple accounts",
	Long: `Vanity is a CLI tool that helps you sync your GitHub contribution graph
across multiple accounts using a shared private repository.

Each collaborator runs 'vanity sync' to:
  1. Export their contribution data to the shared repo
  2. Import other collaborators' contribution data
  3. Create mirror commits so everyone's graph shows combined activity

Only contribution dates and counts are shared - no commit messages,
code, or repository names are ever exposed.

Prerequisites:
  - GitHub CLI (gh) installed and authenticated
  - A shared private repository with collaborators added`,
	Example: `  # First time setup (in your shared repo)
  vanity init
  vanity sync

  # Check status
  vanity status

  # Preview changes without syncing
  vanity sync --dry-run`,
}

// SetVersion sets the version string (called from main)
func SetVersion(v string) {
	version = v
	rootCmd.Version = v
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
