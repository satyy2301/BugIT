package timefreeze

import (
	"encoding/json"
	"os"
)

// Timeline stores clock_gettime values captured during recording.
type Timeline struct {
	Entries []Entry `json:"entries"`
}

type Entry struct {
	Index       int    `json:"index"`
	TimestampNs uint64 `json:"timestamp_ns"`
}

func LoadTimeline(path string) (*Timeline, error) {
	if path == "" {
		return &Timeline{}, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Timeline{}, nil
		}
		return nil, err
	}
	var tl Timeline
	if err := json.Unmarshal(data, &tl); err != nil {
		return nil, err
	}
	return &tl, nil
}

func WriteTimeline(path string, tl *Timeline) error {
	data, err := json.MarshalIndent(tl, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
