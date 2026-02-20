package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"skillpm/internal/config"
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

func TestServiceInstallInjectGitSourceEndToEnd(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available on PATH")
	}

	home := t.TempDir()
	t.Setenv("HOME", home)

	// Create claude adapter directory so it's detected and usable
	claudeDir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatalf("mkdir claude dir failed: %v", err)
	}

	repoURL := setupBareRepo(t, map[string]map[string]string{
		"docx": {
			"SKILL.md":          "# docx\nDocument creation skill",
			"tools/helper.sh":   "#!/bin/bash\necho helper",
			"references/ref.md": "Reference content",
		},
	})

	svc, err := New(Options{ConfigPath: filepath.Join(home, ".skillpm", "config.toml")})
	if err != nil {
		t.Fatalf("new service failed: %v", err)
	}
	svc.Config.Sources = []config.SourceConfig{{
		Name:      "test",
		Kind:      "git",
		URL:       repoURL,
		Branch:    "main",
		ScanPaths: []string{"skills"},
		TrustTier: "review",
	}}
	svc.Config.Adapters = []config.AdapterConfig{{Name: "claude", Enabled: true, Scope: "global"}}
	if err := svc.SaveConfig(); err != nil {
		t.Fatalf("save config failed: %v", err)
	}
	// Reload runtime to pick up adapter config
	svc, err = New(Options{ConfigPath: svc.ConfigPath})
	if err != nil {
		t.Fatalf("reload service failed: %v", err)
	}

	ctx := context.Background()

	// Step 1: Source update
	updated, err := svc.SourceUpdate(ctx, "test")
	if err != nil {
		t.Fatalf("source update failed: %v", err)
	}
	if len(updated) != 1 || updated[0].Source.Name != "test" {
		t.Fatalf("expected update for 'test', got %+v", updated)
	}

	// Step 2: Search
	results, err := svc.Search(ctx, "test", "docx")
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 search result, got %d", len(results))
	}
	if results[0].Name != "docx" {
		t.Fatalf("expected docx, got %q", results[0].Name)
	}

	// Step 3: Install
	lockPath := filepath.Join(t.TempDir(), "skills.lock")
	installed, err := svc.Install(ctx, []string{"test/docx"}, lockPath, false)
	if err != nil {
		t.Fatalf("install failed: %v", err)
	}
	if len(installed) != 1 {
		t.Fatalf("expected 1 installed, got %d", len(installed))
	}

	// Verify SKILL.md has real content
	entries, err := os.ReadDir(filepath.Join(svc.StateRoot, "installed"))
	if err != nil {
		t.Fatalf("read installed dir failed: %v", err)
	}
	found := false
	for _, e := range entries {
		skillMdPath := filepath.Join(svc.StateRoot, "installed", e.Name(), "SKILL.md")
		data, err := os.ReadFile(skillMdPath)
		if err != nil {
			continue
		}
		if strings.Contains(string(data), "Document creation skill") {
			found = true
			// Check ancillary files
			helperPath := filepath.Join(svc.StateRoot, "installed", e.Name(), "tools", "helper.sh")
			helperData, err := os.ReadFile(helperPath)
			if err != nil {
				t.Fatalf("read helper.sh failed: %v", err)
			}
			if !strings.Contains(string(helperData), "echo helper") {
				t.Fatalf("unexpected helper.sh content: %q", string(helperData))
			}
			refPath := filepath.Join(svc.StateRoot, "installed", e.Name(), "references", "ref.md")
			refData, err := os.ReadFile(refPath)
			if err != nil {
				t.Fatalf("read ref.md failed: %v", err)
			}
			if string(refData) != "Reference content" {
				t.Fatalf("unexpected ref.md content: %q", string(refData))
			}
			break
		}
	}
	if !found {
		t.Fatalf("installed SKILL.md does not contain real content")
	}

	// Step 4: Inject into claude adapter
	injResult, err := svc.Inject(ctx, "claude", nil)
	if err != nil {
		t.Fatalf("inject failed: %v", err)
	}
	if len(injResult.Injected) != 1 {
		t.Fatalf("expected 1 injected, got %d", len(injResult.Injected))
	}

	// Verify content in agent skills dir
	// claude adapter places skills at ~/.claude/skills/{name}/
	agentSkillDir := filepath.Join(claudeDir, "skills", "docx")
	skillMdData, err := os.ReadFile(filepath.Join(agentSkillDir, "SKILL.md"))
	if err != nil {
		t.Fatalf("read injected SKILL.md failed: %v", err)
	}
	if !strings.Contains(string(skillMdData), "Document creation skill") {
		t.Fatalf("injected SKILL.md missing real content: %q", string(skillMdData))
	}

	// Verify ancillary files are also injected
	helperInAgent := filepath.Join(agentSkillDir, "tools", "helper.sh")
	helperData, err := os.ReadFile(helperInAgent)
	if err != nil {
		t.Fatalf("read injected tools/helper.sh failed: %v", err)
	}
	if !strings.Contains(string(helperData), "echo helper") {
		t.Fatalf("injected helper.sh missing content: %q", string(helperData))
	}
}
