package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// paths.go
// ---------------------------------------------------------------------------

func TestExpandPath(t *testing.T) {
	t.Run("empty path returns error", func(t *testing.T) {
		_, err := ExpandPath("")
		if err == nil {
			t.Fatal("expected error for empty path")
		}
	})

	t.Run("tilde only expands to home", func(t *testing.T) {
		got, err := ExpandPath("~")
		if err != nil {
			t.Fatal(err)
		}
		home, _ := os.UserHomeDir()
		if got != home {
			t.Fatalf("got %q, want %q", got, home)
		}
	})

	t.Run("tilde slash prefix expands", func(t *testing.T) {
		got, err := ExpandPath("~/foo/bar")
		if err != nil {
			t.Fatal(err)
		}
		home, _ := os.UserHomeDir()
		want := filepath.Join(home, "foo/bar")
		if got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	})

	t.Run("absolute path unchanged", func(t *testing.T) {
		got, err := ExpandPath("/usr/local/bin")
		if err != nil {
			t.Fatal(err)
		}
		if got != "/usr/local/bin" {
			t.Fatalf("got %q, want /usr/local/bin", got)
		}
	})

	t.Run("relative path unchanged", func(t *testing.T) {
		got, err := ExpandPath("relative/path")
		if err != nil {
			t.Fatal(err)
		}
		if got != "relative/path" {
			t.Fatalf("got %q, want relative/path", got)
		}
	})
}

func TestDefaultConfigPath(t *testing.T) {
	p := DefaultConfigPath()
	if p == "" {
		t.Fatal("expected non-empty path")
	}
	if !strings.HasSuffix(p, filepath.Join(".skillpm", "config.toml")) {
		t.Fatalf("expected path ending in .skillpm/config.toml, got %q", p)
	}
}

func TestResolveStorageRoot(t *testing.T) {
	t.Run("default tilde path", func(t *testing.T) {
		cfg := DefaultConfig()
		got, err := ResolveStorageRoot(cfg)
		if err != nil {
			t.Fatal(err)
		}
		home, _ := os.UserHomeDir()
		want := filepath.Join(home, ".skillpm")
		if got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	})

	t.Run("absolute path", func(t *testing.T) {
		cfg := Config{Storage: StorageConfig{Root: "/tmp/skillpm-test"}}
		got, err := ResolveStorageRoot(cfg)
		if err != nil {
			t.Fatal(err)
		}
		if got != "/tmp/skillpm-test" {
			t.Fatalf("got %q", got)
		}
	})

	t.Run("empty root errors", func(t *testing.T) {
		cfg := Config{Storage: StorageConfig{Root: ""}}
		_, err := ResolveStorageRoot(cfg)
		if err == nil {
			t.Fatal("expected error for empty root")
		}
	})
}

// ---------------------------------------------------------------------------
// mutate.go
// ---------------------------------------------------------------------------

func TestRemoveSource(t *testing.T) {
	t.Run("nil config", func(t *testing.T) {
		err := RemoveSource(nil, "foo")
		if err == nil {
			t.Fatal("expected error for nil config")
		}
	})

	t.Run("remove existing", func(t *testing.T) {
		cfg := Config{Sources: []SourceConfig{
			{Name: "a", Kind: "git", URL: "https://example.com", TrustTier: "review"},
			{Name: "b", Kind: "git", URL: "https://example.com", TrustTier: "review"},
		}}
		if err := RemoveSource(&cfg, "a"); err != nil {
			t.Fatal(err)
		}
		if len(cfg.Sources) != 1 {
			t.Fatalf("expected 1 source, got %d", len(cfg.Sources))
		}
		if cfg.Sources[0].Name != "b" {
			t.Fatalf("expected source b to remain, got %q", cfg.Sources[0].Name)
		}
	})

	t.Run("not found", func(t *testing.T) {
		cfg := Config{Sources: []SourceConfig{
			{Name: "a", Kind: "git", URL: "https://example.com", TrustTier: "review"},
		}}
		err := RemoveSource(&cfg, "missing")
		if err == nil {
			t.Fatal("expected error for missing source")
		}
	})
}

