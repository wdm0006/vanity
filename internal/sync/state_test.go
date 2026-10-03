package sync

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"testing"
	"time"
)

func TestSetMirroredCountBuildsMissingMaps(t *testing.T) {
	// A state built by hand starts with a nil MirroredCounts map; SetMirroredCount
	// has to create both the outer user map and the inner date map before writing.
	state := &SyncState{Username: "alice"}

	state.SetMirroredCount("bob", "2024-01-05", 3)
	state.SetMirroredCount("bob", "2024-01-06", 7)
	state.SetMirroredCount("carol", "2024-01-05", 1)

	want := map[string]map[string]int{
		"bob":   {"2024-01-05": 3, "2024-01-06": 7},
		"carol": {"2024-01-05": 1},
	}
	if !reflect.DeepEqual(state.MirroredCounts, want) {
		t.Fatalf("MirroredCounts = %#v, want %#v", state.MirroredCounts, want)
	}
}

func TestSetMirroredCountOverwritesTheRecordedTotal(t *testing.T) {
	state := &SyncState{Username: "alice"}

	state.SetMirroredCount("bob", "2024-01-05", 3)
	state.SetMirroredCount("bob", "2024-01-05", 9)

	if got := state.GetMirroredCount("bob", "2024-01-05"); got != 9 {
		t.Errorf("GetMirroredCount = %d, want 9 (the total replaces, it does not accumulate)", got)
	}
}

func TestGetMirroredCountReportsZeroForUnrecordedEntries(t *testing.T) {
	state := &SyncState{Username: "alice"}
	if got := state.GetMirroredCount("bob", "2024-01-05"); got != 0 {
		t.Errorf("nil map: GetMirroredCount = %d, want 0", got)
	}

	state.SetMirroredCount("bob", "2024-01-05", 4)
	if got := state.GetMirroredCount("carol", "2024-01-05"); got != 0 {
		t.Errorf("unknown user: GetMirroredCount = %d, want 0", got)
	}
	if got := state.GetMirroredCount("bob", "2024-02-01"); got != 0 {
		t.Errorf("unknown date: GetMirroredCount = %d, want 0", got)
	}
	if got := state.GetMirroredCount("bob", "2024-01-05"); got != 4 {
		t.Errorf("recorded entry: GetMirroredCount = %d, want 4", got)
	}
}

func TestClearAllMirroredCountsDropsEveryUser(t *testing.T) {
	state := &SyncState{Username: "alice"}
	state.SetMirroredCount("bob", "2024-01-05", 3)
	state.SetMirroredCount("carol", "2024-02-11", 8)

	state.ClearAllMirroredCounts()

	if len(state.MirroredCounts) != 0 {
		t.Fatalf("MirroredCounts = %#v, want empty", state.MirroredCounts)
	}
	if got := state.GetMirroredCount("bob", "2024-01-05"); got != 0 {
		t.Errorf("GetMirroredCount after clear = %d, want 0", got)
	}
	// The map must be usable again, not left nil, so a rebuild can re-record.
	state.SetMirroredCount("bob", "2024-01-05", 3)
	if got := state.GetMirroredCount("bob", "2024-01-05"); got != 3 {
		t.Errorf("GetMirroredCount after re-record = %d, want 3", got)
	}
}

func TestGetTotalMirroredDatesCountsDistinctDates(t *testing.T) {
	state := &SyncState{Username: "alice"}
	if got := state.GetTotalMirroredDates("bob"); got != 0 {
		t.Errorf("nil map: GetTotalMirroredDates = %d, want 0", got)
	}

	state.SetMirroredCount("bob", "2024-01-05", 3)
	state.SetMirroredCount("bob", "2024-01-06", 7)
	state.SetMirroredCount("bob", "2024-01-06", 9) // same date again
	state.SetMirroredCount("carol", "2024-01-05", 1)

	if got := state.GetTotalMirroredDates("bob"); got != 2 {
		t.Errorf("GetTotalMirroredDates(bob) = %d, want 2", got)
	}
	if got := state.GetTotalMirroredDates("carol"); got != 1 {
		t.Errorf("GetTotalMirroredDates(carol) = %d, want 1", got)
	}
	if got := state.GetTotalMirroredDates("dave"); got != 0 {
		t.Errorf("GetTotalMirroredDates(dave) = %d, want 0", got)
	}
}

func TestListSyncedUsersExcludesStateFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, vanityDir), 0755); err != nil {
		t.Fatalf("create %s: %v", vanityDir, err)
	}
	// vanity init writes .gitkeep, and every synced identity that has actually
	// run sync leaves a <user>-state.json beside its <user>.json.
	for _, name := range []string{
		".gitkeep",
		"alice.json",
		"alice-state.json",
		"bob.json",
		"state.json",
		"notes.txt",
		"json",
		".json",
	} {
		if err := os.WriteFile(filepath.Join(dir, vanityDir, name), []byte("{}"), 0644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	var users []string
	withWorkingDirectory(t, dir, func() {
		var err error
		users, err = ListSyncedUsers()
		if err != nil {
			t.Fatalf("ListSyncedUsers: %v", err)
		}
	})

	sort.Strings(users)
	want := []string{"alice", "bob", "state"}
	if !reflect.DeepEqual(users, want) {
		t.Fatalf("ListSyncedUsers = %#v, want %#v", users, want)
	}
}

func TestListSyncedUsersFailsWithoutAVanityDirectory(t *testing.T) {
	dir := t.TempDir()

	withWorkingDirectory(t, dir, func() {
		if _, err := ListSyncedUsers(); err == nil {
			t.Fatal("expected an error when .vanity/ is missing, got nil")
		}
	})
}

var saveCases = []struct {
	name string
	file string
	save func() error
	want func() ([]byte, error)
}{
	{
		name: "contribution data",
		file: "alice.json",
		save: func() error { return SaveContributionData(testContributionData()) },
		want: func() ([]byte, error) { return json.MarshalIndent(testContributionData(), "", "  ") },
	},
	{
		name: "sync state",
		file: "alice-state.json",
		save: func() error { return SaveSyncState(testSyncState()) },
		want: func() ([]byte, error) { return json.MarshalIndent(testSyncState(), "", "  ") },
	},
}

func testContributionData() *ContributionData {
	return &ContributionData{
		Username:    "alice",
		LastUpdated: time.Date(2024, 3, 1, 12, 0, 0, 0, time.UTC),
		Contributions: []Contribution{
			{Date: "2024-01-05", Count: 3},
			{Date: "2024-01-06", Count: 7},
		},
	}
}

func testSyncState() *SyncState {
	return &SyncState{
		Username:       "alice",
		LastSync:       time.Date(2024, 3, 1, 12, 0, 0, 0, time.UTC),
		MirroredCounts: map[string]map[string]int{"bob": {"2024-01-05": 2}},
	}
}

func vanityDirEntries(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir(vanityDir)
	if err != nil {
		t.Fatalf("read %s: %v", vanityDir, err)
	}
	var names []string
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names
}

func TestSaveWritesIndentedJSONWithMode0644(t *testing.T) {
	for _, tc := range saveCases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.Mkdir(filepath.Join(dir, vanityDir), 0755); err != nil {
				t.Fatalf("create %s: %v", vanityDir, err)
			}
			withWorkingDirectory(t, dir, func() {
				path := filepath.Join(vanityDir, tc.file)
				// Overwrite an existing file to exercise replacement, not just creation.
				if err := os.WriteFile(path, []byte("old"), 0600); err != nil {
					t.Fatalf("seed %s: %v", path, err)
				}

				if err := tc.save(); err != nil {
					t.Fatalf("save: %v", err)
				}

				want, err := tc.want()
				if err != nil {
					t.Fatalf("marshal: %v", err)
				}
				got, err := os.ReadFile(path)
				if err != nil {
					t.Fatalf("read %s: %v", path, err)
				}
				if string(got) != string(want) {
					t.Errorf("%s =\n%s\nwant\n%s", path, got, want)
				}
				if runtime.GOOS != "windows" {
					info, err := os.Stat(path)
					if err != nil {
						t.Fatalf("stat %s: %v", path, err)
					}
					if mode := info.Mode().Perm(); mode != 0644 {
						t.Errorf("%s mode = %o, want 644", path, mode)
					}
				}
				if names := vanityDirEntries(t); !reflect.DeepEqual(names, []string{tc.file}) {
					t.Errorf("%s contains %v, want only %s", vanityDir, names, tc.file)
				}
			})
		})
	}
}

func TestSaveFailureBeforeReplacementPreservesExistingFile(t *testing.T) {
	for _, tc := range saveCases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.Mkdir(filepath.Join(dir, vanityDir), 0755); err != nil {
				t.Fatalf("create %s: %v", vanityDir, err)
			}
			replaceErr := errors.New("simulated replace failure")
			original := renameFile
			renameFile = func(string, string) error { return replaceErr }
			t.Cleanup(func() { renameFile = original })

			withWorkingDirectory(t, dir, func() {
				path := filepath.Join(vanityDir, tc.file)
				existing := []byte("{\n  \"username\": \"alice\",\n  \"previous\": true\n}")
				if err := os.WriteFile(path, existing, 0644); err != nil {
					t.Fatalf("seed %s: %v", path, err)
				}

				if err := tc.save(); !errors.Is(err, replaceErr) {
					t.Errorf("save error = %v, want %v", err, replaceErr)
				}

				got, err := os.ReadFile(path)
				if err != nil {
					t.Fatalf("read %s: %v", path, err)
				}
				if string(got) != string(existing) {
					t.Errorf("%s changed after a failed save:\n%s\nwant\n%s", path, got, existing)
				}
				if names := vanityDirEntries(t); !reflect.DeepEqual(names, []string{tc.file}) {
					t.Errorf("%s contains %v after a failed save, want only %s", vanityDir, names, tc.file)
				}
			})
		})
	}
}
