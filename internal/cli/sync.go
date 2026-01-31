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
imports other collaborators' contributions, and creates mirror commits.`,
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
