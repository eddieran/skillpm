package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const realNetworkOptInEnv = "SKILLPM_E2E_REAL_NETWORK"

func TestRealNetworkInstallAndInject(t *testing.T) {
	requireRealNetworkOptIn(t)

	tests := []struct {
		name       string
		target     string
		expectSlug string
		skipOn429  bool
	}{
		{
			name:       "ClawHub Standard Slug",
			target:     "clawhub/slack",
			expectSlug: "clawhub/slack",
			skipOn429:  true,
		},
		{
			name:       "ClawHub Custom URL",
			target:     "https://clawhub.ai/slack",
			expectSlug: "clawhub/slack",
			skipOn429:  true,
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
			installArgs := []string{"--config", cfgPath, "install", tc.target, "--force"}
			installOut, err := runCLIResult(t, bin, env, nil, installArgs...)
			if err != nil {
				if tc.skipOn429 && strings.Contains(installOut, "status 429") {
					t.Skipf("skipping live ClawHub smoke because the registry returned HTTP 429: %s", strings.TrimSpace(installOut))
				}
				t.Fatalf("command failed: %s\nargs=%v\noutput=%s", err, installArgs, installOut)
			}
			if !strings.Contains(installOut, tc.expectSlug) {
				t.Fatalf("expected output to contain installed slug %q, got:\n%s", tc.expectSlug, installOut)
			}

			// 2. Inject to openclaw
			injectOut := runCLI(t, bin, env, "--config", cfgPath, "inject", "--agent", "openclaw", tc.expectSlug)
			if !strings.Contains(injectOut, "injected into openclaw:") {
				t.Fatalf("expected output to contain 'injected into openclaw:', got:\n%s", injectOut)
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

func requireRealNetworkOptIn(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping real network tests in short mode")
	}
	if os.Getenv(realNetworkOptInEnv) != "1" {
		t.Skipf("skipping real network tests by default; set %s=1 to opt in", realNetworkOptInEnv)
	}
}
