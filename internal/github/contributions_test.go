package github

import (
	"strings"
	"testing"
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
