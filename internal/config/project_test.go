package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindProjectRoot(t *testing.T) {
	t.Run("found at current dir", func(t *testing.T) {
		tmp := t.TempDir()
		manifestDir := filepath.Join(tmp, ".skillpm")
		if err := os.MkdirAll(manifestDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(manifestDir, "skills.toml"), []byte("version = 1\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		root, found := FindProjectRoot(tmp)
		if !found {
			t.Fatal("expected to find project root")
		}
		if root != tmp {
			t.Fatalf("got root %q, want %q", root, tmp)
		}
	})

	t.Run("found in parent dir", func(t *testing.T) {
		tmp := t.TempDir()
		manifestDir := filepath.Join(tmp, ".skillpm")
		if err := os.MkdirAll(manifestDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(manifestDir, "skills.toml"), []byte("version = 1\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		subDir := filepath.Join(tmp, "src", "deep", "nested")
		if err := os.MkdirAll(subDir, 0o755); err != nil {
			t.Fatal(err)
		}
		root, found := FindProjectRoot(subDir)
		if !found {
			t.Fatal("expected to find project root in parent")
		}
		if root != tmp {
			t.Fatalf("got root %q, want %q", root, tmp)
		}
	})

	t.Run("not found", func(t *testing.T) {
		tmp := t.TempDir()
		_, found := FindProjectRoot(tmp)
		if found {
			t.Fatal("expected not to find project root")
		}
	})

	t.Run("nested projects innermost wins", func(t *testing.T) {
		tmp := t.TempDir()
		// outer project
		outerManifest := filepath.Join(tmp, ".skillpm")
		if err := os.MkdirAll(outerManifest, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(outerManifest, "skills.toml"), []byte("version = 1\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		// inner project
		innerDir := filepath.Join(tmp, "sub", "inner")
		innerManifest := filepath.Join(innerDir, ".skillpm")
		if err := os.MkdirAll(innerManifest, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(innerManifest, "skills.toml"), []byte("version = 1\n"), 0o644); err != nil {
			t.Fatal(err)
		}

		root, found := FindProjectRoot(innerDir)
		if !found {
			t.Fatal("expected to find project root")
		}
		if root != innerDir {
			t.Fatalf("got root %q, want innermost %q", root, innerDir)
		}
	})
}

func TestResolveScope(t *testing.T) {
	t.Run("explicit global", func(t *testing.T) {
		scope, root, err := ResolveScope("global", "/tmp")
		if err != nil {
			t.Fatal(err)
		}
		if scope != ScopeGlobal {
			t.Fatalf("got scope %q, want global", scope)
		}
		if root != "" {
			t.Fatalf("got root %q, want empty", root)
		}
	})

	t.Run("explicit project with manifest", func(t *testing.T) {
		tmp := t.TempDir()
		manifestDir := filepath.Join(tmp, ".skillpm")
		if err := os.MkdirAll(manifestDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(manifestDir, "skills.toml"), []byte("version = 1\n"), 0o644); err != nil {
			t.Fatal(err)
		}

		scope, root, err := ResolveScope("project", tmp)
		if err != nil {
			t.Fatal(err)
		}
		if scope != ScopeProject {
			t.Fatalf("got scope %q, want project", scope)
		}
		if root != tmp {
			t.Fatalf("got root %q, want %q", root, tmp)
		}
	})

	t.Run("explicit project without manifest errors", func(t *testing.T) {
		tmp := t.TempDir()
		_, _, err := ResolveScope("project", tmp)
		if err == nil {
			t.Fatal("expected error for --scope project with no manifest")
		}
	})

	t.Run("auto-detect project", func(t *testing.T) {
		tmp := t.TempDir()
		manifestDir := filepath.Join(tmp, ".skillpm")
		if err := os.MkdirAll(manifestDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(manifestDir, "skills.toml"), []byte("version = 1\n"), 0o644); err != nil {
			t.Fatal(err)
		}

		scope, root, err := ResolveScope("", tmp)
		if err != nil {
			t.Fatal(err)
		}
		if scope != ScopeProject {
			t.Fatalf("got scope %q, want project (auto-detected)", scope)
		}
		if root != tmp {
			t.Fatalf("got root %q, want %q", root, tmp)
		}
	})

	t.Run("auto-detect global", func(t *testing.T) {
		tmp := t.TempDir()
		scope, root, err := ResolveScope("", tmp)
		if err != nil {
			t.Fatal(err)
		}
		if scope != ScopeGlobal {
			t.Fatalf("got scope %q, want global (auto-detected)", scope)
		}
		if root != "" {
			t.Fatalf("got root %q, want empty", root)
		}
	})

	t.Run("invalid scope", func(t *testing.T) {
		_, _, err := ResolveScope("workspace", "/tmp")
		if err == nil {
			t.Fatal("expected error for invalid scope")
		}
	})
}

func TestLoadSaveProjectManifest(t *testing.T) {
	t.Run("round-trip", func(t *testing.T) {
		tmp := t.TempDir()
		original := ProjectManifest{
			Version: 1,
			Sources: []SourceConfig{
				{Name: "team", Kind: "git", URL: "https://github.com/team/skills.git", Branch: "main", ScanPaths: []string{"skills"}, TrustTier: "trusted"},
			},
			Skills: []ProjectSkillEntry{
				{Ref: "team/code-review", Constraint: "^1.0.0"},
				{Ref: "team/lint-rules", Constraint: "2.1.0"},
			},
			Adapters: []AdapterConfig{
				{Name: "claude", Enabled: true, Scope: "project"},
			},
		}
		if err := SaveProjectManifest(tmp, original); err != nil {
			t.Fatal(err)
		}
		loaded, err := LoadProjectManifest(tmp)
		if err != nil {
			t.Fatal(err)
		}
		if loaded.Version != original.Version {
			t.Fatalf("version mismatch: got %d, want %d", loaded.Version, original.Version)
		}
		if len(loaded.Skills) != len(original.Skills) {
			t.Fatalf("skills count mismatch: got %d, want %d", len(loaded.Skills), len(original.Skills))
		}
		for i := range original.Skills {
			if loaded.Skills[i].Ref != original.Skills[i].Ref {
				t.Fatalf("skill[%d] ref mismatch: got %q, want %q", i, loaded.Skills[i].Ref, original.Skills[i].Ref)
			}
			if loaded.Skills[i].Constraint != original.Skills[i].Constraint {
				t.Fatalf("skill[%d] constraint mismatch: got %q, want %q", i, loaded.Skills[i].Constraint, original.Skills[i].Constraint)
			}
		}
		if len(loaded.Sources) != len(original.Sources) {
			t.Fatalf("sources count mismatch: got %d, want %d", len(loaded.Sources), len(original.Sources))
		}
		if len(loaded.Adapters) != len(original.Adapters) {
			t.Fatalf("adapters count mismatch: got %d, want %d", len(loaded.Adapters), len(original.Adapters))
		}
	})

	t.Run("missing file", func(t *testing.T) {
		tmp := t.TempDir()
		_, err := LoadProjectManifest(tmp)
		if err == nil {
			t.Fatal("expected error for missing file")
		}
	})

	t.Run("corrupt TOML", func(t *testing.T) {
		tmp := t.TempDir()
		dir := filepath.Join(tmp, ".skillpm")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "skills.toml"), []byte("{{invalid toml"), 0o644); err != nil {
			t.Fatal(err)
		}
		_, err := LoadProjectManifest(tmp)
		if err == nil {
			t.Fatal("expected error for corrupt TOML")
		}
	})

	t.Run("empty skills defaults to empty slice", func(t *testing.T) {
		tmp := t.TempDir()
		dir := filepath.Join(tmp, ".skillpm")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "skills.toml"), []byte("version = 1\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		loaded, err := LoadProjectManifest(tmp)
		if err != nil {
			t.Fatal(err)
		}
		if loaded.Skills == nil {
			t.Fatal("expected non-nil empty skills slice")
		}
		if len(loaded.Skills) != 0 {
			t.Fatalf("expected 0 skills, got %d", len(loaded.Skills))
		}
	})
}

func TestMergedSources(t *testing.T) {
	global := Config{
		Sources: []SourceConfig{
			{Name: "anthropic", Kind: "git", URL: "https://github.com/anthropics/skills.git", TrustTier: "review"},
			{Name: "clawhub", Kind: "clawhub", Site: "https://clawhub.ai/", TrustTier: "review"},
		},
	}

	t.Run("no project sources", func(t *testing.T) {
		project := ProjectManifest{Skills: []ProjectSkillEntry{}}
		merged := MergedSources(global, project)
		if len(merged) != 2 {
			t.Fatalf("expected 2 sources, got %d", len(merged))
		}
	})

	t.Run("project adds new source", func(t *testing.T) {
		project := ProjectManifest{
			Sources: []SourceConfig{
				{Name: "team", Kind: "git", URL: "https://github.com/team/skills.git", TrustTier: "trusted"},
			},
			Skills: []ProjectSkillEntry{},
		}
		merged := MergedSources(global, project)
		if len(merged) != 3 {
			t.Fatalf("expected 3 sources, got %d", len(merged))
		}
		if merged[2].Name != "team" {
			t.Fatalf("expected team as third source, got %q", merged[2].Name)
		}
	})

	t.Run("project overrides global source", func(t *testing.T) {
		project := ProjectManifest{
			Sources: []SourceConfig{
				{Name: "anthropic", Kind: "git", URL: "https://github.com/team/fork.git", TrustTier: "trusted"},
			},
			Skills: []ProjectSkillEntry{},
		}
		merged := MergedSources(global, project)
		if len(merged) != 2 {
			t.Fatalf("expected 2 sources (override, not add), got %d", len(merged))
		}
		if merged[0].URL != "https://github.com/team/fork.git" {
			t.Fatalf("expected overridden URL, got %q", merged[0].URL)
		}
		if merged[0].TrustTier != "trusted" {
			t.Fatalf("expected overridden trust tier, got %q", merged[0].TrustTier)
		}
	})
}

func TestMergedAdapters(t *testing.T) {
	global := Config{
		Adapters: []AdapterConfig{
			{Name: "codex", Enabled: true, Scope: "global"},
			{Name: "openclaw", Enabled: true, Scope: "global"},
		},
	}

	t.Run("no project adapters", func(t *testing.T) {
		project := ProjectManifest{Skills: []ProjectSkillEntry{}}
		merged := MergedAdapters(global, project)
		if len(merged) != 2 {
			t.Fatalf("expected 2 adapters, got %d", len(merged))
		}
	})

	t.Run("project adds adapter", func(t *testing.T) {
		project := ProjectManifest{
			Adapters: []AdapterConfig{
				{Name: "claude", Enabled: true, Scope: "project"},
			},
			Skills: []ProjectSkillEntry{},
		}
		merged := MergedAdapters(global, project)
		if len(merged) != 3 {
			t.Fatalf("expected 3 adapters, got %d", len(merged))
		}
	})

	t.Run("project overrides adapter", func(t *testing.T) {
		project := ProjectManifest{
			Adapters: []AdapterConfig{
				{Name: "codex", Enabled: false, Scope: "project"},
			},
			Skills: []ProjectSkillEntry{},
		}
		merged := MergedAdapters(global, project)
		if len(merged) != 2 {
			t.Fatalf("expected 2 adapters, got %d", len(merged))
		}
		if merged[0].Enabled {
			t.Fatal("expected codex to be disabled by project override")
		}
	})
}

func TestUpsertManifestSkill(t *testing.T) {
	t.Run("add new", func(t *testing.T) {
		m := DefaultProjectManifest()
		UpsertManifestSkill(&m, ProjectSkillEntry{Ref: "team/review", Constraint: "^1.0.0"})
		if len(m.Skills) != 1 {
			t.Fatalf("expected 1 skill, got %d", len(m.Skills))
		}
		if m.Skills[0].Ref != "team/review" {
			t.Fatalf("got ref %q, want team/review", m.Skills[0].Ref)
		}
	})

	t.Run("update existing", func(t *testing.T) {
		m := ProjectManifest{
			Version: 1,
			Skills:  []ProjectSkillEntry{{Ref: "team/review", Constraint: "^1.0.0"}},
		}
		UpsertManifestSkill(&m, ProjectSkillEntry{Ref: "team/review", Constraint: "^2.0.0"})
		if len(m.Skills) != 1 {
			t.Fatalf("expected 1 skill (updated, not duplicated), got %d", len(m.Skills))
		}
		if m.Skills[0].Constraint != "^2.0.0" {
			t.Fatalf("got constraint %q, want ^2.0.0", m.Skills[0].Constraint)
		}
	})
}

func TestRemoveManifestSkill(t *testing.T) {
	t.Run("remove existing", func(t *testing.T) {
		m := ProjectManifest{
			Version: 1,
			Skills: []ProjectSkillEntry{
				{Ref: "team/review", Constraint: "^1.0.0"},
				{Ref: "team/lint", Constraint: "2.0.0"},
			},
		}
		removed := RemoveManifestSkill(&m, "team/review")
		if !removed {
			t.Fatal("expected true for existing skill removal")
		}
		if len(m.Skills) != 1 {
			t.Fatalf("expected 1 skill remaining, got %d", len(m.Skills))
		}
		if m.Skills[0].Ref != "team/lint" {
			t.Fatalf("expected team/lint to remain, got %q", m.Skills[0].Ref)
		}
	})

	t.Run("remove non-existent", func(t *testing.T) {
		m := ProjectManifest{
			Version: 1,
			Skills:  []ProjectSkillEntry{{Ref: "team/review", Constraint: "^1.0.0"}},
		}
		removed := RemoveManifestSkill(&m, "team/nonexistent")
		if removed {
			t.Fatal("expected false for non-existent skill removal")
		}
		if len(m.Skills) != 1 {
			t.Fatal("skills should be unchanged")
		}
	})
}

func TestDefaultProjectManifest(t *testing.T) {
	m := DefaultProjectManifest()
	if m.Version != SchemaVersion {
		t.Fatalf("got version %d, want %d", m.Version, SchemaVersion)
	}
	if m.Skills == nil {
		t.Fatal("expected non-nil skills slice")
	}
	if len(m.Skills) != 0 {
		t.Fatalf("expected 0 skills, got %d", len(m.Skills))
	}
}

func TestProjectPaths(t *testing.T) {
	root := "/home/user/myproject"
	if got := ProjectManifestPath(root); got != filepath.Join(root, ".skillpm", "skills.toml") {
		t.Fatalf("manifest path: got %q", got)
	}
	if got := ProjectLockPath(root); got != filepath.Join(root, ".skillpm", "skills.lock") {
		t.Fatalf("lock path: got %q", got)
	}
	if got := ProjectStateRoot(root); got != filepath.Join(root, ".skillpm") {
		t.Fatalf("state root: got %q", got)
	}
}

func TestEnsureProjectLayout(t *testing.T) {
	tmp := t.TempDir()
	if err := EnsureProjectLayout(tmp); err != nil {
		t.Fatal(err)
	}
	for _, sub := range []string{".skillpm", ".skillpm/installed", ".skillpm/staging", ".skillpm/snapshots"} {
		p := filepath.Join(tmp, sub)
		info, err := os.Stat(p)
		if err != nil {
			t.Fatalf("expected directory %s to exist: %v", sub, err)
		}
		if !info.IsDir() {
			t.Fatalf("expected %s to be a directory", sub)
		}
	}
}

func TestInitProject(t *testing.T) {
	t.Run("creates manifest", func(t *testing.T) {
		tmp := t.TempDir()
		path, err := InitProject(tmp)
		if err != nil {
			t.Fatal(err)
		}
		if path != ProjectManifestPath(tmp) {
			t.Fatalf("got path %q, want %q", path, ProjectManifestPath(tmp))
		}
		// Verify manifest is loadable
		m, err := LoadProjectManifest(tmp)
		if err != nil {
			t.Fatal(err)
		}
		if m.Version != SchemaVersion {
			t.Fatalf("got version %d, want %d", m.Version, SchemaVersion)
		}
	})

	t.Run("errors if already initialized", func(t *testing.T) {
		tmp := t.TempDir()
		if _, err := InitProject(tmp); err != nil {
			t.Fatal(err)
		}
		_, err := InitProject(tmp)
		if err == nil {
			t.Fatal("expected error for double init")
		}
	})
}
