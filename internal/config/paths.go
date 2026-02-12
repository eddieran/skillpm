package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

func DefaultConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".skillpm/config.toml"
	}
	return filepath.Join(home, ".skillpm", "config.toml")
}

func ExpandPath(path string) (string, error) {
	if path == "" {
		return "", errors.New("empty path")
	}
	if path == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return home, nil
	}
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, strings.TrimPrefix(path, "~/")), nil
	}
	return path, nil
}

func ResolveStorageRoot(cfg Config) (string, error) {
	expanded, err := ExpandPath(cfg.Storage.Root)
	if err != nil {
		return "", err
	}
	return filepath.Clean(expanded), nil
}
