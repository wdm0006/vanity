package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestListTrackedFilesReturnsRepositoryRelativePaths(t *testing.T) {
	repo := initTestRepo(t)
	writeFile(t, repo, ".vanity/alice.json", `{"username":"alice"}`)
	writeFile(t, repo, "docs/a file with spaces.md", "docs")
	writeFile(t, repo, "untracked.txt", "ignored")
	runGit(t, repo, "add", ".vanity", "docs")

	var paths []string
	withWorkingDirectory(t, repo, func() {
		var err error
		paths, err = ListTrackedFiles()
		if err != nil {
			t.Fatalf("ListTrackedFiles() error = %v", err)
		}
	})

	want := []string{".vanity/alice.json", "docs/a file with spaces.md"}
	if !reflect.DeepEqual(paths, want) {
		t.Fatalf("ListTrackedFiles() = %v, want %v", paths, want)
	}
}

func TestListTrackedFilesIsEmptyForAnEmptyIndex(t *testing.T) {
	repo := initTestRepo(t)

	withWorkingDirectory(t, repo, func() {
		paths, err := ListTrackedFiles()
		if err != nil {
			t.Fatalf("ListTrackedFiles() error = %v", err)
		}
		if len(paths) != 0 {
			t.Fatalf("ListTrackedFiles() = %v, want no paths", paths)
		}
	})
}

func initTestRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	runGit(t, repo, "init", "-b", "main")
	runGit(t, repo, "config", "user.name", "Vanity Test")
	runGit(t, repo, "config", "user.email", "vanity@example.com")
	return repo
}

func writeFile(t *testing.T, repo, name, contents string) {
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
