package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
	"skillpm/internal/fsutil"
)

func Ensure(path string) (Config, error) {
	if path == "" {
		path = DefaultConfigPath()
	}
	cfg, err := Load(path)
	if err == nil {
		return cfg, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return Config{}, err
	}
	cfg = DefaultConfig()
	if err := Save(path, cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func Load(path string) (Config, error) {
	if path == "" {
		path = DefaultConfigPath()
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("DOC_CONFIG_PARSE: %w", err)
	}
	cfg = Normalize(cfg)
	if err := Validate(cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func Save(path string, cfg Config) error {
	if path == "" {
		path = DefaultConfigPath()
	}
	cfg = Normalize(cfg)
	if err := Validate(cfg); err != nil {
		return err
	}

	parent := filepath.Dir(path)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return err
	}
	blob, err := toml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("DOC_CONFIG_ENCODE: %w", err)
	}
	return fsutil.AtomicWrite(path, blob, 0o644)
}
