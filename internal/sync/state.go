package sync

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

const vanityDir = ".vanity"

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
	return os.WriteFile(path, jsonData, 0644)
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
	return os.WriteFile(path, jsonData, 0644)
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

// GetTotalMirroredDates returns the count of unique dates mirrored from a user
func (s *SyncState) GetTotalMirroredDates(sourceUser string) int {
	userCounts, ok := s.MirroredCounts[sourceUser]
	if !ok {
		return 0
	}
	return len(userCounts)
}
