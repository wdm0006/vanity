package cli

import (
	"reflect"
	"testing"

	"github.com/wdm0006/vanity/internal/github"
	"github.com/wdm0006/vanity/internal/sync"
)

func TestSortedContributions(t *testing.T) {
	input := []github.Contribution{
		{Date: "2024-07-28", Count: 1},
		{Date: "2024-08-25", Count: 2},
		{Date: "2024-09-01", Count: 3},
		{Date: "2024-09-08", Count: 4},
		{Date: "2024-09-15", Count: 5},
		{Date: "2024-11-24", Count: 6},
		{Date: "2024-04-08", Count: 7},
		{Date: "2024-04-15", Count: 8},
	}

	got, total := sortedContributions(input)
	want := []sync.Contribution{
		{Date: "2024-04-08", Count: 7},
		{Date: "2024-04-15", Count: 8},
		{Date: "2024-07-28", Count: 1},
		{Date: "2024-08-25", Count: 2},
		{Date: "2024-09-01", Count: 3},
		{Date: "2024-09-08", Count: 4},
		{Date: "2024-09-15", Count: 5},
		{Date: "2024-11-24", Count: 6},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("sortedContributions() = %#v, want %#v", got, want)
	}
	if total != 36 {
		t.Fatalf("sortedContributions() total = %d, want 36", total)
	}
}