func TestAddSource_NilConfig(t *testing.T) {
	err := AddSource(nil, SourceConfig{Name: "x"})
	if err == nil {
		t.Fatal("expected error for nil config")
	}
}

func TestAddSource_NewSource(t *testing.T) {
	cfg := DefaultConfig()
	err := AddSource(&cfg, SourceConfig{Name: "custom", Kind: "git", URL: "https://example.com/skills.git", TrustTier: "review"})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := FindSource(cfg, "custom"); !ok {
		t.Fatal("expected custom source to be added")
	}
}

func TestFindAdapter(t *testing.T) {
	t.Run("found", func(t *testing.T) {
		cfg := DefaultConfig()
		a, ok := FindAdapter(cfg, "claude")
		if !ok {
			t.Fatal("expected to find claude adapter")
		}
		if !a.Enabled {
			t.Fatal("expected claude adapter to be enabled")
		}
	})

	t.Run("not found", func(t *testing.T) {
		cfg := DefaultConfig()
		_, ok := FindAdapter(cfg, "nonexistent")
		if ok {
			t.Fatal("expected not to find nonexistent adapter")
		}
	})
}

func TestEnableAdapter(t *testing.T) {
	t.Run("nil config", func(t *testing.T) {
		_, err := EnableAdapter(nil, "test", "global")
		if err == nil {
			t.Fatal("expected error for nil config")
		}
	})

	t.Run("empty name", func(t *testing.T) {
		cfg := DefaultConfig()
		_, err := EnableAdapter(&cfg, "", "global")
		if err == nil {
			t.Fatal("expected error for empty name")
		}
	})

	t.Run("whitespace-only name", func(t *testing.T) {
		cfg := DefaultConfig()
		_, err := EnableAdapter(&cfg, "   ", "global")
		if err == nil {
			t.Fatal("expected error for whitespace-only name")
		}
	})

	t.Run("add new adapter", func(t *testing.T) {
		cfg := Config{
			Version:  SchemaVersion,
			Sync:     SyncConfig{Mode: "system", Interval: "6h"},
			Security: SecurityConfig{Profile: "strict"},
			Storage:  StorageConfig{Root: "~/.skillpm"},
			Logging:  LoggingConfig{Level: "info", Format: "text"},
		}
		changed, err := EnableAdapter(&cfg, "newadapter", "global")
		if err != nil {
			t.Fatal(err)
		}
		if !changed {
			t.Fatal("expected changed=true for new adapter")
		}
		a, ok := FindAdapter(cfg, "newadapter")
		if !ok {
			t.Fatal("expected to find new adapter")
		}
		if !a.Enabled {
			t.Fatal("expected adapter to be enabled")
		}
		if a.Scope != "global" {
			t.Fatalf("expected scope global, got %q", a.Scope)
		}
	})

	t.Run("enable disabled adapter", func(t *testing.T) {
		cfg := Config{
			Version:  SchemaVersion,
			Sync:     SyncConfig{Mode: "system", Interval: "6h"},
			Security: SecurityConfig{Profile: "strict"},
			Storage:  StorageConfig{Root: "~/.skillpm"},
			Logging:  LoggingConfig{Level: "info", Format: "text"},
			Adapters: []AdapterConfig{
				{Name: "test", Enabled: false, Scope: "global"},
			},
		}
		changed, err := EnableAdapter(&cfg, "test", "global")
		if err != nil {
			t.Fatal(err)
		}
		if !changed {
			t.Fatal("expected changed=true when enabling disabled adapter")
		}
		if !cfg.Adapters[0].Enabled {
			t.Fatal("expected adapter to be enabled")
		}
	})

	t.Run("already enabled no change", func(t *testing.T) {
		cfg := Config{
			Version:  SchemaVersion,
			Sync:     SyncConfig{Mode: "system", Interval: "6h"},
			Security: SecurityConfig{Profile: "strict"},
			Storage:  StorageConfig{Root: "~/.skillpm"},
			Logging:  LoggingConfig{Level: "info", Format: "text"},
			Adapters: []AdapterConfig{
				{Name: "test", Enabled: true, Scope: "global"},
			},
		}
		changed, err := EnableAdapter(&cfg, "test", "global")
		if err != nil {
			t.Fatal(err)
		}
		if changed {
			t.Fatal("expected changed=false when already enabled")
		}
	})

	t.Run("set scope on existing without scope", func(t *testing.T) {
		cfg := Config{
			Version:  SchemaVersion,
			Sync:     SyncConfig{Mode: "system", Interval: "6h"},
			Security: SecurityConfig{Profile: "strict"},
			Storage:  StorageConfig{Root: "~/.skillpm"},
			Logging:  LoggingConfig{Level: "info", Format: "text"},
			Adapters: []AdapterConfig{
				{Name: "test", Enabled: true, Scope: ""},
			},
		}
		changed, err := EnableAdapter(&cfg, "test", "project")
		if err != nil {
			t.Fatal(err)
		}
		if !changed {
			t.Fatal("expected changed=true when setting scope")
		}
	})

	t.Run("default scope is global", func(t *testing.T) {
		cfg := Config{
			Version:  SchemaVersion,
			Sync:     SyncConfig{Mode: "system", Interval: "6h"},
			Security: SecurityConfig{Profile: "strict"},
			Storage:  StorageConfig{Root: "~/.skillpm"},
			Logging:  LoggingConfig{Level: "info", Format: "text"},
		}
		_, err := EnableAdapter(&cfg, "newone", "")
		if err != nil {
			t.Fatal(err)
		}
		a, ok := FindAdapter(cfg, "newone")
		if !ok {
			t.Fatal("expected to find adapter")
		}
		if a.Scope != "global" {
			t.Fatalf("expected default scope global, got %q", a.Scope)
		}
	})

	t.Run("case insensitive match", func(t *testing.T) {
		cfg := Config{
			Version:  SchemaVersion,
			Sync:     SyncConfig{Mode: "system", Interval: "6h"},
			Security: SecurityConfig{Profile: "strict"},
			Storage:  StorageConfig{Root: "~/.skillpm"},
			Logging:  LoggingConfig{Level: "info", Format: "text"},
			Adapters: []AdapterConfig{
				{Name: "Claude", Enabled: false, Scope: "global"},
			},
		}
		changed, err := EnableAdapter(&cfg, "CLAUDE", "global")
		if err != nil {
			t.Fatal(err)
		}
		if !changed {
			t.Fatal("expected changed=true for case-insensitive match")
		}
	})
}

