package sync

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
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
