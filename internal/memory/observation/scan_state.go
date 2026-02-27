package observation

import (
	"os"
	"time"

	"github.com/pelletier/go-toml/v2"
)

// FileProgress tracks parsing progress for a single session file.
type FileProgress struct {
	Offset   int64     `toml:"offset"`
	ModTime  time.Time `toml:"mod_time"`
	ParsedAt time.Time `toml:"parsed_at"`
	Agent    string    `toml:"agent"`
}

// ScanState tracks incremental parsing progress across all session files.
type ScanState struct {
	Version     int                     `toml:"version"`
	Files       map[string]FileProgress `toml:"files"`
	MtimeAgents map[string]time.Time    `toml:"mtime_agents"`
}

func newScanState() ScanState {
	return ScanState{
		Version:     1,
		Files:       map[string]FileProgress{},
		MtimeAgents: map[string]time.Time{},
	}
}

func loadScanState(path string) ScanState {
	state := newScanState()
	if path == "" {
		return state
	}
	blob, err := os.ReadFile(path)
	if err != nil {
		return state
	}
	_ = toml.Unmarshal(blob, &state)
	if state.Files == nil {
		state.Files = map[string]FileProgress{}
	}
	if state.MtimeAgents == nil {
		state.MtimeAgents = map[string]time.Time{}
	}
	return state
}

func saveScanState(path string, state ScanState) {
	if path == "" {
		return
	}
	blob, err := toml.Marshal(state)
	if err != nil {
		return
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, blob, 0o644); err != nil {
		return
	}
	_ = os.Rename(tmp, path)
}

// GC removes entries for files that no longer exist or are older than maxAge.
func (s *ScanState) GC(maxAge time.Duration) {
	now := time.Now()
	for path, fp := range s.Files {
		if now.Sub(fp.ParsedAt) > maxAge {
			delete(s.Files, path)
			continue
		}
		if _, err := os.Stat(path); os.IsNotExist(err) {
			delete(s.Files, path)
		}
	}
}
