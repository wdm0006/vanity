package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func inInitRepo(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".git"), 0755); err != nil {
		t.Fatal(err)
	}
	wd, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd); initWithWorkflow = false })
}

func TestInitWithoutFlagWritesNoWorkflow(t *testing.T) {
	inInitRepo(t)
	if err := runInit(initCmd, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(syncWorkflowPath); !os.IsNotExist(err) {
		t.Fatalf("workflow written without flag: %v", err)
	}
	if _, err := os.Stat(".vanity/.gitkeep"); err != nil {
		t.Fatal(err)
	}
}

func TestInitWithWorkflowContent(t *testing.T) {
	inInitRepo(t)
	initWithWorkflow = true
	if err := runInit(initCmd, nil); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(syncWorkflowPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(b, syncWorkflowTemplate) {
		t.Fatal("written file differs from embedded template")
	}
	s := string(b)
	for _, want := range []string{"schedule:", "cron:", "workflow_dispatch:", "permissions:\n  contents: write", "concurrency:", "secrets.VANITY_TOKEN"} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q", want)
		}
	}
	if d, sy := strings.Index(s, "run: vanity doctor"), strings.Index(s, "run: vanity sync"); d < 0 || sy < 0 || d > sy {
		t.Errorf("doctor must precede sync (doctor=%d sync=%d)", d, sy)
	}
}

func TestInitWithWorkflowDoesNotOverwrite(t *testing.T) {
	inInitRepo(t)
	if err := os.MkdirAll(filepath.Dir(syncWorkflowPath), 0755); err != nil {
		t.Fatal(err)
	}
	custom := []byte("name: mine\n")
	if err := os.WriteFile(syncWorkflowPath, custom, 0644); err != nil {
		t.Fatal(err)
	}
	initWithWorkflow = true
	if err := runInit(initCmd, nil); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(syncWorkflowPath)
	if !bytes.Equal(got, custom) {
		t.Fatalf("existing file modified: %q", got)
	}
}
