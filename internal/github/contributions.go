package github

import (
	"encoding/json"
	"fmt"
	"os/exec"
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
			return "", fmt.Errorf("gh command failed: %s", string(exitErr.Stderr))
		}
		return "", fmt.Errorf("failed to run gh: %w (is gh CLI installed and authenticated?)", err)
	}

	username := string(output)
	if len(username) > 0 && username[len(username)-1] == '\n' {
		username = username[:len(username)-1]
	}
	return username, nil
}

// FetchContributions fetches contribution data for a user
// If since is not zero, only fetches contributions after that date
func FetchContributions(username string, since time.Time) ([]Contribution, error) {
	query := `
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

	cmd := exec.Command("gh", "api", "graphql",
		"-f", fmt.Sprintf("query=%s", query),
		"-f", fmt.Sprintf("user=%s", username))

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
