package sync

import (
	"testing"

	"github.com/wdm0006/vanity/internal/github"
)

func TestMergeContributionsSortsByDate(t *testing.T) {
	e := &Engine{username: "alice"}

	existing := &ContributionData{
		Username: "alice",
		Contributions: []Contribution{
			{Date: "2024-03-10", Count: 3},
			{Date: "2024-01-05", Count: 1},
			{Date: "2024-02-20", Count: 7},
		},
	}

	new := []github.Contribution{
		{Date: "2024-04-01", Count: 4},
		{Date: "2024-01-05", Count: 9}, // overwrites the existing count
		{Date: "2024-02-01", Count: 2},
	}

	merged := e.mergeContributions(existing, new)

	want := []Contribution{
		{Date: "2024-01-05", Count: 9},
		{Date: "2024-02-01", Count: 2},
		{Date: "2024-02-20", Count: 7},
		{Date: "2024-03-10", Count: 3},
		{Date: "2024-04-01", Count: 4},
	}

	if len(merged.Contributions) != len(want) {
		t.Fatalf("got %d contributions, want %d: %v", len(merged.Contributions), len(want), merged.Contributions)
	}
	for i, w := range want {
		got := merged.Contributions[i]
		if got != w {
			t.Errorf("contribution %d: got %+v, want %+v", i, got, w)
		}
	}

	if merged.Username != "alice" {
		t.Errorf("got username %q, want %q", merged.Username, "alice")
	}
}
