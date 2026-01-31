package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize vanity in the current repository",
	Long:  `Creates a .vanity/ directory to store contribution data and sync state.`,
	RunE:  runInit,
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
	fmt.Println("\nNext steps:")
	fmt.Println("  1. Invite collaborators to this repository")
	fmt.Println("  2. Run 'vanity sync' to sync your contributions")
	return nil
}
