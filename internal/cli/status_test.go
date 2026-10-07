package cli

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunStatusReportsValidAccountsAndMalformedContributions(t *testing.T) {
	repo := setupStatusTest(t)
	writeStatusFile(t, repo, ".vanity/alice.json", `{
  "username": "alice",
  "last_updated": "2024-04-05T12:30:00Z",
  "contributions": [
    {"date": "2024-04-04", "count": 7},
    {"date": "2024-04-05", "count": 5}
  ]
}`)
	writeStatusFile(t, repo, ".vanity/broken.json", `{"contributions":`)
	writeStatusFile(t, repo, ".vanity/corrupt.json", `not json`)

	output, err := captureStatusOutput(t, func() error {
		return runStatus(statusCmd, nil)
	})
	if err == nil {
		t.Fatal("runStatus() error = nil, want malformed contribution error")
	}
	if !strings.Contains(output, "  - alice (you): 12 contributions, last updated 2024-04-05 12:30\n") {
		t.Fatalf("runStatus() output missing exact valid account row:\n%s", output)
	}
	for _, path := range []string{".vanity/broken.json", ".vanity/corrupt.json"} {
		if !strings.Contains(err.Error(), path) {
			t.Fatalf("runStatus() error = %q, want path %s", err, path)
		}
	}
}

func TestRunStatusReportsMalformedCurrentUserState(t *testing.T) {
	repo := setupStatusTest(t)
	writeStatusFile(t, repo, ".vanity/alice.json", `{
  "username": "alice",
  "last_updated": "2024-04-05T12:30:00Z",
  "contributions": [{"date": "2024-04-05", "count": 9}]
}`)
	writeStatusFile(t, repo, ".vanity/alice-state.json", `{"mirrored_counts":`)

	output, err := captureStatusOutput(t, func() error {
		return runStatus(statusCmd, nil)
	})
	if err == nil {
		t.Fatal("runStatus() error = nil, want malformed state error")
	}
	if !strings.Contains(output, "  - alice (you): 9 contributions, last updated 2024-04-05 12:30\n") {
		t.Fatalf("runStatus() output missing exact valid account row:\n%s", output)
	}
	if !strings.Contains(err.Error(), ".vanity/alice-state.json") {
		t.Fatalf("runStatus() error = %q, want path .vanity/alice-state.json", err)
	}
}

func TestRunStatusAllowsMissingCurrentUserState(t *testing.T) {
	repo := setupStatusTest(t)
	writeStatusFile(t, repo, ".vanity/alice.json", `{
  "username": "alice",
  "last_updated": "2024-04-05T12:30:00Z",
  "contributions": [{"date": "2024-04-05", "count": 4}]
}`)

	output, err := captureStatusOutput(t, func() error {
		return runStatus(statusCmd, nil)
	})
	if err != nil {
		t.Fatalf("runStatus() error = %v, want nil for missing state", err)
	}
	if !strings.Contains(output, "  - alice (you): 4 contributions, last updated 2024-04-05 12:30\n") {
		t.Fatalf("runStatus() output missing exact valid account row:\n%s", output)
	}
}

func TestRunStatusJSONPrintsExactTotals(t *testing.T) {
	repo := setupStatusTest(t)
	setStatusJSON(t)
	writeStatusFile(t, repo, ".vanity/alice.json", `{
  "username": "alice",
  "last_updated": "2024-04-05T12:30:00Z",
  "contributions": [{"date": "2024-04-04", "count": 7}, {"date": "2024-04-05", "count": 5}]
}`)
	writeStatusFile(t, repo, ".vanity/bob.json", `{
  "username": "bob",
  "last_updated": "2024-03-01T08:00:00Z",
  "contributions": [{"date": "2024-03-01", "count": 3}]
}`)
	writeStatusFile(t, repo, ".vanity/alice-state.json", `{
  "username": "alice",
  "last_sync": "2024-04-05T12:30:00Z",
  "mirrored_counts": {"bob": {"2024-03-01": 2, "2024-02-01": 1}}
}`)

	output, err := captureStatusOutput(t, func() error {
		return runStatus(statusCmd, nil)
	})
	if err != nil {
		t.Fatalf("runStatus() error = %v", err)
	}
	var got statusReport
	if err := json.Unmarshal([]byte(output), &got); err != nil {
		t.Fatalf("stdout is not a single JSON document: %v\n%s", err, output)
	}
	if got.CurrentUser != "alice" || len(got.Accounts) != 2 || len(got.Mirrored) != 1 {
		t.Fatalf("unexpected report: %+v", got)
	}
	if a := got.Accounts[0]; a.Username != "alice" || a.Total != 12 || a.LastUpdated.Format("2006-01-02T15:04") != "2024-04-05T12:30" {
		t.Fatalf("accounts[0] = %+v", a)
	}
	if a := got.Accounts[1]; a.Username != "bob" || a.Total != 3 {
		t.Fatalf("accounts[1] = %+v", a)
	}
	if m := got.Mirrored[0]; m.Source != "bob" || m.Dates != 2 || m.Total != 3 {
		t.Fatalf("mirrored[0] = %+v", m)
	}
}

