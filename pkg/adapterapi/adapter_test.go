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
