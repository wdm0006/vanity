package sync

import (
	"fmt"
	"os"
	"time"

	"github.com/wmcginnis/vanity/internal/git"
	"github.com/wmcginnis/vanity/internal/github"
)

// Engine handles the sync process
type Engine struct {
	username string
}

// NewEngine creates a new sync engine
func NewEngine() (*Engine, error) {
	// Check prerequisites
	if !git.IsGitRepo() {
		return nil, fmt.Errorf("not a git repository")
	}

	if _, err := os.Stat(vanityDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("vanity not initialized (run 'vanity init' first)")
	}

	username, err := github.GetCurrentUser()
	if err != nil {
		return nil, fmt.Errorf("failed to get GitHub user: %w", err)
	}

	return &Engine{username: username}, nil
}

// Sync performs the full sync process
func (e *Engine) Sync(dryRun bool) error {
	fmt.Printf("Syncing as %s...\n\n", e.username)

	// Step 1: Pull latest changes
	if git.HasRemote() {
		fmt.Println("Pulling latest changes...")
		if !dryRun {
			if err := git.Pull(); err != nil {
				fmt.Printf("Warning: git pull failed: %v\n", err)
			}
		}
	}

	// Step 2: Load current state
	state, err := LoadSyncState(e.username)
	if err != nil {
		return fmt.Errorf("failed to load sync state: %w", err)
	}

	// Step 3: Fetch own contributions
	fmt.Println("Fetching your contributions from GitHub...")
	contributions, err := github.FetchContributions(e.username, state.LastSync)
	if err != nil {
		return fmt.Errorf("failed to fetch contributions: %w", err)
	}
	fmt.Printf("  Found %d contribution days\n", len(contributions))

	// Step 4: Update own contribution data
	contribData, err := LoadContributionData(e.username)
	if err != nil {
		return fmt.Errorf("failed to load contribution data: %w", err)
	}

	// Merge new contributions
	contribData = e.mergeContributions(contribData, contributions)
	contribData.LastUpdated = time.Now()

	if !dryRun {
		if err := SaveContributionData(contribData); err != nil {
			return fmt.Errorf("failed to save contribution data: %w", err)
		}
	}
	fmt.Printf("  Updated %s.json with %d total contribution days\n", e.username, len(contribData.Contributions))

	// Step 5: Mirror other users' contributions
	users, err := ListSyncedUsers()
	if err != nil {
		return fmt.Errorf("failed to list synced users: %w", err)
	}

	totalMirrored := 0
	for _, user := range users {
		if user == e.username {
			continue
		}

		mirrored, err := e.mirrorUser(user, state, dryRun)
		if err != nil {
			fmt.Printf("Warning: failed to mirror %s: %v\n", user, err)
			continue
		}
		totalMirrored += mirrored
	}

	if totalMirrored > 0 {
		fmt.Printf("\nCreated %d mirror commits\n", totalMirrored)
	} else if len(users) > 1 {
		fmt.Println("\nNo new contributions to mirror")
	}

	// Step 6: Update and save state
	state.LastSync = time.Now()
	if !dryRun {
		if err := SaveSyncState(state); err != nil {
			return fmt.Errorf("failed to save sync state: %w", err)
		}
	}

	// Step 7: Commit changes
	if !dryRun && git.HasUncommittedChanges() {
		fmt.Println("\nCommitting changes...")
		if err := git.Add(".vanity/"); err != nil {
			return fmt.Errorf("failed to stage changes: %w", err)
		}
		if err := git.Commit(fmt.Sprintf("vanity: sync %s", e.username)); err != nil {
			return fmt.Errorf("failed to commit: %w", err)
		}
	}

	// Step 8: Push changes
	if !dryRun && git.HasRemote() {
		fmt.Println("Pushing changes...")
		if err := git.Push(); err != nil {
			return fmt.Errorf("failed to push: %w", err)
		}
	}

	fmt.Println("\nSync complete!")
	return nil
}

// mergeContributions merges new contributions into existing data
func (e *Engine) mergeContributions(existing *ContributionData, new []github.Contribution) *ContributionData {
	// Create a map of existing contributions by date
	byDate := make(map[string]int)
	for _, c := range existing.Contributions {
		byDate[c.Date] = c.Count
	}

	// Add new contributions (overwrite if date exists)
	for _, c := range new {
		byDate[c.Date] = c.Count
	}

	// Convert back to slice
	var contributions []Contribution
	for date, count := range byDate {
		contributions = append(contributions, Contribution{
			Date:  date,
			Count: count,
		})
	}

	return &ContributionData{
		Username:      existing.Username,
		LastUpdated:   time.Now(),
		Contributions: contributions,
	}
}

// mirrorUser creates mirror commits for another user's contributions
func (e *Engine) mirrorUser(sourceUser string, state *SyncState, dryRun bool) (int, error) {
	contribData, err := LoadContributionData(sourceUser)
	if err != nil {
		return 0, err
	}

	mirrored := 0
	for _, contrib := range contribData.Contributions {
		// Get how many we've already mirrored for this date
		alreadyMirrored := state.GetMirroredCount(sourceUser, contrib.Date)

		// Calculate how many new commits we need
		delta := contrib.Count - alreadyMirrored
		if delta <= 0 {
			continue
		}

		if dryRun {
			fmt.Printf("  Would create %d commits for %s from %s (had %d, now %d)\n",
				delta, contrib.Date, sourceUser, alreadyMirrored, contrib.Count)
		} else {
			if err := git.CreateBackdatedCommits(contrib.Date, delta, sourceUser); err != nil {
				return mirrored, fmt.Errorf("failed to create commits for %s: %w", contrib.Date, err)
			}
		}

		// Update the mirrored count to the current total
		state.SetMirroredCount(sourceUser, contrib.Date, contrib.Count)
		mirrored += delta
	}

	if mirrored > 0 {
		fmt.Printf("  Mirrored %d contributions from %s\n", mirrored, sourceUser)
	}

	return mirrored, nil
}
