package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"skillpm/internal/store"
)

func TestServiceInstallInjectSyncFlow(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("OPENCLAW_STATE_DIR", filepath.Join(home, "openclaw-state"))
	t.Setenv("OPENCLAW_CONFIG_PATH", filepath.Join(home, "openclaw-config.toml"))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		base := "http://" + r.Host
		switch {
		case r.URL.Path == "/.well-known/clawhub.json":
			_ = json.NewEncoder(w).Encode(map[string]string{"apiBase": base + "/"})
		case r.URL.Path == "/api/v1/skills/forms":
			_ = json.NewEncoder(w).Encode(map[string]any{"moderation": map[string]bool{"isSuspicious": true, "isMalwareBlocked": false}})
		case r.URL.Path == "/api/v1/skills/forms/versions":
			_ = json.NewEncoder(w).Encode(map[string]any{"versions": []string{"1.0.0", "1.1.0"}})
		case r.URL.Path == "/api/v1/download":
			_ = json.NewEncoder(w).Encode(map[string]any{"version": "1.1.0", "content": "zipbytes"})
		case r.URL.Path == "/api/v1/search":
			_ = json.NewEncoder(w).Encode(map[string]any{"items": []map[string]string{{"slug": "forms", "description": "forms skill"}}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	cfgPath := filepath.Join(home, ".skillpm", "config.toml")
	svc, err := New(Options{ConfigPath: cfgPath, HTTPClient: server.Client()})
	if err != nil {
		t.Fatalf("new service failed: %v", err)
	}
	if _, err := svc.SourceAdd("myhub", server.URL+"/", "clawhub", "", "review"); err != nil {
		t.Fatalf("source add failed: %v", err)
	}
	if _, err := svc.SourceUpdate(context.Background(), "myhub"); err != nil {
		t.Fatalf("source update failed: %v", err)
	}
	if _, err := svc.Search(context.Background(), "myhub", "forms"); err != nil {
		t.Fatalf("search failed: %v", err)
	}

	lockPath := filepath.Join(home, "project", "skills.lock")
	if _, err := svc.Install(context.Background(), []string{"myhub/forms"}, lockPath, false); err == nil {
		t.Fatalf("expected suspicious install to require force")
	}
	installed, err := svc.Install(context.Background(), []string{"myhub/forms"}, lockPath, true)
	if err != nil {
		t.Fatalf("forced install failed: %v", err)
	}
	if len(installed) != 1 {
		t.Fatalf("expected one installed skill")
	}

	injectRes, err := svc.Inject(context.Background(), "openclaw", nil)
	if err != nil {
		t.Fatalf("inject failed: %v", err)
	}
	if len(injectRes.Injected) != 1 {
		t.Fatalf("expected one injected skill, got %d", len(injectRes.Injected))
	}

	report, err := svc.SyncRun(context.Background(), lockPath, true, false)
	if err != nil {
		t.Fatalf("sync failed: %v", err)
	}
	if len(report.UpdatedSources) == 0 {
		t.Fatalf("expected updated source report")
	}

	st, err := store.LoadState(svc.StateRoot)
	if err != nil {
		t.Fatalf("load state failed: %v", err)
	}
	if len(st.Injections) == 0 {
		t.Fatalf("expected injection state to persist")
	}
}
