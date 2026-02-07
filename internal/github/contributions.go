package github

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"regexp"
	"strconv"
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

// ScrapeAllContributions fetches the complete contribution history by scraping
// the GitHub profile page. This includes private contributions that aren't
// available via the API.
func ScrapeAllContributions(username string) ([]Contribution, error) {
	// First, get the user's account creation date
	createdAt, err := getUserCreatedAt(username)
	if err != nil {
		return nil, fmt.Errorf("failed to get account creation date: %w", err)
	}

	var allContributions []Contribution
	now := time.Now()
	currentYear := now.Year()
	startYear := createdAt.Year()

	// Fetch each year's contributions by scraping
	for year := startYear; year <= currentYear; year++ {
		contributions, err := scrapeContributionsForYear(username, year)
		if err != nil {
			return nil, fmt.Errorf("failed to scrape contributions for %d: %w", year, err)
		}

		allContributions = append(allContributions, contributions...)
	}

	return allContributions, nil
}

// scrapeContributionsForYear fetches contributions for a specific year by
// scraping the GitHub contributions page
func scrapeContributionsForYear(username string, year int) ([]Contribution, error) {
	url := fmt.Sprintf("https://github.com/users/%s/contributions?from=%d-01-01&to=%d-12-31",
		username, year, year)

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch contributions page: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	return parseContributionsFromHTML(string(body), year)
}

// parseContributionsFromHTML extracts contribution data from the HTML
func parseContributionsFromHTML(html string, year int) ([]Contribution, error) {
	var contributions []Contribution

	// Match tooltips like: >5 contributions on April 8th.</tool-tip>
	// or: >1 contribution on April 8th.</tool-tip>
	tooltipRegex := regexp.MustCompile(`>(\d+) contributions? on ([A-Za-z]+) (\d+)(?:st|nd|rd|th)\.</tool-tip>`)
	matches := tooltipRegex.FindAllStringSubmatch(html, -1)

	for _, match := range matches {
		if len(match) != 4 {
			continue
		}

		count, err := strconv.Atoi(match[1])
		if err != nil {
			continue
		}

		monthName := match[2]
		dayNum, err := strconv.Atoi(match[3])
		if err != nil {
			continue
		}

		// Convert month name to number
		month := monthNameToNumber(monthName)
		if month == 0 {
			continue
		}

		// Format date as YYYY-MM-DD
		date := fmt.Sprintf("%d-%02d-%02d", year, month, dayNum)

		contributions = append(contributions, Contribution{
			Date:  date,
			Count: count,
		})
	}

	return contributions, nil
}

// monthNameToNumber converts a month name to its number
func monthNameToNumber(name string) int {
	months := map[string]int{
		"January":   1,
		"February":  2,
		"March":     3,
		"April":     4,
		"May":       5,
		"June":      6,
		"July":      7,
		"August":    8,
		"September": 9,
		"October":   10,
		"November":  11,
		"December":  12,
	}
	return months[name]
}
