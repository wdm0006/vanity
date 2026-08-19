package sync

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
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

func TestMirrorUserCreatesOnlyBackdatedDeltas(t *testing.T) {
	repo := initTestRepo(t, "main")
	writeTestFile(t, repo, ".vanity/bob.json",
		`{"username":"bob","contributions":[{"date":"2024-01-02","count":2},{"date":"2024-03-04","count":1}]}`)

	state := &SyncState{Username: "alice"}
	engine := &Engine{username: "alice"}
	batchCount := 0

	withWorkingDirectory(t, repo, func() {
		mirrored, err := engine.mirrorUser("bob", state, false, &batchCount)
		if err != nil {
			t.Fatalf("first mirrorUser() error = %v", err)
		}
		if mirrored != 3 {
			t.Fatalf("first mirrorUser() mirrored %d commits, want 3", mirrored)
		}
		if got := runGit(t, repo, "rev-list", "--count", "HEAD"); got != "3" {
			t.Fatalf("commit count after first mirror = %s, want 3", got)
		}

		dates := strings.Fields(runGit(t, repo, "log", "--format=%ad", "--date=short"))
		wantDates := []string{"2024-03-04", "2024-01-02", "2024-01-02"}
		if strings.Join(dates, ",") != strings.Join(wantDates, ",") {
			t.Fatalf("author dates = %v, want %v", dates, wantDates)
		}

		mirrored, err = engine.mirrorUser("bob", state, false, &batchCount)
		if err != nil {
			t.Fatalf("second mirrorUser() error = %v", err)
		}
		if mirrored != 0 {
			t.Fatalf("second mirrorUser() mirrored %d commits, want 0", mirrored)
		}
		if got := runGit(t, repo, "rev-list", "--count", "HEAD"); got != "3" {
			t.Fatalf("commit count after second mirror = %s, want 3", got)
		}

		writeTestFile(t, repo, ".vanity/bob.json",
			`{"username":"bob","contributions":[{"date":"2024-01-02","count":4},{"date":"2024-03-04","count":1}]}`)
		mirrored, err = engine.mirrorUser("bob", state, false, &batchCount)
		if err != nil {
			t.Fatalf("incremental mirrorUser() error = %v", err)
		}
		if mirrored != 2 {
			t.Fatalf("incremental mirrorUser() mirrored %d commits, want 2", mirrored)
		}
		if got := runGit(t, repo, "rev-list", "--count", "HEAD"); got != "5" {
			t.Fatalf("commit count after incremental mirror = %s, want 5", got)
		}
	})

	if got := state.GetMirroredCount("bob", "2024-01-02"); got != 4 {
		t.Errorf("mirrored count for 2024-01-02 = %d, want 4", got)
	}
	if got := state.GetMirroredCount("bob", "2024-03-04"); got != 1 {
		t.Errorf("mirrored count for 2024-03-04 = %d, want 1", got)
	}
	if batchCount != 5 {
		t.Errorf("batch count = %d, want 5", batchCount)
	}
}

func TestMirrorAllUsersSucceedsWhenEverySourceMirrors(t *testing.T) {
	repo := initTestRepo(t, "main")
	writeTestFile(t, repo, ".vanity/alice.json",
		`{"username":"alice","contributions":[{"date":"2024-05-05","count":1}]}`)
	writeTestFile(t, repo, ".vanity/bob.json",
		`{"username":"bob","contributions":[{"date":"2024-01-02","count":2}]}`)
	writeTestFile(t, repo, ".vanity/carol.json",
		`{"username":"carol","contributions":[{"date":"2024-02-03","count":1}]}`)

	state := &SyncState{Username: "alice"}
	batchCount := 0
	var mirrored int
	var err error
	withWorkingDirectory(t, repo, func() {
		mirrored, err = (&Engine{username: "alice"}).mirrorAllUsers(
			[]string{"alice", "bob", "carol"}, state, false, &batchCount)
	})

	if err != nil {
		t.Fatalf("mirrorAllUsers() error = %v, want nil", err)
	}
	if mirrored != 3 {
		t.Errorf("mirrored %d commits, want 3", mirrored)
	}
	// The current user is never mirrored onto themselves.
	dates := strings.Fields(runGit(t, repo, "log", "--format=%ad", "--date=short"))
	wantDates := []string{"2024-02-03", "2024-01-02", "2024-01-02"}
	if strings.Join(dates, ",") != strings.Join(wantDates, ",") {
		t.Errorf("author dates = %v, want %v", dates, wantDates)
	}
}

