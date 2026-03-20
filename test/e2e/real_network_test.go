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
	if os.Getenv("CI") != "" {
		t.Skip("skipping real network tests in CI environment to avoid rate limits")
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
			installOut, err := runCLIRaw(t, bin, env, nil, "--config", cfgPath, "install", tc.target, "--force")
			if err != nil {
				if isUpstreamRateLimit(installOut) {
					t.Skipf("upstream rate-limited live network validation: %s", strings.TrimSpace(installOut))
				}
				t.Fatalf("command failed: %v\nargs=%v\noutput=%s", err, []string{"--config", cfgPath, "install", tc.target, "--force"}, installOut)
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

func isUpstreamRateLimit(output string) bool {
	lower := strings.ToLower(output)
	return strings.Contains(lower, "status 429") || strings.Contains(lower, "rate limit")
}
