package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/wdm0006/vanity/internal/github"
	syncpkg "github.com/wdm0006/vanity/internal/sync"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show sync status",
	Long: `Shows the current sync status including connected accounts and last sync time.

Displays:
  - Your GitHub username (via gh CLI)
  - All synced users and their contribution counts
  - When each user last synced
  - How many contributions you've mirrored from each user`,
	Example: `  vanity status
  vanity status --json`,
	RunE: runStatus,
}

var statusJSON bool

func init() {
	statusCmd.Flags().BoolVar(&statusJSON, "json", false, "Print status as a single JSON document")
}

type statusReport struct {
	CurrentUser string          `json:"current_user"`
	Accounts    []statusAccount `json:"accounts"`
	Mirrored    []statusMirror  `json:"mirrored"`
}

type statusAccount struct {
	Username    string    `json:"username"`
	Total       int       `json:"total"`
	LastUpdated time.Time `json:"last_updated"`
}

type statusMirror struct {
	Source string `json:"source"`
	Dates  int    `json:"dates"`
	Total  int    `json:"total"`
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

	report := statusReport{CurrentUser: username, Accounts: []statusAccount{}, Mirrored: []statusMirror{}}

	if !statusJSON {
		fmt.Printf("Current user: %s\n\n", username)
	}

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
		if statusJSON {
			return writeStatusJSON(report)
		}
		fmt.Println("No synced users yet. Run 'vanity sync' to get started.")
		return nil
	}

	if !statusJSON {
		fmt.Println("Synced users:")
	}
	var failures []error
	for _, user := range users {
		contribPath := filepath.Join(".vanity", user+".json")
		data, err := os.ReadFile(contribPath)
		if err != nil {
			failures = append(failures, fmt.Errorf("%s: read contribution data: %w", contribPath, err))
			continue
		}

		var contribs syncpkg.ContributionData
		if err := json.Unmarshal(data, &contribs); err != nil {
			failures = append(failures, fmt.Errorf("%s: decode contribution data: %w", contribPath, err))
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

		report.Accounts = append(report.Accounts, statusAccount{Username: user, Total: totalContribs, LastUpdated: contribs.LastUpdated})
		if !statusJSON {
			fmt.Printf("  - %s%s: %d contributions, last updated %s\n",
				user, marker, totalContribs, contribs.LastUpdated.Format("2006-01-02 15:04"))
		}
	}

	// Show state info for current user
	statePath := filepath.Join(".vanity", username+"-state.json")
	if data, err := os.ReadFile(statePath); err == nil {
		var state syncpkg.SyncState
		if err := json.Unmarshal(data, &state); err != nil {
			failures = append(failures, fmt.Errorf("%s: decode sync state: %w", statePath, err))
		} else {
			if !statusJSON {
				fmt.Printf("\nLast sync: %s\n", state.LastSync.Format("2006-01-02 15:04"))
			}

			if len(state.MirroredCounts) > 0 {
				if !statusJSON {
					fmt.Println("Mirrored from:")
				}
				for user, dateCounts := range state.MirroredCounts {
					totalCommits := 0
					for _, count := range dateCounts {
						totalCommits += count
					}
					report.Mirrored = append(report.Mirrored, statusMirror{Source: user, Dates: len(dateCounts), Total: totalCommits})
					if !statusJSON {
						fmt.Printf("  - %s: %d dates, %d commits\n", user, len(dateCounts), totalCommits)
					}
				}
			}
		}
	} else if !os.IsNotExist(err) {
		failures = append(failures, fmt.Errorf("%s: read sync state: %w", statePath, err))
	}

	if statusJSON {
		sort.Slice(report.Mirrored, func(i, j int) bool { return report.Mirrored[i].Source < report.Mirrored[j].Source })
		if err := writeStatusJSON(report); err != nil {
			failures = append(failures, err)
		}
	}

	return errors.Join(failures...)
}

func writeStatusJSON(report statusReport) error {
	out, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("encode status: %w", err)
	}
	fmt.Println(string(out))
	return nil
}
