package e2e

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
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

	runCLI(t, bin, env, "--config", cfgPath, "source", "add", "myhub", server.URL+"/", "--kind", "clawhub")
	runCLI(t, bin, env, "--config", cfgPath, "source", "update", "myhub")
	searchOut := runCLI(t, bin, env, "--config", cfgPath, "search", "forms", "--source", "myhub")
	assertContains(t, searchOut, "myhub/forms")

	failOut := runCLIExpectFail(t, bin, env, "--config", cfgPath, "install", "myhub/forms", "--lockfile", lockPath)
	assertContains(t, failOut, "SEC_SUSPICIOUS_CONFIRM")

	runCLI(t, bin, env, "--config", cfgPath, "install", "myhub/forms", "--lockfile", lockPath, "--force")
	runCLI(t, bin, env, "--config", cfgPath, "inject", "--agent", "openclaw")

	runCLI(t, bin, env, "--config", cfgPath, "sync", "--lockfile", lockPath, "--force")

	runCLI(t, bin, env, "--config", cfgPath, "schedule", "install", "2h")
	scheduleOut := runCLI(t, bin, env, "--config", cfgPath, "schedule", "list")
	assertContains(t, scheduleOut, "interval=2h")
	runCLI(t, bin, env, "--config", cfgPath, "schedule", "remove")

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

	repoURL := setupBareRepoE2E(t, map[string]map[string]string{
		"demo": {"SKILL.md": "# demo\nDemo skill"},
	})
	// Create claude adapter dir so it's detected
	claudeDir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatalf("create claude dir failed: %v", err)
	}

	runCLI(t, bin, env, "--config", cfgPath, "source", "add", "local", repoURL, "--kind", "git")
	runCLI(t, bin, env, "--config", cfgPath, "doctor", "--enable-detected")
	runCLI(t, bin, env, "--config", cfgPath, "install", "local/demo", "--lockfile", lockPath)
	runCLI(t, bin, env, "--config", cfgPath, "inject", "--agent", "claude")
	out := runCLI(t, bin, env, "--config", cfgPath, "doctor")
	assertContains(t, out, "healthy")

	if _, err := os.Stat(lockPath); err != nil {
		t.Fatalf("expected lockfile to exist: %v", err)
	}

	// Verify SKILL.md has real content in the agent skills dir
	skillMdPath := filepath.Join(claudeDir, "skills", "demo", "SKILL.md")
	data, err := os.ReadFile(skillMdPath)
	if err != nil {
		t.Fatalf("read injected SKILL.md failed: %v", err)
	}
	assertContains(t, string(data), "Demo skill")
}

func TestCLISyncJSONDryRunNextBatchSignals(t *testing.T) {
	home := t.TempDir()
	bin, env := buildCLI(t, home)
	cfgPath := filepath.Join(home, ".skillpm", "config.toml")
	lockPath := filepath.Join(home, "workspace", "skills.lock")

	repoURL := setupBareRepoE2E(t, map[string]map[string]string{
		"demo": {"SKILL.md": "# demo\nDemo skill"},
	})
	runCLI(t, bin, env, "--config", cfgPath, "source", "add", "local", repoURL, "--kind", "git")
	runCLI(t, bin, env, "--config", cfgPath, "install", "local/demo", "--lockfile", lockPath)

	out := runCLI(t, bin, env, "--config", cfgPath, "sync", "--lockfile", lockPath, "--dry-run", "--json")

	var got struct {
		SchemaVersion    string `json:"schemaVersion"`
		Mode             string `json:"mode"`
		DryRun           bool   `json:"dryRun"`
		CanProceed       bool   `json:"canProceed"`
		NextBatchReady   bool   `json:"nextBatchReady"`
		NextBatchBlocker string `json:"nextBatchBlocker"`
	}
	trimJSON := out
	if idx := strings.Index(trimJSON, "{"); idx > 0 {
		trimJSON = trimJSON[idx:]
	}
	if err := json.Unmarshal([]byte(trimJSON), &got); err != nil {
		t.Fatalf("unmarshal sync json failed: %v\noutput=%s", err, out)
	}

	if got.SchemaVersion != "v1" {
		t.Fatalf("expected schemaVersion=v1, got %q", got.SchemaVersion)
	}
	if got.Mode != "dry-run" || !got.DryRun {
		t.Fatalf("expected dry-run mode, got mode=%q dryRun=%v", got.Mode, got.DryRun)
	}
	if !got.CanProceed {
		t.Fatalf("expected canProceed=true in dry-run summary")
	}
	if got.NextBatchReady {
		t.Fatalf("expected nextBatchReady=false in dry-run summary")
	}
	if got.NextBatchBlocker != "dry-run-mode" {
		t.Fatalf("expected nextBatchBlocker=dry-run-mode, got %q", got.NextBatchBlocker)
	}
}

func TestCLISyncJSONApplyNextBatchSignals(t *testing.T) {
	home := t.TempDir()
	bin, env := buildCLI(t, home)
	cfgPath := filepath.Join(home, ".skillpm", "config.toml")
	lockPath := filepath.Join(home, "workspace", "skills.lock")

	repoURL := setupBareRepoE2E(t, map[string]map[string]string{
		"demo": {"SKILL.md": "# demo\nDemo skill"},
	})
	runCLI(t, bin, env, "--config", cfgPath, "source", "add", "local", repoURL, "--kind", "git")
	runCLI(t, bin, env, "--config", cfgPath, "install", "local/demo", "--lockfile", lockPath)

	out := runCLI(t, bin, env, "--config", cfgPath, "sync", "--lockfile", lockPath, "--json")

	var got struct {
		SchemaVersion    string `json:"schemaVersion"`
		Mode             string `json:"mode"`
		DryRun           bool   `json:"dryRun"`
		CanProceed       bool   `json:"canProceed"`
		NextBatchReady   bool   `json:"nextBatchReady"`
		NextBatchBlocker string `json:"nextBatchBlocker"`
	}
	trimJSON := out
	if idx := strings.Index(trimJSON, "{"); idx > 0 {
		trimJSON = trimJSON[idx:]
	}
	if err := json.Unmarshal([]byte(trimJSON), &got); err != nil {
		t.Fatalf("unmarshal sync json failed: %v\noutput=%s", err, out)
	}

	if got.SchemaVersion != "v1" {
		t.Fatalf("expected schemaVersion=v1, got %q", got.SchemaVersion)
	}
	if got.Mode != "apply" || got.DryRun {
		t.Fatalf("expected apply mode, got mode=%q dryRun=%v", got.Mode, got.DryRun)
	}
	if !got.CanProceed {
		t.Fatalf("expected canProceed=true in apply summary")
	}
	if !got.NextBatchReady {
		t.Fatalf("expected nextBatchReady=true in apply summary")
	}
	if got.NextBatchBlocker != "none" {
		t.Fatalf("expected nextBatchBlocker=none, got %q", got.NextBatchBlocker)
	}
}
