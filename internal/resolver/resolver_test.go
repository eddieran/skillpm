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
		// GitHub bare repo URL (no tree/blob) → skill="." to scan entire repo
		{"https://github.com/someone/repo", ParsedRef{Source: "someone_repo", Skill: ".", IsURL: true, URL: "https://github.com/someone/repo.git", Branch: "main"}, false},
		// /blob/ URLs
		{"https://github.com/openclaw/skills/blob/main/skills/shashwatgtm/content-writing-thought-leadership/SKILL.md", ParsedRef{Source: "openclaw_skills", Skill: "skills/shashwatgtm/content-writing-thought-leadership", IsURL: true, URL: "https://github.com/openclaw/skills.git", Branch: "main"}, false},
		// /blob/ URL without file extension
		{"https://github.com/foo/bar/blob/dev/src/skills/my-skill", ParsedRef{Source: "foo_bar", Skill: "src/skills/my-skill", IsURL: true, URL: "https://github.com/foo/bar.git", Branch: "dev"}, false},
		// /tree/ URL with trailing SKILL.md (should be stripped)
		{"https://github.com/openai/skills/tree/main/skills/.curated/gh-fix-ci/SKILL.md", ParsedRef{Source: "openai_skills", Skill: "skills/.curated/gh-fix-ci", IsURL: true, URL: "https://github.com/openai/skills.git", Branch: "main"}, false},
		// --- Generic git host URLs ---
		// GitLab bare URL
		{"https://gitlab.okg.com/eddie-group/eddie-skills", ParsedRef{Source: "eddie-group_eddie-skills", Skill: ".", IsURL: true, URL: "https://gitlab.okg.com/eddie-group/eddie-skills.git", Branch: ""}, false},
		// GitLab with .git suffix
		{"https://gitlab.okg.com/eddie-group/eddie-skills.git", ParsedRef{Source: "eddie-group_eddie-skills", Skill: ".", IsURL: true, URL: "https://gitlab.okg.com/eddie-group/eddie-skills.git", Branch: ""}, false},
		// GitLab /-/tree/ with branch and skill path
		{"https://gitlab.com/org/repo/-/tree/develop/skills/my-skill", ParsedRef{Source: "org_repo", Skill: "skills/my-skill", IsURL: true, URL: "https://gitlab.com/org/repo.git", Branch: "develop"}, false},
		// GitLab /-/blob/ with SKILL.md (should strip file)
		{"https://gitlab.com/org/repo/-/blob/main/skills/review/SKILL.md", ParsedRef{Source: "org_repo", Skill: "skills/review", IsURL: true, URL: "https://gitlab.com/org/repo.git", Branch: "main"}, false},
		// Bitbucket bare URL
		{"https://bitbucket.org/team/skills-repo", ParsedRef{Source: "team_skills-repo", Skill: ".", IsURL: true, URL: "https://bitbucket.org/team/skills-repo.git", Branch: ""}, false},
		// Self-hosted Gitea
		{"https://git.internal.io/devops/shared-skills", ParsedRef{Source: "devops_shared-skills", Skill: ".", IsURL: true, URL: "https://git.internal.io/devops/shared-skills.git", Branch: ""}, false},
		// GitLab nested group
		{"https://gitlab.com/org/sub/repo/-/tree/main/my-skill", ParsedRef{Source: "org_sub_repo", Skill: "my-skill", IsURL: true, URL: "https://gitlab.com/org/sub/repo.git", Branch: "main"}, false},
		// Generic URL with standard /tree/ (non-GitLab host)
		{"https://gitea.example.com/team/repo/tree/dev/plugins", ParsedRef{Source: "team_repo", Skill: "plugins", IsURL: true, URL: "https://gitea.example.com/team/repo.git", Branch: "dev"}, false},
		// URL with only one path segment → error
		{"https://gitlab.com/only-one", ParsedRef{}, true},
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