func TestMirrorAllUsersReportsFailedSourcesAndKeepsGoing(t *testing.T) {
	repo := initTestRepo(t, "main")
	writeTestFile(t, repo, ".vanity/bob.json",
		`{"username":"bob","contributions":[{"date":"2024-01-02","count":2}]}`)
	writeTestFile(t, repo, ".vanity/carol.json",
		`{"username":"carol","contributions":[{"date":"2024-02-03","count":1}]}`)
	// Reject bob's first mirror commit; every later commit (carol's) succeeds.
	failNthCommit(t, repo, 1)

	state := &SyncState{Username: "alice"}
	batchCount := 0
	var mirrored int
	var err error
	withWorkingDirectory(t, repo, func() {
		mirrored, err = (&Engine{username: "alice"}).mirrorAllUsers(
			[]string{"bob", "carol"}, state, false, &batchCount)
	})

	if err == nil {
		t.Fatal("mirrorAllUsers() error = nil, want an error naming the failed source")
	}
	if !strings.Contains(err.Error(), "bob") {
		t.Errorf("error %q does not name the failed source bob", err)
	}
	if strings.Contains(err.Error(), "carol") {
		t.Errorf("error %q names carol, which mirrored successfully", err)
	}
	if want := "failed to mirror 1 of 2 source accounts"; !strings.Contains(err.Error(), want) {
		t.Errorf("error %q does not summarize the failure as %q", err, want)
	}

	// The failure must not abandon the remaining sources.
	if mirrored != 1 {
		t.Errorf("mirrored %d commits, want 1 (carol's)", mirrored)
	}
	dates := strings.Fields(runGit(t, repo, "log", "--format=%ad", "--date=short"))
	if strings.Join(dates, ",") != "2024-02-03" {
		t.Errorf("author dates = %v, want only carol's 2024-02-03", dates)
	}
	if got := state.GetMirroredCount("carol", "2024-02-03"); got != 1 {
		t.Errorf("mirrored count for carol = %d, want 1", got)
	}
	if got := state.GetMirroredCount("bob", "2024-01-02"); got != 0 {
		t.Errorf("mirrored count for bob = %d, want 0", got)
	}
}

// failNthCommit installs a pre-commit hook that rejects the nth commit attempt in the
// repository and allows every other one, so a single mirror commit fails deterministically.
func failNthCommit(t *testing.T, repo string, n int) {
	t.Helper()
	hooksDir := filepath.Join(t.TempDir(), "hooks")
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		t.Fatalf("create hooks directory: %v", err)
	}
	counter := filepath.Join(hooksDir, "attempts")
	script := fmt.Sprintf(`#!/bin/sh
count=$(cat %q 2>/dev/null || echo 0)
count=$((count + 1))
echo "$count" > %q
if [ "$count" -eq %d ]; then
	exit 1
fi
exit 0
`, counter, counter, n)
	if err := os.WriteFile(filepath.Join(hooksDir, "pre-commit"), []byte(script), 0755); err != nil {
		t.Fatalf("write pre-commit hook: %v", err)
	}
	runGit(t, repo, "config", "core.hooksPath", hooksDir)
}

