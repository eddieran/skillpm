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
	tests := []struct {
		in      string
		want    ParsedRef
		wantErr bool
	}{
		{"anthropic/pdf@1.2.3", ParsedRef{Source: "anthropic", Skill: "pdf", Constraint: "1.2.3"}, false},
		{"anthropic/pdf", ParsedRef{Source: "anthropic", Skill: "pdf", Constraint: ""}, false},
		{"https://clawhub.ai/steipete/slack", ParsedRef{Source: "clawhub", Skill: "steipete/slack", Constraint: ""}, false},
		{"https://clawhub.ai/steipete/slack@1.0.0", ParsedRef{Source: "clawhub", Skill: "steipete/slack", Constraint: "1.0.0"}, false},
		{"https://github.com/dgunning/edgartools/tree/main/edgar/ai/skills", ParsedRef{Source: "dgunning_edgartools", Skill: "edgar/ai/skills", IsURL: true, URL: "https://github.com/dgunning/edgartools.git", Branch: "main"}, false},
		{"https://github.com/jeremylongshore/skills/tree/v2/plugins", ParsedRef{Source: "jeremylongshore_skills", Skill: "plugins", IsURL: true, URL: "https://github.com/jeremylongshore/skills.git", Branch: "v2"}, false},
		{"https://github.com/someone/repo", ParsedRef{Source: "someone_repo", Skill: "repo", IsURL: true, URL: "https://github.com/someone/repo.git", Branch: "main"}, false},
		{"badref", ParsedRef{}, true},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := ParseRef(tt.in)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseRef() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("ParseRef() got = %+v, want %+v", got, tt.want)
			}
		})
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
