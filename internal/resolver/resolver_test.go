package resolver

import (
	"context"
	"net/http"
	"testing"

	"skillpm/internal/config"
	"skillpm/internal/source"
	"skillpm/internal/store"
)

func TestParseRef(t *testing.T) {
	parsed, err := ParseRef("anthropic/pdf@1.2.3")
	if err != nil {
		t.Fatalf("expected parse success: %v", err)
	}
	if parsed.Source != "anthropic" || parsed.Skill != "pdf" || parsed.Constraint != "1.2.3" {
		t.Fatalf("unexpected parse result: %+v", parsed)
	}
	if _, err := ParseRef("badref"); err == nil {
		t.Fatalf("expected parse error for invalid ref")
	}
}

func TestResolveManyUsesLockVersionWhenConstraintMissing(t *testing.T) {
	cfg := config.DefaultConfig()
	svc := &Service{Sources: source.NewManager(http.DefaultClient, t.TempDir())}
	lock := store.Lockfile{
		Version: store.LockVersion,
		Skills: []store.LockSkill{{
			SkillRef:        "anthropic/pdf",
			ResolvedVersion: "9.9.9",
			Checksum:        "sha256:x",
			SourceRef:       "git://example",
		}},
	}
	resolved, err := svc.ResolveMany(context.Background(), cfg, []string{"anthropic/pdf"}, lock)
	if err != nil {
		t.Fatalf("resolve failed: %v", err)
	}
	if len(resolved) != 1 {
		t.Fatalf("expected one resolved skill")
	}
	if resolved[0].ResolvedVersion != "9.9.9" {
		t.Fatalf("expected locked version, got %q", resolved[0].ResolvedVersion)
	}
}
