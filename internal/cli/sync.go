package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/wmcginnis/vanity/internal/sync"
)

var (
	dryRun bool
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync contributions with collaborators",
	Long: `Fetches your GitHub contributions, exports them to the shared repo,
imports other collaborators' contributions, and creates mirror commits.

The sync process:
  1. Pulls latest changes from the remote
  2. Fetches your contribution data via GitHub API (using gh CLI)
  3. Saves your contributions to .vanity/<username>.json
  4. Reads other collaborators' contribution files
  5. Creates backdated empty commits mirroring their activity
  6. Commits and pushes all changes

Syncs are incremental - only new contributions since your last sync are
processed. If a collaborator's contribution count for a day increases,
only the delta commits are created.`,
	Example: `  # Full sync
  vanity sync

  # Preview what would happen
  vanity sync --dry-run`,
	RunE: runSync,
}

func init() {
	syncCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be done without making changes")
}

func runSync(cmd *cobra.Command, args []string) error {
	engine, err := sync.NewEngine()
	if err != nil {
		return err
	}

	if dryRun {
		fmt.Println("Dry run mode - no changes will be made")
	}

	return engine.Sync(dryRun)
}
