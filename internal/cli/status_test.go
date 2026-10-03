package cli

import (
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
