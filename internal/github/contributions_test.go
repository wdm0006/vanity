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

// realPage2024 is a trimmed but byte-for-byte capture of GitHub's own
// contributions markup — the hand-written fixtures above use single-spaced
// snippets that do not resemble what the endpoint actually serves.
func realPage2024(t *testing.T) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("testdata", "contributions-2024.html"))
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}
	return string(body)
}

func TestParseContributionsFromHTMLReadsRealMarkup(t *testing.T) {
	html := realPage2024(t)

	got, err := parseContributionsFromHTML(html, 2024)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Every ordinal suffix appears in both its singular ("1 contribution") and
	// plural ("N contributions") form; the four "No contributions on ..." days
	// in the fixture must not match at all.
	want := []Contribution{
		{Date: "2024-01-30", Count: 2},  // th, plural, first month
		{Date: "2024-04-23", Count: 1},  // rd, singular
		{Date: "2024-06-21", Count: 1},  // st, singular
		{Date: "2024-07-02", Count: 1},  // nd, singular
		{Date: "2024-07-22", Count: 2},  // nd, plural
		{Date: "2024-08-05", Count: 1},  // th, singular
		{Date: "2024-09-01", Count: 8},  // st, plural
		{Date: "2024-09-23", Count: 24}, // rd, plural
		{Date: "2024-12-17", Count: 32}, // th, plural, last month
	}

	if len(got) != len(want) {
		t.Fatalf("got %d contributions, want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("contribution %d = %#v, want %#v", i, got[i], want[i])
		}
	}

	// The parser reconstructs the date from the tooltip's month name and never
	// reads the day cell's data-date, so cross-checking against that attribute
	// catches a wrong entry in the month table independently of the table above.
	for _, c := range got {
		if !strings.Contains(html, `data-date="`+c.Date+`"`) {
			t.Errorf("parsed date %q has no matching data-date attribute in the page", c.Date)
		}
	}
}

func TestParseContributionsFromHTMLIgnoresUnrecognizedMarkup(t *testing.T) {
	tests := []struct {
		name string
		html string
	}{
		{name: "empty document", html: ""},
		{name: "zero-count tooltip", html: `<tool-tip class="sr-only">No contributions on January 1st.</tool-tip>`},
		{name: "missing ordinal suffix", html: `<tool-tip class="sr-only">5 contributions on April 8.</tool-tip>`},
		{name: "renamed element", html: `<span class="sr-only">5 contributions on April 8th.</span>`},
		{name: "localized month name", html: `<tool-tip class="sr-only">5 contributions on Abril 8th.</tool-tip>`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseContributionsFromHTML(tt.html, 2024)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != 0 {
				t.Fatalf("got %d contributions, want none: %#v", len(got), got)
			}
		})
	}
}

func TestMonthNameToNumber(t *testing.T) {
	want := map[string]int{
		"January": 1, "February": 2, "March": 3, "April": 4,
		"May": 5, "June": 6, "July": 7, "August": 8,
		"September": 9, "October": 10, "November": 11, "December": 12,
		// Unrecognized names resolve to 0, which parseContributionsFromHTML
		// treats as "skip this tooltip".
		"":           0,
		"Jan":        0,
		"january":    0,
		"JANUARY":    0,
		"Enero":      0,
		"Smarch":     0,
		"September ": 0,
	}

	for name, expected := range want {
		if got := monthNameToNumber(name); got != expected {
			t.Errorf("monthNameToNumber(%q) = %d, want %d", name, got, expected)
		}
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
			name:  "same day as since is re-fetched (UTC)",
			since: time.Date(2024, 1, 15, 14, 0, 0, 0, time.UTC),
			want: []Contribution{
				{Date: "2024-01-15", Count: 5},
				{Date: "2024-01-17", Count: 3},
			},
		},
		{
			name:  "same day as since is re-fetched (negative UTC offset)",
			since: time.Date(2024, 1, 15, 14, 0, 0, 0, time.FixedZone("EST", -5*60*60)),
			want: []Contribution{
				{Date: "2024-01-15", Count: 5},
				{Date: "2024-01-17", Count: 3},
			},
		},
		{
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
