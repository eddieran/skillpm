package e2e

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"skillpm/internal/store"
)

func TestCLICriticalFlowSideBySide(t *testing.T) {
	home := t.TempDir()
	bin, env := buildCLI(t, home)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		base := "http://" + r.Host
		switch {
		case r.URL.Path == "/.well-known/clawhub.json":
			_ = json.NewEncoder(w).Encode(map[string]string{"apiBase": base + "/"})
		case r.URL.Path == "/api/v1/search":
			_ = json.NewEncoder(w).Encode(map[string]any{"items": []map[string]string{{"slug": "forms", "description": "forms skill"}}})
		case r.URL.Path == "/api/v1/skills/forms":
			_ = json.NewEncoder(w).Encode(map[string]any{"moderation": map[string]bool{"isSuspicious": true, "isMalwareBlocked": false}})
		case r.URL.Path == "/api/v1/skills/forms/versions":
			_ = json.NewEncoder(w).Encode(map[string]any{"versions": []string{"1.0.0", "1.1.0"}})
		case r.URL.Path == "/api/v1/download":
			_ = json.NewEncoder(w).Encode(map[string]any{"version": "1.1.0", "content": "artifact-content"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	cfgPath := filepath.Join(home, ".skillpm", "config.toml")
	lockPath := filepath.Join(home, "workspace", "skills.lock")
	candidateDir := filepath.Join(home, "openclaw-state", "skillpm", "candidate-skill")

	runCLI(t, bin, env, "--config", cfgPath, "source", "add", "myhub", server.URL+"/", "--kind", "clawhub")
	runCLI(t, bin, env, "--config", cfgPath, "source", "update", "myhub")
	searchOut := runCLI(t, bin, env, "--config", cfgPath, "search", "forms", "--source", "myhub")
	assertContains(t, searchOut, "myhub/forms")

	failOut := runCLIExpectFail(t, bin, env, "--config", cfgPath, "install", "myhub/forms", "--lockfile", lockPath)
	assertContains(t, failOut, "SEC_SUSPICIOUS_CONFIRM")

	runCLI(t, bin, env, "--config", cfgPath, "install", "myhub/forms", "--lockfile", lockPath, "--force")
	runCLI(t, bin, env, "--config", cfgPath, "inject", "--agent", "openclaw")

	if err := os.MkdirAll(candidateDir, 0o755); err != nil {
		t.Fatalf("create candidate dir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(candidateDir, "SKILL.md"), []byte("# candidate"), 0o644); err != nil {
		t.Fatalf("write candidate SKILL.md failed: %v", err)
	}
	runCLI(t, bin, env, "--config", cfgPath, "harvest", "--agent", "openclaw")
	runCLI(t, bin, env, "--config", cfgPath, "validate", candidateDir)
	runCLI(t, bin, env, "--config", cfgPath, "sync", "--lockfile", lockPath, "--force")

	runCLI(t, bin, env, "--config", cfgPath, "schedule", "install", "2h")
	scheduleOut := runCLI(t, bin, env, "--config", cfgPath, "schedule", "list")
	assertContains(t, scheduleOut, "interval=2h")
	runCLI(t, bin, env, "--config", cfgPath, "schedule", "remove")

	runCLI(t, bin, env, "--config", cfgPath, "remove", "--agent", "openclaw", "myhub/forms")
	runCLI(t, bin, env, "--config", cfgPath, "uninstall", "myhub/forms", "--lockfile", lockPath)

	stateRoot := filepath.Join(home, ".skillpm")
	st, err := store.LoadState(stateRoot)
	if err != nil {
		t.Fatalf("load state failed: %v", err)
	}
	if len(st.Installed) != 0 {
		t.Fatalf("expected no installed skills after uninstall, got %d", len(st.Installed))
	}
	lock, err := store.LoadLockfile(lockPath)
	if err != nil {
		t.Fatalf("load lockfile failed: %v", err)
	}
	if len(lock.Skills) != 0 {
		t.Fatalf("expected empty lockfile after uninstall, got %d entries", len(lock.Skills))
	}

	doctorOut := runCLI(t, bin, env, "--config", cfgPath, "doctor")
	assertContains(t, doctorOut, "healthy")
}

func TestCLIEndToEndLocalFlow(t *testing.T) {
	home := t.TempDir()
	bin, env := buildCLI(t, home)
	cfgPath := filepath.Join(home, ".skillpm", "config.toml")
	lockPath := filepath.Join(home, "workspace", "skills.lock")

	runCLI(t, bin, env, "--config", cfgPath, "source", "add", "local", "https://example.com/skills.git", "--kind", "git")
	runCLI(t, bin, env, "--config", cfgPath, "install", "local/demo", "--lockfile", lockPath)
	runCLI(t, bin, env, "--config", cfgPath, "inject", "--agent", "openclaw")
	out := runCLI(t, bin, env, "--config", cfgPath, "doctor")
	assertContains(t, out, "healthy")

	if _, err := os.Stat(lockPath); err != nil {
		t.Fatalf("expected lockfile to exist: %v", err)
	}
}