func TestReplaceSource(t *testing.T) {
	t.Run("nil config", func(t *testing.T) {
		err := ReplaceSource(nil, SourceConfig{Name: "x"})
		if err == nil {
			t.Fatal("expected error for nil config")
		}
	})

	t.Run("replace existing", func(t *testing.T) {
		cfg := DefaultConfig()
		newSrc := SourceConfig{
			Name:      "anthropic",
			Kind:      "git",
			URL:       "https://github.com/custom/skills.git",
			Branch:    "dev",
			ScanPaths: []string{"skills"},
			TrustTier: "trusted",
		}
		err := ReplaceSource(&cfg, newSrc)
		if err != nil {
			t.Fatal(err)
		}
		s, ok := FindSource(cfg, "anthropic")
		if !ok {
			t.Fatal("expected to find replaced source")
		}
		if s.URL != "https://github.com/custom/skills.git" {
			t.Fatalf("got URL %q, want custom URL", s.URL)
		}
	})

	t.Run("not found", func(t *testing.T) {
		cfg := DefaultConfig()
		err := ReplaceSource(&cfg, SourceConfig{Name: "nonexistent", Kind: "git", URL: "https://x.com"})
		if err == nil {
			t.Fatal("expected error for missing source")
		}
	})
}

func TestFindSource_NotFound(t *testing.T) {
	cfg := Config{}
	_, ok := FindSource(cfg, "missing")
	if ok {
		t.Fatal("expected not found on empty config")
	}
}

