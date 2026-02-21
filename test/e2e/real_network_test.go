package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRealNetworkInstallAndInject(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping real network tests in short mode")
	}

	tests := []struct {
		name       string
		target     string
		expectSlug string
	}{
		{
			name:       "ClawHub Standard Slug",
			target:     "clawhub/slack",
			expectSlug: "clawhub/slack",
		},
		{
			name:       "ClawHub Custom URL",
			target:     "https://clawhub.ai/slack",
			expectSlug: "clawhub/slack",
		},
		{
			name:       "GitHub Pre-configured Source",
			target:     "anthropic/skill-creator",
			expectSlug: "anthropic/skill-creator",
		},
		{
			name:       "GitHub Shallow URL",
			target:     "https://github.com/anthropics/skills/tree/main/skills/skill-creator",
			expectSlug: "anthropics_skills/skills/skill-creator",
		},
		{
			name:       "GitHub Deep URL",
			target:     "https://github.com/dgunning/edgartools/tree/main/edgar/ai/skills/core",
			expectSlug: "dgunning_edgartools/edgar/ai/skills/core",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			bin, env := buildCLI(t, home)
			cfgPath := filepath.Join(home, ".skillpm", "config.toml")

			// 1. Install the target
			// We use --force to bypass any 'untrusted'/'community' interactive prompts
			installOut := runCLI(t, bin, env, "--config", cfgPath, "install", tc.target, "--force")
			if !strings.Contains(installOut, tc.expectSlug) {
				t.Fatalf("expected output to contain installed slug %q, got:\n%s", tc.expectSlug, installOut)
			}

			// 2. Inject to openclaw
			injectOut := runCLI(t, bin, env, "--config", cfgPath, "inject", "--agent", "openclaw", tc.expectSlug)
			if !strings.Contains(injectOut, "injected 1 skill(s)") {
				t.Fatalf("expected output to indicate 1 skill injected, got:\n%s", injectOut)
			}

			// 3. Verify injected.toml exists and contains the skill
			injectedTomlPath := filepath.Join(home, "openclaw-state", "skillpm", "injected.toml")
			content, err := os.ReadFile(injectedTomlPath)
			if err != nil {
				t.Fatalf("failed to read injected.toml: %v", err)
			}

			// Basic containment check to ensure physical fetch succeeded and wrote configs
			if !strings.Contains(string(content), tc.expectSlug) && !strings.Contains(string(content), "SKILL.md") {
				t.Fatalf("injected.toml seems invalid or missing skill data:\n%s", string(content))
			}
		})
	}
}
