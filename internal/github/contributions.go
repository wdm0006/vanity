package github

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type ContributionDay struct {
	Date              string `json:"date"`
	ContributionCount int    `json:"contributionCount"`
}

type ContributionWeek struct {
	ContributionDays []ContributionDay `json:"contributionDays"`
}

type ContributionCalendar struct {
	Weeks []ContributionWeek `json:"weeks"`
}

type ContributionsCollection struct {
	ContributionCalendar ContributionCalendar `json:"contributionCalendar"`
}

type UserData struct {
	ContributionsCollection ContributionsCollection `json:"contributionsCollection"`
}

type GraphQLResponse struct {
	Data struct {
		User UserData `json:"user"`
	} `json:"data"`
}

// Contribution represents a single day's contribution count
type Contribution struct {
	Date  string `json:"date"`
	Count int    `json:"count"`
}

// GetCurrentUser returns the currently authenticated GitHub username
func GetCurrentUser() (string, error) {
	cmd := exec.Command("gh", "api", "user", "--jq", ".login")
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderr := string(exitErr.Stderr)
			// Check for common auth errors
			if strings.Contains(stderr, "auth login") || strings.Contains(stderr, "not logged") {
				return "", fmt.Errorf("not authenticated with GitHub CLI\n\nRun: gh auth login")
			}
			return "", fmt.Errorf("gh command failed: %s", stderr)
		}
		// gh not found in PATH
		if execErr, ok := err.(*exec.Error); ok && execErr.Err == exec.ErrNotFound {
			return "", fmt.Errorf("GitHub CLI (gh) not found\n\nInstall it from: https://cli.github.com\nThen run: gh auth login")
		}
		return "", fmt.Errorf("failed to run gh: %w", err)
	}

	username := string(output)
	if len(username) > 0 && username[len(username)-1] == '\n' {
		username = username[:len(username)-1]
	}
	return username, nil
}

// FetchContributions fetches contribution data for a user (last year only)
// If since is not zero, only fetches contributions after that date
func FetchContributions(username string, since time.Time) ([]Contribution, error) {
	return fetchContributionsForYear(username, time.Time{}, time.Time{}, since)
}

// FetchAllContributions fetches the complete contribution history for a user
// by iterating through years from their account creation date
func FetchAllContributions(username string) ([]Contribution, error) {
	// First, get the user's account creation date
	createdAt, err := getUserCreatedAt(username)
	if err != nil {
		return nil, fmt.Errorf("failed to get account creation date: %w", err)
	}

	var allContributions []Contribution
	now := time.Now()
	currentYear := now.Year()
	startYear := createdAt.Year()

	// Fetch each year's contributions
	for year := startYear; year <= currentYear; year++ {
		from := time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)
		to := time.Date(year, 12, 31, 23, 59, 59, 0, time.UTC)

		// Don't go past today
		if to.After(now) {
			to = now
		}

		contributions, err := fetchContributionsForYear(username, from, to, time.Time{})
		if err != nil {
			return nil, fmt.Errorf("failed to fetch contributions for %d: %w", year, err)
		}

		allContributions = append(allContributions, contributions...)
	}

	return allContributions, nil
}

// getUserCreatedAt fetches the account creation date for a user
func getUserCreatedAt(username string) (time.Time, error) {
	cmd := exec.Command("gh", "api", fmt.Sprintf("users/%s", username), "--jq", ".created_at")
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return time.Time{}, fmt.Errorf("gh api failed: %s", string(exitErr.Stderr))
		}
		return time.Time{}, err
	}

	dateStr := strings.TrimSpace(string(output))
	createdAt, err := time.Parse(time.RFC3339, dateStr)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to parse date %s: %w", dateStr, err)
	}

	return createdAt, nil
}

// fetchContributionsForYear fetches contributions for a specific date range
func fetchContributionsForYear(username string, from, to time.Time, since time.Time) ([]Contribution, error) {
	var query string
	var cmd *exec.Cmd

	if from.IsZero() || to.IsZero() {
		// Default query (last year)
		query = `
query($user: String!) {
  user(login: $user) {
    contributionsCollection {
      contributionCalendar {
        weeks {
          contributionDays {
            date
            contributionCount
          }
        }
      }
    }
  }
}`
		cmd = exec.Command("gh", "api", "graphql",
			"-f", fmt.Sprintf("query=%s", query),
			"-f", fmt.Sprintf("user=%s", username))
	} else {
		// Query with date range
		query = `
query($user: String!, $from: DateTime!, $to: DateTime!) {
  user(login: $user) {
    contributionsCollection(from: $from, to: $to) {
      contributionCalendar {
        weeks {
          contributionDays {
            date
            contributionCount
          }
        }
      }
    }
  }
}`
		cmd = exec.Command("gh", "api", "graphql",
			"-f", fmt.Sprintf("query=%s", query),
			"-f", fmt.Sprintf("user=%s", username),
			"-f", fmt.Sprintf("from=%s", from.Format(time.RFC3339)),
			"-f", fmt.Sprintf("to=%s", to.Format(time.RFC3339)))
	}

	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("gh graphql failed: %s", string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("failed to run gh: %w", err)
	}

	var resp GraphQLResponse
	if err := json.Unmarshal(output, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	var contributions []Contribution
	for _, week := range resp.Data.User.ContributionsCollection.ContributionCalendar.Weeks {
		for _, day := range week.ContributionDays {
			if day.ContributionCount == 0 {
				continue
			}

			// Filter by since date if provided
			if !since.IsZero() {
				dayDate, err := time.Parse("2006-01-02", day.Date)
				if err != nil {
					continue
				}
				if dayDate.Before(since) || dayDate.Equal(since) {
					continue
				}
			}

			contributions = append(contributions, Contribution{
				Date:  day.Date,
				Count: day.ContributionCount,
			})
		}
	}

	return contributions, nil
}
