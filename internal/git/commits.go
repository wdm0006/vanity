package git

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// IsGitRepo checks if the current directory is a git repository
func IsGitRepo() bool {
	_, err := os.Stat(".git")
	return err == nil
}

// Pull pulls the latest changes from the remote
func Pull() error {
	cmd := exec.Command("git", "pull", "--rebase")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// Push pushes changes to the remote
func Push() error {
	cmd := exec.Command("git", "push")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// HasRemote checks if the repository has a remote configured
func HasRemote() bool {
	cmd := exec.Command("git", "remote")
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return len(strings.TrimSpace(string(output))) > 0
}

// Add stages files for commit
func Add(paths ...string) error {
	args := append([]string{"add"}, paths...)
	cmd := exec.Command("git", args...)
	return cmd.Run()
}

// Commit creates a commit with the given message
func Commit(message string) error {
	cmd := exec.Command("git", "commit", "-m", message)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// CreateBackdatedCommit creates an empty commit with a specific date
// The date should be in ISO 8601 format (e.g., "2024-01-15T12:00:00")
func CreateBackdatedCommit(date string, message string) error {
	cmd := exec.Command("git", "commit", "--allow-empty", "-m", message)
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("GIT_AUTHOR_DATE=%s", date),
		fmt.Sprintf("GIT_COMMITTER_DATE=%s", date),
	)
	return cmd.Run()
}

// CreateBackdatedCommits creates multiple empty commits for a given date
// count specifies how many commits to create
func CreateBackdatedCommits(date string, count int, sourceUser string) error {
	for i := 0; i < count; i++ {
		// Spread commits throughout the day to make them look more natural
		hour := (i * 2) % 24
		timestamp := fmt.Sprintf("%sT%02d:00:00", date, hour)
		message := fmt.Sprintf("vanity: mirror from %s (%d/%d)", sourceUser, i+1, count)

		if err := CreateBackdatedCommit(timestamp, message); err != nil {
			return fmt.Errorf("failed to create commit %d/%d: %w", i+1, count, err)
		}
	}
	return nil
}

// GetCurrentBranch returns the name of the current branch
func GetCurrentBranch() (string, error) {
	cmd := exec.Command("git", "branch", "--show-current")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// HasUncommittedChanges checks if there are uncommitted changes
func HasUncommittedChanges() bool {
	cmd := exec.Command("git", "status", "--porcelain")
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return len(strings.TrimSpace(string(output))) > 0
}
