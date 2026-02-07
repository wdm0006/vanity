package cli

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/wmcginnis/vanity/internal/github"
	"github.com/wmcginnis/vanity/internal/sync"
)

var (
	scrapeContributions bool
)

var importCmd = &cobra.Command{
	Use:   "import <username>",
	Short: "Import contributions from another GitHub account",
	Long: `Imports contribution data from a public GitHub account that you don't
have access to (e.g., an old work account you can no longer log into).

This fetches the user's complete public contribution history (all years)
and saves it to the shared repo. On your next 'vanity sync', you'll create
mirror commits for their contributions.

By default, this uses the GitHub API which only returns public contributions.
Use --scrape to fetch all contributions (including private) by scraping the
profile page directly.`,
	Example: `  # Import public contributions only (via API)
  vanity import old-work-username

  # Import ALL contributions including private (via scraping)
  vanity import --scrape old-work-username

  # Then sync to create mirror commits
  vanity sync`,
	Args: cobra.ExactArgs(1),
	RunE: runImport,
}

func init() {
	importCmd.Flags().BoolVar(&scrapeContributions, "scrape", false, "Scrape contribution graph to include private contributions")
	rootCmd.AddCommand(importCmd)
}

func runImport(cmd *cobra.Command, args []string) error {
	username := args[0]

	// Check if .vanity exists
	if _, err := os.Stat(".vanity"); os.IsNotExist(err) {
		return fmt.Errorf("vanity not initialized (run 'vanity init' first)")
	}

	// Check if we're already syncing as this user
	currentUser, err := github.GetCurrentUser()
	if err != nil {
		return fmt.Errorf("failed to get current GitHub user: %w", err)
	}

	if username == currentUser {
		return fmt.Errorf("you're logged in as %s - use 'vanity sync' instead", username)
	}

	var contributions []github.Contribution

	if scrapeContributions {
		fmt.Printf("Scraping full contribution history from %s (including private)...\n", username)
		contributions, err = github.ScrapeAllContributions(username)
		if err != nil {
			return fmt.Errorf("failed to scrape contributions for %s: %w", username, err)
		}
	} else {
		fmt.Printf("Importing full contribution history from %s (public only)...\n", username)
		contributions, err = github.FetchAllContributions(username)
		if err != nil {
			return fmt.Errorf("failed to fetch contributions for %s: %w", username, err)
		}
	}

	if len(contributions) == 0 {
		if scrapeContributions {
			fmt.Printf("No contributions found for %s\n", username)
		} else {
			fmt.Printf("No contributions found for %s (profile may be private - try --scrape)\n", username)
		}
		return nil
	}

	// Convert to sync.Contribution type
	var syncContribs []sync.Contribution
	totalCount := 0
	for _, c := range contributions {
		syncContribs = append(syncContribs, sync.Contribution{
			Date:  c.Date,
			Count: c.Count,
		})
		totalCount += c.Count
	}

	// Save to contribution file
	contribData := &sync.ContributionData{
		Username:      username,
		LastUpdated:   time.Now(),
		Contributions: syncContribs,
	}

	if err := sync.SaveContributionData(contribData); err != nil {
		return fmt.Errorf("failed to save contribution data: %w", err)
	}

	fmt.Printf("Imported %d contributions across %d days from %s\n", totalCount, len(contributions), username)
	fmt.Println("\nNext steps:")
	fmt.Println("  1. Commit the changes: git add .vanity && git commit -m 'Import contributions from", username+"'")
	fmt.Println("  2. Run 'vanity sync' to create mirror commits")

	return nil
}
