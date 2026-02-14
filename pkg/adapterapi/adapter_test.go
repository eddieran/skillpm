package adapterapi

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestInjectResultJSONTags(t *testing.T) {
	v := InjectResult{
		Agent:            "openclaw",
		Injected:         []string{"src/skill"},
		SnapshotPath:     "/tmp/snap",
		RollbackPossible: true,
	}
	blob, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	s := string(blob)
	for _, key := range []string{"\"agent\"", "\"injected\"", "\"snapshotPath\"", "\"rollbackPossible\""} {
		if !strings.Contains(s, key) {
			t.Fatalf("expected key %s in %s", key, s)
		}
	}
}

func TestHarvestResultJSONTags(t *testing.T) {
	v := HarvestResult{
		Agent:     "codex",
		Supported: true,
		Candidates: []HarvestCandidate{{
			Path:    "/tmp/skill",
			Name:    "skill",
			Adapter: "codex",
		}},
	}
	blob, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	s := string(blob)
	for _, key := range []string{"\"agent\"", "\"candidates\"", "\"supported\""} {
		if !strings.Contains(s, key) {
			t.Fatalf("expected key %s in %s", key, s)
		}
	}
}
