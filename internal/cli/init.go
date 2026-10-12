package cli

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

//go:embed templates/vanity-sync.yml
var syncWorkflowTemplate []byte

const syncWorkflowPath = ".github/workflows/vanity-sync.yml"

var initWithWorkflow bool

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize vanity in the current repository",
	Long: `Creates a .vanity/ directory to store contribution data and sync state.

Run this command once in your shared private repository before syncing.
Each collaborator should clone the repo and run 'vanity sync' to join.`,
	Example: `  # Initialize in current directory
  vanity init

  # Then sync your contributions
  vanity sync

  # Also scaffold a scheduled GitHub Actions sync workflow
  vanity init --with-workflow`,
	RunE: runInit,
}

func init() {
	initCmd.Flags().BoolVar(&initWithWorkflow, "with-workflow", false, "Also write "+syncWorkflowPath)
}

func writeSyncWorkflow() error {
	if _, err := os.Stat(syncWorkflowPath); err == nil {
		fmt.Println(syncWorkflowPath, "already exists; leaving it unchanged")
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to check %s: %w", syncWorkflowPath, err)
	}
	if err := os.MkdirAll(filepath.Dir(syncWorkflowPath), 0755); err != nil {
		return fmt.Errorf("failed to create workflow directory: %w", err)
	}
	if err := os.WriteFile(syncWorkflowPath, syncWorkflowTemplate, 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", syncWorkflowPath, err)
	}
	fmt.Println("Wrote", syncWorkflowPath)
	fmt.Println("Add the VANITY_TOKEN secret (a PAT for this account) and the VANITY_GIT_NAME / VANITY_GIT_EMAIL variables.")
	return nil
}

func runInit(cmd *cobra.Command, args []string) error {
	// Check if we're in a git repo
	if _, err := os.Stat(".git"); os.IsNotExist(err) {
		return fmt.Errorf("not a git repository (run this from the root of your vanity repo)")
	}

	// Create .vanity directory
	vanityDir := ".vanity"
	if err := os.MkdirAll(vanityDir, 0755); err != nil {
		return fmt.Errorf("failed to create %s directory: %w", vanityDir, err)
	}

	// Create .gitkeep to ensure directory is tracked
	gitkeepPath := filepath.Join(vanityDir, ".gitkeep")
	if _, err := os.Stat(gitkeepPath); os.IsNotExist(err) {
		if err := os.WriteFile(gitkeepPath, []byte{}, 0644); err != nil {
			return fmt.Errorf("failed to create .gitkeep: %w", err)
		}
	}

	fmt.Println("Initialized vanity in", vanityDir)
	if initWithWorkflow {
		if err := writeSyncWorkflow(); err != nil {
			return err
		}
	}
	fmt.Println("\nNext steps:")
	fmt.Println("  1. Invite collaborators to this repository")
	fmt.Println("  2. Run 'vanity sync' to sync your contributions")
	return nil
}
