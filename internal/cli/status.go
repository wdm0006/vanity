package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/wmcginnis/vanity/internal/github"
	syncpkg "github.com/wmcginnis/vanity/internal/sync"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show sync status",
	Long:  `Shows the current sync status including connected accounts and last sync time.`,
	RunE:  runStatus,
}

func runStatus(cmd *cobra.Command, args []string) error {
	// Check if .vanity exists
	if _, err := os.Stat(".vanity"); os.IsNotExist(err) {
		return fmt.Errorf("vanity not initialized (run 'vanity init' first)")
	}

	// Get current user
	username, err := github.GetCurrentUser()
	if err != nil {
		return fmt.Errorf("failed to get GitHub user: %w", err)
	}

	fmt.Printf("Current user: %s\n\n", username)

	// List all contribution files
	entries, err := os.ReadDir(".vanity")
	if err != nil {
		return fmt.Errorf("failed to read .vanity directory: %w", err)
	}

	var users []string
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".json") && !strings.HasSuffix(entry.Name(), "-state.json") {
			users = append(users, strings.TrimSuffix(entry.Name(), ".json"))
		}
	}

	if len(users) == 0 {
		fmt.Println("No synced users yet. Run 'vanity sync' to get started.")
		return nil
	}

	fmt.Println("Synced users:")
	for _, user := range users {
		contribPath := filepath.Join(".vanity", user+".json")
		data, err := os.ReadFile(contribPath)
		if err != nil {
			continue
		}

		var contribs syncpkg.ContributionData
		if err := json.Unmarshal(data, &contribs); err != nil {
			continue
		}

		totalContribs := 0
		for _, c := range contribs.Contributions {
			totalContribs += c.Count
		}

		marker := ""
		if user == username {
			marker = " (you)"
		}

		fmt.Printf("  - %s%s: %d contributions, last updated %s\n",
			user, marker, totalContribs, contribs.LastUpdated.Format("2006-01-02 15:04"))
	}

	// Show state info for current user
	statePath := filepath.Join(".vanity", username+"-state.json")
	if data, err := os.ReadFile(statePath); err == nil {
		var state syncpkg.SyncState
		if err := json.Unmarshal(data, &state); err == nil {
			fmt.Printf("\nLast sync: %s\n", state.LastSync.Format("2006-01-02 15:04"))

			if len(state.MirroredCounts) > 0 {
				fmt.Println("Mirrored from:")
				for user, dateCounts := range state.MirroredCounts {
					totalCommits := 0
					for _, count := range dateCounts {
						totalCommits += count
					}
					fmt.Printf("  - %s: %d dates, %d commits\n", user, len(dateCounts), totalCommits)
				}
			}
		}
	}

	return nil
}