// ---------------------------------------------------------------------------
// validate.go
// ---------------------------------------------------------------------------

func TestValidate(t *testing.T) {
	// helper: start from a valid config and tweak one field
	valid := func() Config {
		return DefaultConfig()
	}

	t.Run("valid default passes", func(t *testing.T) {
		if err := Validate(valid()); err != nil {
			t.Fatalf("expected valid: %v", err)
		}
	})

	t.Run("bad version", func(t *testing.T) {
		c := valid()
		c.Version = 999
		err := Validate(c)
		if err == nil || !strings.Contains(err.Error(), "DOC_CONFIG_VERSION") {
			t.Fatalf("expected version error, got %v", err)
		}
	})

	t.Run("missing sync mode", func(t *testing.T) {
		c := valid()
		c.Sync.Mode = ""
		err := Validate(c)
		if err == nil || !strings.Contains(err.Error(), "DOC_CONFIG_SYNC") {
			t.Fatalf("expected sync error, got %v", err)
		}
	})

	t.Run("missing sync interval", func(t *testing.T) {
		c := valid()
		c.Sync.Interval = ""
		err := Validate(c)
		if err == nil || !strings.Contains(err.Error(), "DOC_CONFIG_SYNC") {
			t.Fatalf("expected sync error, got %v", err)
		}
	})

	t.Run("missing security profile", func(t *testing.T) {
		c := valid()
		c.Security.Profile = ""
		err := Validate(c)
		if err == nil || !strings.Contains(err.Error(), "SEC_CONFIG_SECURITY") {
			t.Fatalf("expected security error, got %v", err)
		}
	})

	t.Run("missing storage root", func(t *testing.T) {
		c := valid()
		c.Storage.Root = ""
		err := Validate(c)
		if err == nil || !strings.Contains(err.Error(), "DOC_CONFIG_STORAGE") {
			t.Fatalf("expected storage error, got %v", err)
		}
	})

	t.Run("missing logging level", func(t *testing.T) {
		c := valid()
		c.Logging.Level = ""
		err := Validate(c)
		if err == nil || !strings.Contains(err.Error(), "DOC_CONFIG_LOGGING") {
			t.Fatalf("expected logging error, got %v", err)
		}
	})

	t.Run("missing logging format", func(t *testing.T) {
		c := valid()
		c.Logging.Format = ""
		err := Validate(c)
		if err == nil || !strings.Contains(err.Error(), "DOC_CONFIG_LOGGING") {
			t.Fatalf("expected logging error, got %v", err)
		}
	})

	t.Run("source with empty name", func(t *testing.T) {
		c := valid()
		c.Sources = append(c.Sources, SourceConfig{Name: "", Kind: "git", URL: "https://x.com"})
		err := Validate(c)
		if err == nil || !strings.Contains(err.Error(), "name is required") {
			t.Fatalf("expected source name error, got %v", err)
		}
	})

	t.Run("duplicate source name", func(t *testing.T) {
		c := valid()
		c.Sources = append(c.Sources, SourceConfig{Name: "anthropic", Kind: "git", URL: "https://x.com"})
		err := Validate(c)
		if err == nil || !strings.Contains(err.Error(), "duplicate source") {
			t.Fatalf("expected duplicate error, got %v", err)
		}
	})

	t.Run("unsupported source kind", func(t *testing.T) {
		c := Config{
			Version:  SchemaVersion,
			Sync:     SyncConfig{Mode: "system", Interval: "6h"},
			Security: SecurityConfig{Profile: "strict"},
			Storage:  StorageConfig{Root: "~/.skillpm"},
			Logging:  LoggingConfig{Level: "info", Format: "text"},
			Sources: []SourceConfig{
				{Name: "bad", Kind: "ftp", TrustTier: "review"},
			},
		}
		err := Validate(c)
		if err == nil || !strings.Contains(err.Error(), "unsupported source kind") {
			t.Fatalf("expected kind error, got %v", err)
		}
	})

	t.Run("invalid trust tier", func(t *testing.T) {
		c := Config{
			Version:  SchemaVersion,
			Sync:     SyncConfig{Mode: "system", Interval: "6h"},
			Security: SecurityConfig{Profile: "strict"},
			Storage:  StorageConfig{Root: "~/.skillpm"},
			Logging:  LoggingConfig{Level: "info", Format: "text"},
			Sources: []SourceConfig{
				{Name: "bad", Kind: "git", URL: "https://x.com", TrustTier: "dangerous"},
			},
		}
		err := Validate(c)
		if err == nil || !strings.Contains(err.Error(), "invalid trust tier") {
			t.Fatalf("expected trust tier error, got %v", err)
		}
	})

	t.Run("git source missing URL", func(t *testing.T) {
		c := Config{
			Version:  SchemaVersion,
			Sync:     SyncConfig{Mode: "system", Interval: "6h"},
			Security: SecurityConfig{Profile: "strict"},
			Storage:  StorageConfig{Root: "~/.skillpm"},
			Logging:  LoggingConfig{Level: "info", Format: "text"},
			Sources: []SourceConfig{
				{Name: "nogit", Kind: "git", TrustTier: "review"},
			},
		}
		err := Validate(c)
		if err == nil || !strings.Contains(err.Error(), "missing url") {
			t.Fatalf("expected git URL error, got %v", err)
		}
	})

	t.Run("dir source missing path", func(t *testing.T) {
		c := Config{
			Version:  SchemaVersion,
			Sync:     SyncConfig{Mode: "system", Interval: "6h"},
			Security: SecurityConfig{Profile: "strict"},
			Storage:  StorageConfig{Root: "~/.skillpm"},
			Logging:  LoggingConfig{Level: "info", Format: "text"},
			Sources: []SourceConfig{
				{Name: "nodir", Kind: "dir", TrustTier: "review"},
			},
		}
		err := Validate(c)
		if err == nil || !strings.Contains(err.Error(), "missing path") {
			t.Fatalf("expected dir path error, got %v", err)
		}
	})

	t.Run("clawhub source gets defaults", func(t *testing.T) {
		c := Config{
			Version:  SchemaVersion,
			Sync:     SyncConfig{Mode: "system", Interval: "6h"},
			Security: SecurityConfig{Profile: "strict"},
			Storage:  StorageConfig{Root: "~/.skillpm"},
			Logging:  LoggingConfig{Level: "info", Format: "text"},
			Sources: []SourceConfig{
				{Name: "hub", Kind: "clawhub", TrustTier: "review"},
			},
		}
		if err := Validate(c); err != nil {
			t.Fatal(err)
		}
		// Validate fills in defaults for clawhub
		if c.Sources[0].Site == "" {
			t.Fatal("expected site to be set")
		}
	})

	t.Run("empty adapter name", func(t *testing.T) {
		c := valid()
		c.Adapters = append(c.Adapters, AdapterConfig{Name: "  ", Enabled: true})
		err := Validate(c)
		if err == nil || !strings.Contains(err.Error(), "adapter name is required") {
			t.Fatalf("expected adapter name error, got %v", err)
		}
	})

	t.Run("duplicate adapter name", func(t *testing.T) {
		c := Config{
			Version:  SchemaVersion,
			Sync:     SyncConfig{Mode: "system", Interval: "6h"},
			Security: SecurityConfig{Profile: "strict"},
			Storage:  StorageConfig{Root: "~/.skillpm"},
			Logging:  LoggingConfig{Level: "info", Format: "text"},
			Adapters: []AdapterConfig{
				{Name: "dup", Enabled: true},
				{Name: "dup", Enabled: false},
			},
		}
		err := Validate(c)
		if err == nil || !strings.Contains(err.Error(), "duplicate adapter") {
			t.Fatalf("expected duplicate adapter error, got %v", err)
		}
	})

	t.Run("dir source valid", func(t *testing.T) {
		c := Config{
			Version:  SchemaVersion,
			Sync:     SyncConfig{Mode: "system", Interval: "6h"},
			Security: SecurityConfig{Profile: "strict"},
			Storage:  StorageConfig{Root: "~/.skillpm"},
			Logging:  LoggingConfig{Level: "info", Format: "text"},
			Sources: []SourceConfig{
				{Name: "local", Kind: "dir", URL: "/tmp/skills", TrustTier: "trusted"},
			},
		}
		if err := Validate(c); err != nil {
			t.Fatalf("expected valid dir source: %v", err)
		}
	})
}

