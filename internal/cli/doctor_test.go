package cli

import (
	"bytes"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// setupDoctorRepo creates an initialized git repo, isolates git config from
// the host, and puts a gh stub on PATH. emailsBody is printed for
// `gh api user/emails`; when emailsFail is true that call exits 1 instead.
func setupDoctorRepo(t *testing.T, email, emailsBody string, emailsFail bool) string {
	t.Helper()

	repo := t.TempDir()
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(t.TempDir(), "none"))
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	for _, args := range [][]string{{"init", "-q"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	if email != "" {
		cmd := exec.Command("git", "config", "user.email", email)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git config: %v\n%s", err, out)
		}
	}
	if err := os.Mkdir(filepath.Join(repo, ".vanity"), 0755); err != nil {
		t.Fatal(err)
	}

	binDir := t.TempDir()
	emailsCmd := "cat <<'JSON'\n" + emailsBody + "\nJSON\n"
	if emailsFail {
		emailsCmd = "echo 'HTTP 404: Not Found (missing user:email scope)' >&2\nexit 1\n"
	}
	script := "#!/bin/sh\ncase \"$2\" in\n  user) printf 'alice\\n';;\n  user/emails)\n" + emailsCmd + ";;\n  *) exit 1;;\nesac\n"
	if err := os.WriteFile(filepath.Join(binDir, "gh"), []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldDir) })
	return repo
}

func snapshotTree(t *testing.T, root string) map[string]string {
	t.Helper()
	snap := map[string]string{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		snap[p] = string(data) + "|" + info.ModTime().String()
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return snap
}

func levelOf(results []doctorResult, name string) (string, string) {
	for _, r := range results {
		if r.Name == name {
			return r.Level, r.Message
		}
	}
	return "", ""
}

func TestDoctorEmailChecks(t *testing.T) {
	tests := []struct {
		name       string
		email      string
		body       string
		fail       bool
		wantLevel  string
		wantSubstr string
	}{
		{"empty email fails", "", `[]`, false, levelFail, "user.email is not set"},
		{"verified email passes", "Me@Example.com", `[{"email":"me@example.com","verified":true,"primary":true}]`, false, levelPass, "verified on your GitHub account"},
		{"absent email warns", "me@work.com", `[{"email":"me@example.com","verified":true}]`, false, levelWarn, "not linked"},
		{"unverified email warns", "me@example.com", `[{"email":"me@example.com","verified":false}]`, false, levelWarn, "not verified"},
		{"endpoint failure is could-not-verify", "me@example.com", ``, true, levelWarn, "could not verify"},
		{"noreply address passes", "1+alice@users.noreply.github.com", `[]`, false, levelPass, "noreply"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupDoctorRepo(t, tt.email, tt.body, tt.fail)
			results := runDoctorChecks(defaultDoctorDeps())
			level, msg := levelOf(results, "git author email")
			if level != tt.wantLevel || !strings.Contains(msg, tt.wantSubstr) {
				t.Fatalf("got %s %q, want %s containing %q", level, msg, tt.wantLevel, tt.wantSubstr)
			}
			var buf bytes.Buffer
			err := renderDoctor(&buf, results)
			if (tt.wantLevel == levelFail) != (err != nil) {
				t.Fatalf("renderDoctor error = %v for level %s", err, tt.wantLevel)
			}
		})
	}
}

func TestDoctorFailsOutsideInitializedRepo(t *testing.T) {
	d := defaultDoctorDeps()
	d.pathExists = func(string) bool { return false }
	d.lookPath = func(string) error { return os.ErrNotExist }
	results := runDoctorChecks(d)
	for _, name := range []string{"git repository", "vanity initialized", "git installed", "gh installed"} {
		if level, _ := levelOf(results, name); level != levelFail {
			t.Errorf("%s level = %q, want FAIL", name, level)
		}
	}
}

func TestDoctorRemoteWarnsWhenMissing(t *testing.T) {
	setupDoctorRepo(t, "me@example.com", `[{"email":"me@example.com","verified":true}]`, false)
	results := runDoctorChecks(defaultDoctorDeps())
	if level, _ := levelOf(results, "remote"); level != levelWarn {
		t.Fatalf("remote level = %q, want WARN", level)
	}
	if level, _ := levelOf(results, "gh authenticated"); level != levelPass {
		t.Fatalf("gh authenticated level = %q, want PASS", level)
	}
}

func TestDoctorModifiesNothing(t *testing.T) {
	repo := setupDoctorRepo(t, "me@work.com", `[{"email":"me@example.com","verified":true}]`, false)
	if err := os.WriteFile(filepath.Join(repo, ".vanity", "alice.json"), []byte(`{}`), 0644); err != nil {
		t.Fatal(err)
	}
	before := snapshotTree(t, repo)
	var buf bytes.Buffer
	_ = renderDoctor(&buf, runDoctorChecks(defaultDoctorDeps()))
	if after := snapshotTree(t, repo); !reflect.DeepEqual(before, after) {
		t.Fatal("doctor changed files in the repository")
	}
}

func TestDoctorRegisteredOnRoot(t *testing.T) {
	for _, c := range rootCmd.Commands() {
		if c.Name() == "doctor" {
			return
		}
	}
	t.Fatal("doctor not registered on root command")
}
