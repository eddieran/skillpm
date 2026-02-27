package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"skillpm/internal/config"
)

func TestEnableDetectedAdapters(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("OPENCLAW_STATE_DIR", filepath.Join(home, "openclaw-state"))
	for _, dir := range []string{filepath.Join(home, ".cursor"), filepath.Join(home, ".claude"), filepath.Join(home, "openclaw-state")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir failed: %v", err)
		}
	}
	svc, err := New(Options{ConfigPath: filepath.Join(home, ".skillpm", "config.toml")})
	if err != nil {
		t.Fatalf("new service failed: %v", err)
	}
	_, err = svc.EnableDetectedAdapters()
	if err != nil {
		t.Fatalf("enable detected failed: %v", err)
	}
	// Adapters may already be in default config; verify they are enabled.
	for _, name := range []string{"cursor", "claude", "openclaw"} {
		a, ok := config.FindAdapter(svc.Config, name)
		if !ok || !a.Enabled {
			t.Fatalf("expected adapter %s enabled in config", name)
		}
	}
}

func TestScheduleWritesBackendFiles(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("SKILLPM_SCHEDULER_ROOT", filepath.Join(home, "scheduler-root"))
	t.Setenv("SKILLPM_SCHEDULER_SKIP_COMMANDS", "1")
	svc, err := New(Options{ConfigPath: filepath.Join(home, ".skillpm", "config.toml")})
	if err != nil {
		t.Fatalf("new service failed: %v", err)
	}
	if _, err := svc.Schedule("install", "3h"); err != nil {
		t.Fatalf("schedule install failed: %v", err)
	}
	filesFound := 0
	_ = filepath.Walk(filepath.Join(home, "scheduler-root"), func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			filesFound++
		}
		return nil
	})
	if filesFound == 0 {
		t.Fatalf("expected scheduler files to be written")
	}
	if _, err := svc.Schedule("remove", ""); err != nil {
		t.Fatalf("schedule remove failed: %v", err)
	}
}

func TestServiceSelfUpdate(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	target := filepath.Join(home, "skillpm-bin")
	if err := os.WriteFile(target, []byte("old-bin"), 0o755); err != nil {
		t.Fatalf("write target failed: %v", err)
	}

	newBin := []byte("new-bin")
	h := sha256.Sum256(newBin)
	checksum := hex.EncodeToString(h[:])
	mux := http.NewServeMux()
	mux.HandleFunc("/bin", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(newBin)
	})
	mux.HandleFunc("/manifest", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"version":  "9.9.9",
			"url":      "/bin",
			"checksum": checksum,
		})
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	t.Setenv("SKILLPM_UPDATE_MANIFEST_URL", server.URL+"/manifest")
	t.Setenv("SKILLPM_SELF_UPDATE_TARGET", target)

	svc, err := New(Options{ConfigPath: filepath.Join(home, ".skillpm", "config.toml"), HTTPClient: server.Client()})
	if err != nil {
		t.Fatalf("new service failed: %v", err)
	}
	svc.Config.Security.RequireSignatures = false
	if err := svc.SelfUpdate(context.Background(), "stable"); err != nil {
		t.Fatalf("self update failed: %v", err)
	}
	blob, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read target failed: %v", err)
	}
	if string(blob) != string(newBin) {
		t.Fatalf("expected target to be updated")
	}
}
