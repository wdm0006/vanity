package sync

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

const vanityDir = ".vanity"

// renameFile publishes a completed temporary file; tests replace it to simulate
// a failure after the payload is written but before the destination changes.
var renameFile = os.Rename

// ContributionData holds contribution history for a user
type ContributionData struct {
	Username      string         `json:"username"`
	LastUpdated   time.Time      `json:"last_updated"`
	Contributions []Contribution `json:"contributions"`
}

// Contribution represents contributions for a single day
type Contribution struct {
	Date  string `json:"date"`
	Count int    `json:"count"`
}

// SyncState tracks what has been synced for a user
type SyncState struct {
	Username       string                    `json:"username"`
	LastSync       time.Time                 `json:"last_sync"`
	MirroredCounts map[string]map[string]int `json:"mirrored_counts"` // user -> date -> count mirrored
}

// LoadContributionData loads contribution data for a user
func LoadContributionData(username string) (*ContributionData, error) {
	path := filepath.Join(vanityDir, username+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &ContributionData{
				Username:      username,
				Contributions: []Contribution{},
			}, nil
		}
		return nil, err
	}

	var contribs ContributionData
	if err := json.Unmarshal(data, &contribs); err != nil {
		return nil, err
	}
	return &contribs, nil
}

// SaveContributionData saves contribution data for a user
func SaveContributionData(data *ContributionData) error {
	path := filepath.Join(vanityDir, data.Username+".json")
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(path, jsonData, 0644)
}

// LoadSyncState loads sync state for a user
func LoadSyncState(username string) (*SyncState, error) {
	path := filepath.Join(vanityDir, username+"-state.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &SyncState{
				Username:       username,
				MirroredCounts: make(map[string]map[string]int),
			}, nil
		}
		return nil, err
	}

	var state SyncState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}
	if state.MirroredCounts == nil {
		state.MirroredCounts = make(map[string]map[string]int)
	}
	return &state, nil
}

// SaveSyncState saves sync state for a user
func SaveSyncState(state *SyncState) error {
	path := filepath.Join(vanityDir, state.Username+"-state.json")
	jsonData, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(path, jsonData, 0644)
}

// writeFileAtomic writes data to a temporary file beside path and renames it
// over path, so a failed write never truncates the existing file.
func writeFileAtomic(path string, data []byte, perm os.FileMode) (err error) {
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer func() {
		if err != nil {
			tmp.Close()
			os.Remove(tmpPath)
		}
	}()

	if _, err = tmp.Write(data); err != nil {
		return err
	}
	if err = tmp.Sync(); err != nil {
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	if err = os.Chmod(tmpPath, perm); err != nil {
		return err
	}
	return renameFile(tmpPath, path)
}

// ListSyncedUsers returns a list of usernames that have contribution data
func ListSyncedUsers() ([]string, error) {
	entries, err := os.ReadDir(vanityDir)
	if err != nil {
		return nil, err
	}

	var users []string
	for _, entry := range entries {
		name := entry.Name()
		if len(name) > 5 && name[len(name)-5:] == ".json" && (len(name) < 11 || name[len(name)-11:] != "-state.json") {
			users = append(users, name[:len(name)-5])
		}
	}
	return users, nil
}

// GetMirroredCount returns how many contributions have been mirrored for a user/date
func (s *SyncState) GetMirroredCount(sourceUser, date string) int {
	userCounts, ok := s.MirroredCounts[sourceUser]
	if !ok {
		return 0
	}
	return userCounts[date]
}

// SetMirroredCount records how many contributions have been mirrored for a user/date
func (s *SyncState) SetMirroredCount(sourceUser, date string, count int) {
	if s.MirroredCounts == nil {
		s.MirroredCounts = make(map[string]map[string]int)
	}
	if s.MirroredCounts[sourceUser] == nil {
		s.MirroredCounts[sourceUser] = make(map[string]int)
	}
	s.MirroredCounts[sourceUser][date] = count
}

// ClearAllMirroredCounts resets all mirrored counts so a full rebuild will re-mirror everything
func (s *SyncState) ClearAllMirroredCounts() {
	s.MirroredCounts = make(map[string]map[string]int)
}

// GetTotalMirroredDates returns the count of unique dates mirrored from a user
func (s *SyncState) GetTotalMirroredDates(sourceUser string) int {
	userCounts, ok := s.MirroredCounts[sourceUser]
	if !ok {
		return 0
	}
	return len(userCounts)
}