func TestRunStatusJSONKeepsValidRowsOnMalformedInput(t *testing.T) {
	repo := setupStatusTest(t)
	setStatusJSON(t)
	writeStatusFile(t, repo, ".vanity/alice.json", `{
  "username": "alice",
  "last_updated": "2024-04-05T12:30:00Z",
  "contributions": [{"date": "2024-04-05", "count": 4}]
}`)
	writeStatusFile(t, repo, ".vanity/broken.json", `{"contributions":`)

	output, err := captureStatusOutput(t, func() error {
		return runStatus(statusCmd, nil)
	})
	if err == nil || !strings.Contains(err.Error(), ".vanity/broken.json") {
		t.Fatalf("runStatus() error = %v, want broken.json error", err)
	}
	var got statusReport
	if err := json.Unmarshal([]byte(output), &got); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, output)
	}
	if len(got.Accounts) != 1 || got.Accounts[0].Username != "alice" || got.Accounts[0].Total != 4 {
		t.Fatalf("accounts = %+v, want only alice with 4", got.Accounts)
	}
}

func TestRunStatusJSONEmptyRepoPrintsEmptyArrays(t *testing.T) {
	setupStatusTest(t)
	setStatusJSON(t)

	output, err := captureStatusOutput(t, func() error {
		return runStatus(statusCmd, nil)
	})
	if err != nil {
		t.Fatalf("runStatus() error = %v", err)
	}
	if !strings.Contains(output, `"accounts": []`) || !strings.Contains(output, `"mirrored": []`) {
		t.Fatalf("want empty arrays, got:\n%s", output)
	}
}

func setStatusJSON(t *testing.T) {
	t.Helper()
	statusJSON = true
	t.Cleanup(func() { statusJSON = false })
}

func setupStatusTest(t *testing.T) string {
	t.Helper()

	repo := t.TempDir()
	if err := os.Mkdir(filepath.Join(repo, ".vanity"), 0755); err != nil {
		t.Fatalf("create .vanity directory: %v", err)
	}

	binDir := t.TempDir()
	ghPath := filepath.Join(binDir, "gh")
	if err := os.WriteFile(ghPath, []byte("#!/bin/sh\nprintf 'alice\\n'\n"), 0755); err != nil {
		t.Fatalf("write gh stub: %v", err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	if err := os.Chdir(repo); err != nil {
		t.Fatalf("change working directory: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(oldDir); err != nil {
			t.Errorf("restore working directory: %v", err)
		}
	})

	return repo
}

func writeStatusFile(t *testing.T, repo, name, contents string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(repo, name), []byte(contents), 0644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

func captureStatusOutput(t *testing.T, fn func() error) (string, error) {
	t.Helper()

	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stdout pipe: %v", err)
	}
	oldStdout := os.Stdout
	os.Stdout = writer
	t.Cleanup(func() { os.Stdout = oldStdout })

	runErr := fn()
	if err := writer.Close(); err != nil {
		t.Fatalf("close stdout writer: %v", err)
	}
	os.Stdout = oldStdout
	output, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	if err := reader.Close(); err != nil {
		t.Fatalf("close stdout reader: %v", err)
	}
	return string(output), runErr
}