// ---------------------------------------------------------------------------
// normalize.go
// ---------------------------------------------------------------------------

func TestNormalize(t *testing.T) {
	t.Run("fills all defaults on empty config", func(t *testing.T) {
		cfg := Normalize(Config{})
		if cfg.Version != SchemaVersion {
			t.Fatalf("version: got %d, want %d", cfg.Version, SchemaVersion)
		}
		if cfg.Sync.Mode != "system" {
			t.Fatalf("sync mode: got %q", cfg.Sync.Mode)
		}
		if cfg.Sync.Interval != "6h" {
			t.Fatalf("sync interval: got %q", cfg.Sync.Interval)
		}
		if cfg.Security.Profile != "strict" {
			t.Fatalf("security profile: got %q", cfg.Security.Profile)
		}
		if cfg.Storage.Root != "~/.skillpm" {
			t.Fatalf("storage root: got %q", cfg.Storage.Root)
		}
		if cfg.Logging.Level != "info" {
			t.Fatalf("logging level: got %q", cfg.Logging.Level)
		}
		if cfg.Logging.Format != "text" {
			t.Fatalf("logging format: got %q", cfg.Logging.Format)
		}
	})

	t.Run("clawhub source defaults", func(t *testing.T) {
		cfg := Normalize(Config{
			Sources: []SourceConfig{
				{Name: "hub", Kind: "clawhub"},
			},
		})
		s := cfg.Sources[0]
		if s.TrustTier != "review" {
			t.Fatalf("trust tier: got %q", s.TrustTier)
		}
		if s.Site != "https://clawhub.ai/" {
			t.Fatalf("site: got %q", s.Site)
		}
		if s.Registry != "https://clawhub.ai/" {
			t.Fatalf("registry: got %q", s.Registry)
		}
		if len(s.WellKnown) != 2 {
			t.Fatalf("well_known: got %v", s.WellKnown)
		}
		if s.APIVersion != "v1" {
			t.Fatalf("api_version: got %q", s.APIVersion)
		}
	})

	t.Run("clawhub custom registry inherits site", func(t *testing.T) {
		cfg := Normalize(Config{
			Sources: []SourceConfig{
				{Name: "hub", Kind: "clawhub", Site: "https://custom.io/"},
			},
		})
		s := cfg.Sources[0]
		if s.Registry != "https://custom.io/" {
			t.Fatalf("registry should inherit site: got %q", s.Registry)
		}
	})

	t.Run("git source defaults", func(t *testing.T) {
		cfg := Normalize(Config{
			Sources: []SourceConfig{
				{Name: "repo", Kind: "git", URL: "https://x.com/r.git"},
			},
		})
		s := cfg.Sources[0]
		if s.TrustTier != "review" {
			t.Fatalf("trust tier: got %q", s.TrustTier)
		}
		if len(s.ScanPaths) != 1 || s.ScanPaths[0] != "skills" {
			t.Fatalf("scan_paths: got %v", s.ScanPaths)
		}
	})

	t.Run("preserves existing values", func(t *testing.T) {
		cfg := Normalize(Config{
			Version:  SchemaVersion,
			Sync:     SyncConfig{Mode: "manual", Interval: "1h"},
			Security: SecurityConfig{Profile: "permissive"},
			Storage:  StorageConfig{Root: "/custom/path"},
			Logging:  LoggingConfig{Level: "debug", Format: "json"},
			Sources: []SourceConfig{
				{Name: "hub", Kind: "clawhub", Site: "https://other.io/", Registry: "https://reg.io/", TrustTier: "trusted", WellKnown: []string{"/custom"}, APIVersion: "v2"},
				{Name: "repo", Kind: "git", URL: "https://x.com", TrustTier: "trusted", ScanPaths: []string{"custom"}},
			},
		})
		if cfg.Sync.Mode != "manual" {
			t.Fatalf("mode overwritten: got %q", cfg.Sync.Mode)
		}
		if cfg.Sources[0].Registry != "https://reg.io/" {
			t.Fatalf("registry overwritten: got %q", cfg.Sources[0].Registry)
		}
		if cfg.Sources[1].ScanPaths[0] != "custom" {
			t.Fatalf("scan_paths overwritten: got %v", cfg.Sources[1].ScanPaths)
		}
	})
}

