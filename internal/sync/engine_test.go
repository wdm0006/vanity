package sync

import (
	"io"
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

func TestPrepareRebuildDryRunClearsCountsWithoutTouchingRepo(t *testing.T) {
	repo := initTestRepo(t, "feature")
	writeTestFile(t, repo, ".vanity/alice.json", `{"username":"alice"}`)
	writeTestFile(t, repo, "app.txt", "unrelated")
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-m", "initial")
	originalHead := runGit(t, repo, "rev-parse", "HEAD")

	state := &SyncState{Username: "alice", MirroredCounts: map[string]map[string]int{"bob": {"2024-01-01": 1}}}
	withWorkingDirectory(t, repo, func() {
		if err := (&Engine{username: "alice", rebuild: true}).prepareRebuild(state, true); err != nil {
			t.Fatalf("prepareRebuild() error = %v", err)
		}
	})

	if len(state.MirroredCounts) != 0 {
		t.Fatalf("mirrored counts were not cleared: %#v", state.MirroredCounts)
	}
	if head := runGit(t, repo, "rev-parse", "HEAD"); head != originalHead {
		t.Fatalf("HEAD changed from %s to %s", originalHead, head)
	}
	if branch := runGit(t, repo, "branch", "--show-current"); branch != "feature" {
		t.Fatalf("current branch = %q, want feature", branch)
	}
	if branches := runGit(t, repo, "branch", "--list", "temp-rebuild"); branches != "" {
		t.Fatalf("temporary rebuild branch was created: %q", branches)
	}
	if status := runGit(t, repo, "status", "--porcelain"); status != "" {
		t.Fatalf("working tree was modified: %q", status)
	}
}

func TestPrepareRebuildWithoutRebuildKeepsCounts(t *testing.T) {
	state := &SyncState{Username: "alice", MirroredCounts: map[string]map[string]int{"bob": {"2024-01-01": 1}}}

	if err := (&Engine{username: "alice"}).prepareRebuild(state, true); err != nil {
		t.Fatalf("prepareRebuild() error = %v", err)
	}

	if got := state.GetMirroredCount("bob", "2024-01-01"); got != 1 {
		t.Fatalf("mirrored count = %d, want 1", got)
	}
}

func TestDryRunRebuildPreviewsEveryStoredContribution(t *testing.T) {
	repo := initTestRepo(t, "feature")
	writeTestFile(t, repo, ".vanity/bob.json",
		`{"username":"bob","contributions":[{"date":"2024-01-01","count":3},{"date":"2024-01-02","count":5}]}`)

	newState := func() *SyncState {
		return &SyncState{
			Username:       "alice",
			MirroredCounts: map[string]map[string]int{"bob": {"2024-01-01": 3, "2024-01-02": 4}},
		}
	}

	var plainCount, rebuildCount int
	var plainOutput, rebuildOutput string
	withWorkingDirectory(t, repo, func() {
		plainCount, plainOutput = previewMirror(t, &Engine{username: "alice"}, newState())
		rebuildCount, rebuildOutput = previewMirror(t, &Engine{username: "alice", rebuild: true}, newState())
	})

	// Without --rebuild the preview stays incremental: only the extra commit on 2024-01-02.
	if plainCount != 1 {
		t.Errorf("plain dry run previewed %d commits, want 1\n%s", plainCount, plainOutput)
	}
	if !strings.Contains(plainOutput, "Would create 1 commits for 2024-01-02 from bob") {
		t.Errorf("plain dry run output missing incremental preview:\n%s", plainOutput)
	}
	if strings.Contains(plainOutput, "2024-01-01") {
		t.Errorf("plain dry run previewed an already mirrored date:\n%s", plainOutput)
	}

	// With --rebuild the preview covers the whole stored history, as the real rebuild would.
	if rebuildCount != 8 {
		t.Errorf("rebuild dry run previewed %d commits, want 8\n%s", rebuildCount, rebuildOutput)
	}
	for _, want := range []string{
		"Would create 3 commits for 2024-01-01 from bob",
		"Would create 5 commits for 2024-01-02 from bob",
	} {
		if !strings.Contains(rebuildOutput, want) {
			t.Errorf("rebuild dry run output missing %q:\n%s", want, rebuildOutput)
		}
	}

	if _, err := os.Stat(filepath.Join(repo, ".vanity", "alice-state.json")); !os.IsNotExist(err) {
		t.Errorf("dry run wrote sync state: err = %v", err)
	}
}

// previewMirror runs the dry-run rebuild preparation and mirror preview the way Sync does,
// returning the number of commits previewed and everything printed while doing so.
func previewMirror(t *testing.T, e *Engine, state *SyncState) (int, string) {
	t.Helper()
	var mirrored int
	output := captureStdout(t, func() {
		if err := e.prepareRebuild(state, true); err != nil {
			t.Errorf("prepareRebuild() error = %v", err)
			return
		}
		batchCount := 0
		count, err := e.mirrorUser("bob", state, true, &batchCount)
		if err != nil {
			t.Errorf("mirrorUser() error = %v", err)
			return
		}
		mirrored = count
	})
	return mirrored, output
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("create pipe: %v", err)
	}
	original := os.Stdout
	os.Stdout = writer
	defer func() { os.Stdout = original }()

	fn()

	if err := writer.Close(); err != nil {
		t.Fatalf("close pipe: %v", err)
	}
	output, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read captured output: %v", err)
	}
	return string(output)
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
