package github

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseScrapedContributions(t *testing.T) {
	tests := []struct {
		name              string
		html              string
		want              []Contribution
		wantErrorContains string
	}{
		{
			name:              "positive total without recognized tooltips",
			html:              `<h2>2,510 contributions in 2023</h2><tool-tip>changed markup</tool-tip>`,
			wantErrorContains: "parsed 0 days but reports 2510 contributions",
		},
		{
			name: "zero total without tooltips",
			html: `<h2>0 contributions in 2023</h2>`,
		},
		{
			name: "absent total without tooltips",
			html: `<div>No contribution summary</div>`,
		},
		{
			name: "recognized tooltips",
			html: `<h2>6 contributions in 2023</h2>
				<tool-tip>5 contributions on April 8th.</tool-tip>
				<tool-tip>1 contribution on December 21st.</tool-tip>`,
			want: []Contribution{
				{Date: "2023-04-08", Count: 5},
				{Date: "2023-12-21", Count: 1},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseScrapedContributions(tt.html, 2023)
			if tt.wantErrorContains != "" {
				if err == nil {
					t.Fatal("expected an error, got nil")
				}
				if !strings.Contains(err.Error(), tt.wantErrorContains) {
					t.Fatalf("error = %q, want it to contain %q", err, tt.wantErrorContains)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("got %d contributions, want %d: %#v", len(got), len(tt.want), got)
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Errorf("contribution %d = %#v, want %#v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

// stubGitHubCLI puts a `gh` on PATH that prints the given GraphQL response body.
func stubGitHubCLI(t *testing.T, response string) {
	t.Helper()

	dir := t.TempDir()
	script := fmt.Sprintf("#!/bin/sh\ncat <<'JSON'\n%s\nJSON\n", response)
	path := filepath.Join(dir, "gh")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("failed to write gh stub: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestFetchContributionsSinceFilter(t *testing.T) {
	const response = `{"data":{"user":{"contributionsCollection":{"contributionCalendar":{"weeks":[
		{"contributionDays":[
			{"date":"2024-01-14","contributionCount":7},
			{"date":"2024-01-15","contributionCount":5},
			{"date":"2024-01-16","contributionCount":0},
			{"date":"2024-01-17","contributionCount":3}
		]}
	]}}}}}`

	tests := []struct {
		name  string
		since time.Time
		want  []Contribution
	}{
		{
			// Catches the original bug: the last-synced day was skipped because
			// its midnight was Before the wall-clock timestamp.
			name:  "same day as since is re-fetched (UTC)",
			since: time.Date(2024, 1, 15, 14, 0, 0, 0, time.UTC),
			want: []Contribution{
				{Date: "2024-01-15", Count: 5},
				{Date: "2024-01-17", Count: 3},
			},
		},
		{
			// Catches truncating since in its own location instead of UTC: for a
			// negative offset that still lands after the UTC-parsed day.
			name:  "same day as since is re-fetched (negative UTC offset)",
			since: time.Date(2024, 1, 15, 14, 0, 0, 0, time.FixedZone("EST", -5*60*60)),
			want: []Contribution{
				{Date: "2024-01-15", Count: 5},
				{Date: "2024-01-17", Count: 3},
			},
		},
		{
			// Catches dropping the filter entirely: earlier days must stay out.
			name:  "days before since are skipped",
			since: time.Date(2024, 1, 17, 9, 30, 0, 0, time.UTC),
			want: []Contribution{
				{Date: "2024-01-17", Count: 3},
			},
		},
		{
			name:  "zero since returns every non-zero day",
			since: time.Time{},
			want: []Contribution{
				{Date: "2024-01-14", Count: 7},
				{Date: "2024-01-15", Count: 5},
				{Date: "2024-01-17", Count: 3},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stubGitHubCLI(t, response)

			got, err := FetchContributions("someone", tt.since)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("got %d contributions, want %d: %#v", len(got), len(tt.want), got)
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Errorf("contribution %d = %#v, want %#v", i, got[i], tt.want[i])
				}
			}
		})
	}
}
