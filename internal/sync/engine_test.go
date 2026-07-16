package sync

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

	newContributions := []github.Contribution{
		{Date: "2024-04-01", Count: 4},
		{Date: "2024-01-05", Count: 9},
		{Date: "2024-02-01", Count: 2},
	}

	merged := e.mergeContributions(existing, newContributions)
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
	for i, expected := range want {
		if got := merged.Contributions[i]; got != expected {
			t.Errorf("contribution %d: got %+v, want %+v", i, got, expected)
		}
	}
	if merged.Username != "alice" {
		t.Errorf("got username %q, want %q", merged.Username, "alice")
	}
}

func TestRebuildHistoryPreservesOnlyVanityFilesOnCurrentBranch(t *testing.T) {
	repo := initTestRepo(t, "feature")
	writeTestFile(t, repo, ".vanity/alice.json", `{"username":"alice"}`)
	writeTestFile(t, repo, "app.txt", "unrelated")
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-m", "initial")

	withWorkingDirectory(t, repo, func() {
		state := &SyncState{MirroredCounts: map[string]map[string]int{"bob": {"2024-01-01": 1}}}
		if err := (&Engine{}).rebuildHistory(state); err != nil {
			t.Fatalf("rebuildHistory() error = %v", err)
		}
		if len(state.MirroredCounts) != 0 {
			t.Fatalf("mirrored counts were not cleared: %#v", state.MirroredCounts)
		}
	})

	if branch := runGit(t, repo, "branch", "--show-current"); branch != "feature" {
		t.Fatalf("current branch = %q, want feature", branch)
	}
	if message := runGit(t, repo, "log", "-1", "--format=%s"); message != "vanity: rebuild init" {
		t.Fatalf("commit message = %q, want vanity: rebuild init", message)
	}
	if files := strings.Fields(runGit(t, repo, "ls-tree", "-r", "--name-only", "HEAD")); len(files) != 1 || files[0] != ".vanity/alice.json" {
		t.Fatalf("rebuilt tree files = %v, want only .vanity/alice.json", files)
	}
}

func TestRebuildHistoryRejectsDetachedHead(t *testing.T) {
	repo := initTestRepo(t, "feature")
	writeTestFile(t, repo, ".vanity/alice.json", `{"username":"alice"}`)
	writeTestFile(t, repo, "app.txt", "unrelated")
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-m", "initial")
	originalHead := runGit(t, repo, "rev-parse", "HEAD")
	runGit(t, repo, "checkout", "--detach")

	withWorkingDirectory(t, repo, func() {
		err := (&Engine{}).rebuildHistory(&SyncState{})
		if err == nil || !strings.Contains(err.Error(), "detached HEAD") {
			t.Fatalf("rebuildHistory() error = %v, want detached HEAD error", err)
		}
	})

	if head := runGit(t, repo, "rev-parse", "HEAD"); head != originalHead {
		t.Fatalf("HEAD changed from %s to %s", originalHead, head)
	}
	if branches := runGit(t, repo, "branch", "--list", "temp-rebuild"); branches != "" {
		t.Fatalf("temporary rebuild branch was created: %q", branches)
	}
}

func initTestRepo(t *testing.T, branch string) string {
	t.Helper()
	repo := t.TempDir()
	runGit(t, repo, "init", "-b", branch)
	runGit(t, repo, "config", "user.name", "Vanity Test")
	runGit(t, repo, "config", "user.email", "vanity@example.com")
	return repo
}

func writeTestFile(t *testing.T, repo, name, contents string) {
	t.Helper()
	path := filepath.Join(repo, name)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("create parent directory: %v", err)
	}
	if err := os.WriteFile(path, []byte(contents), 0644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

func runGit(t *testing.T, repo string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = repo
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, output)
	}
	return strings.TrimSpace(string(output))
}

func withWorkingDirectory(t *testing.T, dir string, fn func()) {
	t.Helper()
	original, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("change working directory: %v", err)
	}
	defer func() {
		if err := os.Chdir(original); err != nil {
			t.Fatalf("restore working directory: %v", err)
		}
	}()
	fn()
}
