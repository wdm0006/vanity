package cli

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
	"github.com/wdm0006/vanity/internal/git"
	"github.com/wdm0006/vanity/internal/github"
)

const (
	levelPass = "PASS"
	levelWarn = "WARN"
	levelFail = "FAIL"
)

type doctorResult struct {
	Level   string
	Name    string
	Message string
}

// doctorDeps holds the external lookups so checks can be driven in tests.
type doctorDeps struct {
	lookPath    func(string) error
	pathExists  func(string) bool
	currentUser func() (string, error)
	gitEmail    func() (string, error)
	userEmails  func() ([]github.UserEmail, error)
	hasRemote   func() bool
}

func defaultDoctorDeps() doctorDeps {
	return doctorDeps{
		lookPath: func(name string) error {
			_, err := exec.LookPath(name)
			return err
		},
		pathExists: func(p string) bool {
			_, err := os.Stat(p)
			return err == nil
		},
		currentUser: github.GetCurrentUser,
		gitEmail: func() (string, error) {
			out, err := exec.Command("git", "config", "user.email").Output()
			return strings.TrimSpace(string(out)), err
		},
		userEmails: github.GetUserEmails,
		hasRemote:  git.HasRemote,
	}
}

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check that this repository and account are ready to sync",
	Long: `Runs read-only preflight checks and prints one PASS/WARN/FAIL line per check.

Mirror commits only count on your contribution graph when the git author
email is verified on your GitHub account, so doctor checks that
'git config user.email' is set and linked to the account.

Exits 1 if any check fails.`,
	Example:       `  vanity doctor`,
	SilenceUsage:  true,
	SilenceErrors: false,
	RunE: func(cmd *cobra.Command, args []string) error {
		results := runDoctorChecks(defaultDoctorDeps())
		return renderDoctor(os.Stdout, results)
	},
}

func renderDoctor(w io.Writer, results []doctorResult) error {
	failed := 0
	for _, r := range results {
		fmt.Fprintf(w, "%s  %s: %s\n", r.Level, r.Name, r.Message)
		if r.Level == levelFail {
			failed++
		}
	}
	if failed > 0 {
		return fmt.Errorf("%d check(s) failed", failed)
	}
	return nil
}

func runDoctorChecks(d doctorDeps) []doctorResult {
	var results []doctorResult
	add := func(level, name, msg string) {
		results = append(results, doctorResult{level, name, msg})
	}

	if d.pathExists(".git") {
		add(levelPass, "git repository", ".git found")
	} else {
		add(levelFail, "git repository", ".git not found (run from the root of your sync repository)")
	}
	if d.pathExists(".vanity") {
		add(levelPass, "vanity initialized", ".vanity/ found")
	} else {
		add(levelFail, "vanity initialized", ".vanity/ not found (run 'vanity init')")
	}

	gitOK := d.lookPath("git") == nil
	if gitOK {
		add(levelPass, "git installed", "git found on PATH")
	} else {
		add(levelFail, "git installed", "git not found on PATH")
	}
	ghOK := d.lookPath("gh") == nil
	if ghOK {
		add(levelPass, "gh installed", "gh found on PATH")
	} else {
		add(levelFail, "gh installed", "gh not found on PATH (install from https://cli.github.com)")
	}

	authed := false
	if ghOK {
		if user, err := d.currentUser(); err != nil {
			add(levelFail, "gh authenticated", firstLine(err.Error()))
		} else {
			authed = true
			add(levelPass, "gh authenticated", "logged in as "+user)
		}
	}

	if gitOK {
		email, _ := d.gitEmail()
		if email == "" {
			add(levelFail, "git author email", "user.email is not set (run 'git config user.email <address verified on your GitHub account>')")
		} else if !authed {
			add(levelWarn, "git author email", email+" is set, could not verify it against GitHub (gh not authenticated)")
		} else {
			results = append(results, checkEmailLinked(email, d.userEmails))
		}

		if d.hasRemote() {
			add(levelPass, "remote", "a remote is configured")
		} else {
			add(levelWarn, "remote", "no remote configured; sync will not push")
		}
	}
	return results
}

func checkEmailLinked(email string, fetch func() ([]github.UserEmail, error)) doctorResult {
	const name = "git author email"
	if strings.HasSuffix(strings.ToLower(email), "@users.noreply.github.com") {
		return doctorResult{levelPass, name, email + " is a GitHub noreply address"}
	}
	emails, err := fetch()
	if err != nil {
		return doctorResult{levelWarn, name, email + " is set, could not verify it against your GitHub account (" + firstLine(err.Error()) + ")"}
	}
	for _, e := range emails {
		if strings.EqualFold(e.Email, email) {
			if e.Verified {
				return doctorResult{levelPass, name, email + " is verified on your GitHub account"}
			}
			return doctorResult{levelWarn, name, email + " is on your GitHub account but not verified; mirror commits will not count"}
		}
	}
	return doctorResult{levelWarn, name, email + " is not linked to your GitHub account; mirror commits will not count"}
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
