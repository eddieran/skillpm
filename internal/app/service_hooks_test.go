package app

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"skillpm/internal/config"
)

// newHookTestService creates a Service with a git source, claude adapter, and
// the given hooks config. The source contains a single "hook-skill" skill.
func newHookTestService(t *testing.T, hooksCfg config.HooksConfig) *Service {
	t.Helper()

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
		"hook-skill": {"SKILL.md": "# hook-skill\nA test skill for hooks"},
	})

	cfgPath := filepath.Join(home, ".skillpm", "config.toml")
	svc, err := New(Options{ConfigPath: cfgPath})
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
	svc.Config.Hooks = hooksCfg
	if err := svc.SaveConfig(); err != nil {
		t.Fatalf("save config failed: %v", err)
	}
	// Reload to pick up adapter config and hooks config in all subsystems
	svc, err = New(Options{ConfigPath: cfgPath})
	if err != nil {
		t.Fatalf("reload service failed: %v", err)
	}
	return svc
}

func TestHookPreInstallEnvVars(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "pre_install_env")
	svc := newHookTestService(t, config.HooksConfig{
		PreInstall: []string{
			fmt.Sprintf("echo \"phase=$SKILLPM_PHASE ref=$SKILLPM_SKILL_REF\" >> %s", marker),
		},
	})

	lockPath := filepath.Join(t.TempDir(), "skills.lock")
	_, err := svc.Install(context.Background(), []string{"test/hook-skill"}, lockPath, false)
	if err != nil {
		t.Fatalf("install failed: %v", err)
	}

	data, err := os.ReadFile(marker)
	if err != nil {
		t.Fatalf("pre_install hook marker not created: %v", err)
	}
	content := strings.TrimSpace(string(data))
	if !strings.Contains(content, "phase=pre_install") {
		t.Errorf("expected SKILLPM_PHASE=pre_install in output, got %q", content)
	}
	if !strings.Contains(content, "test/hook-skill") {
		t.Errorf("expected SKILLPM_SKILL_REF containing test/hook-skill, got %q", content)
	}
}

func TestHookPostInstallRunsAfterSuccess(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "post_install_marker")
	svc := newHookTestService(t, config.HooksConfig{
		PostInstall: []string{fmt.Sprintf("touch %s", marker)},
	})

	lockPath := filepath.Join(t.TempDir(), "skills.lock")
	_, err := svc.Install(context.Background(), []string{"test/hook-skill"}, lockPath, false)
	if err != nil {
		t.Fatalf("install failed: %v", err)
	}

	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("post_install hook marker not created: %v", err)
	}
}

func TestHookPreInstallFailureAbortsInstall(t *testing.T) {
	svc := newHookTestService(t, config.HooksConfig{
		PreInstall: []string{"exit 1"},
	})

	lockPath := filepath.Join(t.TempDir(), "skills.lock")
	_, err := svc.Install(context.Background(), []string{"test/hook-skill"}, lockPath, false)
	if err == nil {
		t.Fatalf("expected install to fail due to pre_install hook failure")
	}

	// Verify no skills were installed
	installed, listErr := svc.ListInstalled()
	if listErr != nil {
		t.Fatalf("list installed failed: %v", listErr)
	}
	if len(installed) != 0 {
		t.Fatalf("expected no installed skills after hook abort, got %d", len(installed))
	}
}

func TestHookPostInstallFailureDoesNotAbort(t *testing.T) {
	svc := newHookTestService(t, config.HooksConfig{
		PostInstall: []string{"exit 1"},
	})

	lockPath := filepath.Join(t.TempDir(), "skills.lock")
	installed, err := svc.Install(context.Background(), []string{"test/hook-skill"}, lockPath, false)
	if err != nil {
		t.Fatalf("install should succeed despite post_install hook failure: %v", err)
	}
	if len(installed) != 1 {
		t.Fatalf("expected 1 installed skill, got %d", len(installed))
	}
}

func TestHookInjectPreAndPost(t *testing.T) {
	preMarker := filepath.Join(t.TempDir(), "pre_inject_marker")
	postMarker := filepath.Join(t.TempDir(), "post_inject_marker")
	svc := newHookTestService(t, config.HooksConfig{
		PreInject:  []string{fmt.Sprintf("touch %s", preMarker)},
		PostInject: []string{fmt.Sprintf("touch %s", postMarker)},
	})

	ctx := context.Background()
	lockPath := filepath.Join(t.TempDir(), "skills.lock")
	_, err := svc.Install(ctx, []string{"test/hook-skill"}, lockPath, false)
	if err != nil {
		t.Fatalf("install failed: %v", err)
	}

	_, err = svc.Inject(ctx, "claude", nil)
	if err != nil {
		t.Fatalf("inject failed: %v", err)
	}

	if _, err := os.Stat(preMarker); err != nil {
		t.Fatalf("pre_inject hook marker not created: %v", err)
	}
	if _, err := os.Stat(postMarker); err != nil {
		t.Fatalf("post_inject hook marker not created: %v", err)
	}
}

func TestHookRemovePreAndPost(t *testing.T) {
	preMarker := filepath.Join(t.TempDir(), "pre_remove_marker")
	postMarker := filepath.Join(t.TempDir(), "post_remove_marker")
	svc := newHookTestService(t, config.HooksConfig{
		PreRemove:  []string{fmt.Sprintf("touch %s", preMarker)},
		PostRemove: []string{fmt.Sprintf("touch %s", postMarker)},
	})

	ctx := context.Background()
	lockPath := filepath.Join(t.TempDir(), "skills.lock")
	_, err := svc.Install(ctx, []string{"test/hook-skill"}, lockPath, false)
	if err != nil {
		t.Fatalf("install failed: %v", err)
	}

	_, err = svc.Inject(ctx, "claude", nil)
	if err != nil {
		t.Fatalf("inject failed: %v", err)
	}

	_, err = svc.RemoveInjected(ctx, "claude", []string{"test/hook-skill"})
	if err != nil {
		t.Fatalf("remove injected failed: %v", err)
	}

	if _, err := os.Stat(preMarker); err != nil {
		t.Fatalf("pre_remove hook marker not created: %v", err)
	}
	if _, err := os.Stat(postMarker); err != nil {
		t.Fatalf("post_remove hook marker not created: %v", err)
	}
}

func TestHookEmptyConfigNoOp(t *testing.T) {
	svc := newHookTestService(t, config.HooksConfig{})

	lockPath := filepath.Join(t.TempDir(), "skills.lock")
	installed, err := svc.Install(context.Background(), []string{"test/hook-skill"}, lockPath, false)
	if err != nil {
		t.Fatalf("install with empty hooks should not fail: %v", err)
	}
	if len(installed) != 1 {
		t.Fatalf("expected 1 installed skill, got %d", len(installed))
	}
}
