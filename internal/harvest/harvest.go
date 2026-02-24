package harvest

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"skillpm/internal/adapter"
	"skillpm/internal/importer"
	"skillpm/internal/store"
	"skillpm/pkg/adapterapi"
)

type Service struct {
	Runtime   *adapter.Runtime
	StateRoot string
}

type InboxEntry struct {
	Agent     string    `json:"agent"`
	Path      string    `json:"path"`
	SkillName string    `json:"skillName"`
	Valid     bool      `json:"valid"`
	Reason    string    `json:"reason,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}

func (s *Service) Harvest(ctx context.Context, agentName string) ([]InboxEntry, string, error) {
	if s.Runtime == nil {
		return nil, "", fmt.Errorf("HRV_RUNTIME: runtime not configured")
	}
	adp, err := s.Runtime.Get(agentName)
	if err != nil {
		return nil, "", err
	}
	res, err := adp.HarvestCandidates(ctx, adapterapi.HarvestRequest{Scope: "global"})
	if err != nil {
		return nil, "", err
	}
	entries := make([]InboxEntry, 0, len(res.Candidates))
	for _, c := range res.Candidates {
		entry := InboxEntry{Agent: agentName, Path: c.Path, SkillName: c.Name, CreatedAt: time.Now().UTC()}
		if _, err := importer.ValidateSkillDir(c.Path); err != nil {
			entry.Valid = false
			entry.Reason = err.Error()
		} else {
			entry.Valid = true
		}
		entries = append(entries, entry)
	}
	path, err := s.persistInbox(entries)
	if err != nil {
		return nil, "", err
	}
	return entries, path, nil
}

func (s *Service) persistInbox(entries []InboxEntry) (string, error) {
	if err := store.EnsureLayout(s.StateRoot); err != nil {
		return "", err
	}
	name := fmt.Sprintf("harvest-%d.json", time.Now().UnixNano())
	path := filepath.Join(store.InboxRoot(s.StateRoot), name)
	blob, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, blob, 0o644); err != nil {
		return "", err
	}
	return path, nil
}
