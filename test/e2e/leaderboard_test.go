package e2e

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestLeaderboardDefaultOutput(t *testing.T) {
	home := t.TempDir()
	bin, env := buildCLI(t, home)
	cfgPath := home + "/.skillpm/config.toml"

	out := runCLI(t, bin, env, "--config", cfgPath, "leaderboard")
	assertContains(t, out, "Skill Leaderboard")
	assertContains(t, out, "SKILL")
	assertContains(t, out, "DLs")
	assertContains(t, out, "INSTALL COMMAND")
	assertContains(t, out, "code-review")
	assertContains(t, out, "Showing 15 entries")
}

func TestLeaderboardJSONOutput(t *testing.T) {
	home := t.TempDir()
	bin, env := buildCLI(t, home)
	cfgPath := home + "/.skillpm/config.toml"

	out := runCLI(t, bin, env, "--config", cfgPath, "leaderboard", "--json", "--limit", "3")
	var entries []map[string]any
	if err := json.Unmarshal([]byte(out), &entries); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, out)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
	first := entries[0]
	for _, key := range []string{"rank", "slug", "name", "category", "downloads", "rating", "source", "description"} {
		if _, ok := first[key]; !ok {
			t.Fatalf("missing key %q in JSON entry", key)
		}
	}
}

func TestLeaderboardCategoryFilter(t *testing.T) {
	home := t.TempDir()
	bin, env := buildCLI(t, home)
	cfgPath := home + "/.skillpm/config.toml"

	out := runCLI(t, bin, env, "--config", cfgPath, "leaderboard", "--category", "security")
	assertContains(t, out, "SECURITY")
	assertContains(t, out, "secret-scanner")
	if strings.Contains(out, "code-review") {
		t.Fatal("code-review (tool) should not appear in security filter")
	}
}

func TestLeaderboardLimitFlag(t *testing.T) {
	home := t.TempDir()
	bin, env := buildCLI(t, home)
	cfgPath := home + "/.skillpm/config.toml"

	out := runCLI(t, bin, env, "--config", cfgPath, "leaderboard", "--limit", "5")
	assertContains(t, out, "Showing 5 entries")
}

func TestLeaderboardInvalidCategory(t *testing.T) {
	home := t.TempDir()
	bin, env := buildCLI(t, home)
	cfgPath := home + "/.skillpm/config.toml"

	out := runCLIExpectFail(t, bin, env, "--config", cfgPath, "leaderboard", "--category", "bogus")
	assertContains(t, out, "LB_CATEGORY")
}