// ---------------------------------------------------------------------------
// config.go — Ensure / Load / Save error paths
// ---------------------------------------------------------------------------

func TestLoad_InvalidTOML(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "bad.toml")
	if err := os.WriteFile(path, []byte("{{not toml"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Load(path)
	if err == nil || !strings.Contains(err.Error(), "DOC_CONFIG_PARSE") {
		t.Fatalf("expected parse error, got %v", err)
	}
}

func TestLoad_ValidationFailure(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "bad_version.toml")
	// version 999 will fail validation
	if err := os.WriteFile(path, []byte("version = 999\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Load(path)
	if err == nil || !strings.Contains(err.Error(), "DOC_CONFIG_VERSION") {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestLoad_FileNotFound(t *testing.T) {
	_, err := Load("/nonexistent/path/config.toml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestSave_ValidationFailure(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "out.toml")
	cfg := Config{Version: 999} // will fail validation after Normalize sets defaults but version stays 999
	// Normalize won't overwrite Version=999 because it only sets it if 0
	err := Save(path, cfg)
	if err == nil || !strings.Contains(err.Error(), "DOC_CONFIG_VERSION") {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestSave_CreatesParentDir(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "sub", "deep", "config.toml")
	cfg := DefaultConfig()
	if err := Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("file should exist: %v", err)
	}
}

func TestEnsure_ReturnsExistingConfig(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "config.toml")
	// First create it
	cfg := DefaultConfig()
	if err := Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	// Ensure should load existing
	loaded, err := Ensure(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Version != SchemaVersion {
		t.Fatalf("expected version %d, got %d", SchemaVersion, loaded.Version)
	}
}

func TestEnsure_NonErrNotExist(t *testing.T) {
	tmp := t.TempDir()
	// Create a directory where the config file should be (so ReadFile fails with not-a-file, not ErrNotExist)
	path := filepath.Join(tmp, "config.toml")
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	_, err := Ensure(path)
	if err == nil {
		t.Fatal("expected error when path is a directory")
	}
}

// ---------------------------------------------------------------------------
// project.go — SaveProjectManifest and InitProject edge cases
// ---------------------------------------------------------------------------

func TestSaveProjectManifest_SetsVersionZero(t *testing.T) {
	tmp := t.TempDir()
	m := ProjectManifest{
		Version: 0, // should be set to SchemaVersion
		Skills:  []ProjectSkillEntry{},
	}
	if err := SaveProjectManifest(tmp, m); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadProjectManifest(tmp)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Version != SchemaVersion {
		t.Fatalf("expected version %d, got %d", SchemaVersion, loaded.Version)
	}
}

func TestSaveProjectManifest_ReadOnlyDir(t *testing.T) {
	tmp := t.TempDir()
	roDir := filepath.Join(tmp, "readonly")
	if err := os.MkdirAll(roDir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(roDir, 0o755) })
	m := DefaultProjectManifest()
	err := SaveProjectManifest(roDir, m)
	if err == nil {
		t.Fatal("expected error writing to read-only directory")
	}
}