func TestMirrorUserCheckpointsPartialDailyBatch(t *testing.T) {
	repo := initTestRepo(t, "main")
	writeTestFile(t, repo, ".vanity/bob.json",
		`{"username":"bob","contributions":[{"date":"2024-01-02","count":3},{"date":"2024-03-04","count":1}]}`)
	failOnCommit(t, repo, 2)

	state := &SyncState{Username: "alice"}
	engine := &Engine{username: "alice"}
	batchCount := 0

	withWorkingDirectory(t, repo, func() {
		mirrored, err := engine.mirrorUser("bob", state, false, &batchCount)
		if err == nil {
			t.Fatalf("mirrorUser() error = nil, want the second commit to fail")
		}
		if mirrored != 1 {
			t.Fatalf("mirrorUser() mirrored %d commits, want 1", mirrored)
		}
		if got := commitCount(t, repo); got != 1 {
			t.Fatalf("commit count after partial batch = %d, want 1", got)
		}
		if got := state.GetMirroredCount("bob", "2024-01-02"); got != 1 {
			t.Fatalf("mirrored count for 2024-01-02 = %d, want 1", got)
		}
		if got := state.GetMirroredCount("bob", "2024-03-04"); got != 0 {
			t.Fatalf("mirrored count for 2024-03-04 = %d, want 0", got)
		}
		if batchCount != 1 {
			t.Fatalf("batch count after partial batch = %d, want 1", batchCount)
		}

		allowAllCommits(t, repo)

		mirrored, err = engine.mirrorUser("bob", state, false, &batchCount)
		if err != nil {
			t.Fatalf("retry mirrorUser() error = %v", err)
		}
		if mirrored != 3 {
			t.Fatalf("retry mirrorUser() mirrored %d commits, want 3", mirrored)
		}
	})

	if got := commitCount(t, repo); got != 4 {
		t.Fatalf("commit count after retry = %d, want 4", got)
	}
	dates := strings.Fields(runGit(t, repo, "log", "--format=%ad", "--date=short"))
	byDate := map[string]int{}
	for _, date := range dates {
		byDate[date]++
	}
	if byDate["2024-01-02"] != 3 || byDate["2024-03-04"] != 1 {
		t.Fatalf("commits per date = %v, want 3 on 2024-01-02 and 1 on 2024-03-04", byDate)
	}
	for _, message := range strings.Split(runGit(t, repo, "log", "--format=%s"), "\n") {
		if !strings.HasPrefix(message, "vanity: mirror from bob ") {
			t.Fatalf("unexpected commit message %q", message)
		}
	}
	if got := state.GetMirroredCount("bob", "2024-01-02"); got != 3 {
		t.Errorf("mirrored count for 2024-01-02 = %d, want 3", got)
	}
	if got := state.GetMirroredCount("bob", "2024-03-04"); got != 1 {
		t.Errorf("mirrored count for 2024-03-04 = %d, want 1", got)
	}
	if batchCount != 4 {
		t.Errorf("batch count = %d, want 4", batchCount)
	}
}

func TestMirrorUserKeepsCountWhenFirstCommitFails(t *testing.T) {
	repo := initTestRepo(t, "main")
	writeTestFile(t, repo, ".vanity/bob.json",
		`{"username":"bob","contributions":[{"date":"2024-01-02","count":3}]}`)
	failOnCommit(t, repo, 1)

	state := &SyncState{Username: "alice"}
	state.SetMirroredCount("bob", "2024-01-02", 1)
	engine := &Engine{username: "alice"}
	batchCount := 0

	withWorkingDirectory(t, repo, func() {
		mirrored, err := engine.mirrorUser("bob", state, false, &batchCount)
		if err == nil {
			t.Fatalf("mirrorUser() error = nil, want the first commit to fail")
		}
		if mirrored != 0 {
			t.Fatalf("mirrorUser() mirrored %d commits, want 0", mirrored)
		}
	})

	if got := commitCount(t, repo); got != 0 {
		t.Errorf("commit count = %d, want 0", got)
	}
	if got := state.GetMirroredCount("bob", "2024-01-02"); got != 1 {
		t.Errorf("mirrored count for 2024-01-02 = %d, want it left at 1", got)
	}
	if batchCount != 0 {
		t.Errorf("batch count = %d, want 0", batchCount)
	}
}

// failOnCommit installs a pre-commit hook that rejects the nth commit attempted in repo.
func failOnCommit(t *testing.T, repo string, n int) {
	t.Helper()
	hooks := filepath.Join(repo, "testhooks")
	if err := os.MkdirAll(hooks, 0755); err != nil {
		t.Fatalf("create hooks directory: %v", err)
	}
	script := fmt.Sprintf(`#!/bin/sh
counter="$(dirname "$0")/attempts"
attempts=$(cat "$counter" 2>/dev/null || echo 0)
attempts=$((attempts + 1))
echo "$attempts" > "$counter"
[ "$attempts" -eq %d ] && exit 1
exit 0
`, n)
	if err := os.WriteFile(filepath.Join(hooks, "pre-commit"), []byte(script), 0755); err != nil {
		t.Fatalf("write pre-commit hook: %v", err)
	}
	runGit(t, repo, "config", "core.hooksPath", hooks)
}

