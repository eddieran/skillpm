package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfigIsValid(t *testing.T) {
	cfg := DefaultConfig()
	if err := Validate(cfg); err != nil {
		t.Fatalf("default config should validate: %v", err)
	}
	if _, ok := FindSource(cfg, "clawhub"); !ok {
		t.Fatalf("expected default clawhub source")
	}
}

func TestEnsureCreatesAndLoadsConfig(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "config.toml")
	cfg, err := Ensure(path)
	if err != nil {
		t.Fatalf("ensure failed: %v", err)
	}
	if cfg.Version != SchemaVersion {
		t.Fatalf("expected schema version %d, got %d", SchemaVersion, cfg.Version)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("config file should exist: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if len(loaded.Sources) == 0 {
		t.Fatalf("expected default sources")
	}
}

func TestAddSourceRejectsDuplicate(t *testing.T) {
	cfg := DefaultConfig()
	err := AddSource(&cfg, SourceConfig{Name: "clawhub", Kind: "clawhub", Site: "https://clawhub.ai/", TrustTier: "review"})
	if err == nil {
		t.Fatalf("expected duplicate source error")
	}
}