// allowAllCommits removes any hook installed by failOnCommit.
func allowAllCommits(t *testing.T, repo string) {
	t.Helper()
	if err := os.Remove(filepath.Join(repo, "testhooks", "pre-commit")); err != nil {
		t.Fatalf("remove pre-commit hook: %v", err)
	}
}

func commitCount(t *testing.T, repo string) int {
	t.Helper()
	count, err := strconv.Atoi(runGit(t, repo, "rev-list", "--count", "--all"))
	if err != nil {
		t.Fatalf("parse commit count: %v", err)
	}
	return count
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

func TestRebuildHistoryRefusesTrackedFilesOutsideVanity(t *testing.T) {
	repo := initTestRepo(t, "feature")
	writeTestFile(t, repo, ".vanity/alice.json", `{"username":"alice"}`)
	writeTestFile(t, repo, "README.md", "docs")
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-m", "initial")
	originalHead := runGit(t, repo, "rev-parse", "HEAD")
	originalTree := runGit(t, repo, "ls-tree", "-r", "--name-only", "HEAD")

	state := &SyncState{MirroredCounts: map[string]map[string]int{"bob": {"2024-01-01": 1}}}
	withWorkingDirectory(t, repo, func() {
		err := (&Engine{}).rebuildHistory(state)
		if err == nil {
			t.Fatal("rebuildHistory() error = nil, want refusal")
		}
		if !strings.Contains(err.Error(), "README.md") {
			t.Fatalf("rebuildHistory() error = %v, want it to name README.md", err)
		}
		if !strings.Contains(err.Error(), "dedicated sync repository") {
			t.Fatalf("rebuildHistory() error = %v, want it to require a dedicated sync repository", err)
		}
	})

	if got := state.GetMirroredCount("bob", "2024-01-01"); got != 1 {
		t.Fatalf("mirrored count = %d, want the refused rebuild to leave state alone", got)
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
		t.Fatalf("index or working tree was modified: %q", status)
	}
	if tree := runGit(t, repo, "ls-tree", "-r", "--name-only", "HEAD"); tree != originalTree {
		t.Fatalf("tracked tree = %q, want %q", tree, originalTree)
	}
	if contents, err := os.ReadFile(filepath.Join(repo, "README.md")); err != nil || string(contents) != "docs" {
		t.Fatalf("README.md = %q, %v; want it left on disk untouched", contents, err)
	}
}

func TestRebuildHistoryClassifiesVanityPrefixedPathsStructurally(t *testing.T) {
	repo := initTestRepo(t, "feature")
	writeTestFile(t, repo, ".vanity/alice.json", `{"username":"alice"}`)
	writeTestFile(t, repo, ".vanity-backup/alice.json", `{"username":"alice"}`)
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-m", "initial")

	withWorkingDirectory(t, repo, func() {
		err := (&Engine{}).rebuildHistory(&SyncState{})
		if err == nil || !strings.Contains(err.Error(), ".vanity-backup/alice.json") {
			t.Fatalf("rebuildHistory() error = %v, want .vanity-backup/alice.json treated as outside .vanity/", err)
		}
	})

	if branches := runGit(t, repo, "branch", "--list", "temp-rebuild"); branches != "" {
		t.Fatalf("temporary rebuild branch was created: %q", branches)
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

func TestSyncAbortsBeforeMutationWhenPullFails(t *testing.T) {
	repo := initSyncRepo(t, true)
	ghMarker := stubGitHubCLI(t)
	before := snapshotRepo(t, repo)

	var err error
	withWorkingDirectory(t, repo, func() {
		silenceStderr(t)
		captureStdout(t, func() {
			err = (&Engine{username: "alice", batchSize: 100}).Sync(false)
		})
	})

	if err == nil {
		t.Fatal("Sync() error = nil, want the git pull failure")
	}
	if !strings.Contains(err.Error(), "git pull failed") {
		t.Fatalf("Sync() error = %v, want it to identify the pull failure", err)
	}
	if _, statErr := os.Stat(ghMarker); statErr == nil {
		t.Error("Sync() fetched from GitHub after the pull failed")
	}
	if after := snapshotRepo(t, repo); !reflect.DeepEqual(after, before) {
		t.Errorf("repository mutated after the failed pull:\nbefore = %+v\nafter  = %+v", before, after)
	}
}

func TestSyncModesThatSkipThePull(t *testing.T) {
	tests := []struct {
		name         string
		brokenRemote bool
		rebuild      bool
		dryRun       bool
	}{
		{name: "dry run", brokenRemote: true, dryRun: true},
		{name: "rebuild", brokenRemote: true, rebuild: true},
		{name: "no remote", brokenRemote: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := initSyncRepo(t, tt.brokenRemote)
			ghMarker := stubGitHubCLI(t)

			var err error
			withWorkingDirectory(t, repo, func() {
				silenceStderr(t)
				captureStdout(t, func() {
					err = (&Engine{username: "alice", batchSize: 100, rebuild: tt.rebuild}).Sync(tt.dryRun)
				})
			})

			// The remote is unusable, so reaching the GitHub fetch at all is what proves
			// step 1 was skipped rather than attempted.
			if _, statErr := os.Stat(ghMarker); statErr != nil {
				t.Fatalf("Sync() never reached the GitHub fetch: %v (error was %v)", statErr, err)
			}
			if err == nil || !strings.Contains(err.Error(), "failed to fetch contributions") {
				t.Fatalf("Sync() error = %v, want the stubbed GitHub fetch failure", err)
			}
		})
	}
}

// initSyncRepo builds a repository with committed .vanity/ data for alice and bob.
// With brokenRemote it also configures an origin that does not exist, so
// `git pull --rebase` fails locally without touching the network.
func initSyncRepo(t *testing.T, brokenRemote bool) string {
	t.Helper()
	repo := initTestRepo(t, "main")
	writeTestFile(t, repo, ".vanity/alice.json", `{"username":"alice","contributions":[{"date":"2024-01-01","count":3}]}`)
	writeTestFile(t, repo, ".vanity/bob.json", `{"username":"bob","contributions":[{"date":"2024-01-02","count":2}]}`)
	writeTestFile(t, repo, ".vanity/alice-state.json",
		`{"username":"alice","last_sync":"2024-01-05T00:00:00Z","mirrored_counts":{"bob":{"2024-01-02":2}}}`)
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-m", "initial")
	if brokenRemote {
		runGit(t, repo, "remote", "add", "origin", filepath.Join(repo, "missing-remote.git"))
	}
	return repo
}

// stubGitHubCLI puts a failing gh at the front of PATH and returns the path of the
// marker file it writes when invoked, so a test can tell whether Sync reached the
// GitHub fetch.
func stubGitHubCLI(t *testing.T) string {
	t.Helper()
	binDir := t.TempDir()
	marker := filepath.Join(binDir, "invoked")
	script := fmt.Sprintf(`#!/bin/sh
echo "$@" >> %q
echo "stub gh: no network in tests" >&2
exit 1
`, marker)
	if err := os.WriteFile(filepath.Join(binDir, "gh"), []byte(script), 0755); err != nil {
		t.Fatalf("write gh stub: %v", err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return marker
}

// repoSnapshot captures the repository state a failed sync must leave untouched.
type repoSnapshot struct {
	Head    string
	Commits string
	Vanity  map[string]string
}

func snapshotRepo(t *testing.T, repo string) repoSnapshot {
	t.Helper()
	snapshot := repoSnapshot{
		Head:    runGit(t, repo, "rev-parse", "HEAD"),
		Commits: runGit(t, repo, "rev-list", "--count", "--all"),
		Vanity:  make(map[string]string),
	}
	entries, err := os.ReadDir(filepath.Join(repo, vanityDir))
	if err != nil {
		t.Fatalf("read %s: %v", vanityDir, err)
	}
	for _, entry := range entries {
		contents, err := os.ReadFile(filepath.Join(repo, vanityDir, entry.Name()))
		if err != nil {
			t.Fatalf("read %s: %v", entry.Name(), err)
		}
		snapshot.Vanity[entry.Name()] = string(contents)
	}
	return snapshot
}

// silenceStderr keeps the deliberate git and gh failures out of the test log.
func silenceStderr(t *testing.T) {
	t.Helper()
	devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		t.Fatalf("open %s: %v", os.DevNull, err)
	}
	original := os.Stderr
	os.Stderr = devNull
	t.Cleanup(func() {
		os.Stderr = original
		if err := devNull.Close(); err != nil {
			t.Errorf("close %s: %v", os.DevNull, err)
		}
	})
}
