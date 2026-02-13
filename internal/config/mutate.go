package config

import (
	"fmt"
	"strings"
)

func AddSource(cfg *Config, src SourceConfig) error {
	if cfg == nil {
		return fmt.Errorf("SRC_CONFIG_SOURCE: nil config")
	}
	for _, existing := range cfg.Sources {
		if existing.Name == src.Name {
			return fmt.Errorf("SRC_CONFIG_SOURCE: source %q already exists", src.Name)
		}
	}
	cfg.Sources = append(cfg.Sources, src)
	*cfg = Normalize(*cfg)
	return Validate(*cfg)
}

func RemoveSource(cfg *Config, name string) error {
	if cfg == nil {
		return fmt.Errorf("SRC_CONFIG_SOURCE: nil config")
	}
	for i, s := range cfg.Sources {
		if s.Name == name {
			cfg.Sources = append(cfg.Sources[:i], cfg.Sources[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("SRC_CONFIG_SOURCE: source %q not found", name)
}

func FindSource(cfg Config, name string) (SourceConfig, bool) {
	for _, s := range cfg.Sources {
		if s.Name == name {
			return s, true
		}
	}
	return SourceConfig{}, false
}

func FindAdapter(cfg Config, name string) (AdapterConfig, bool) {
	for _, a := range cfg.Adapters {
		if a.Name == name {
			return a, true
		}
	}
	return AdapterConfig{}, false
}

// EnableAdapter enables an existing adapter or adds it if missing.
// Returns true when the config was changed.
func EnableAdapter(cfg *Config, name string, scope string) (bool, error) {
	if cfg == nil {
		return false, fmt.Errorf("ADP_CONFIG_ADAPTER: nil config")
	}
	if strings.TrimSpace(name) == "" {
		return false, fmt.Errorf("ADP_CONFIG_ADAPTER: empty adapter name")
	}
	if scope == "" {
		scope = "global"
	}
	name = strings.ToLower(strings.TrimSpace(name))
	for i := range cfg.Adapters {
		if strings.ToLower(cfg.Adapters[i].Name) != name {
			continue
		}
		changed := false
		if !cfg.Adapters[i].Enabled {
			cfg.Adapters[i].Enabled = true
			changed = true
		}
		if cfg.Adapters[i].Scope == "" {
			cfg.Adapters[i].Scope = scope
			changed = true
		}
		if !changed {
			return false, nil
		}
		*cfg = Normalize(*cfg)
		return true, Validate(*cfg)
	}
	cfg.Adapters = append(cfg.Adapters, AdapterConfig{Name: name, Enabled: true, Scope: scope})
	*cfg = Normalize(*cfg)
	return true, Validate(*cfg)
}

func ReplaceSource(cfg *Config, src SourceConfig) error {
	if cfg == nil {
		return fmt.Errorf("SRC_CONFIG_SOURCE: nil config")
	}
	for i := range cfg.Sources {
		if cfg.Sources[i].Name == src.Name {
			cfg.Sources[i] = src
			*cfg = Normalize(*cfg)
			return Validate(*cfg)
		}
	}
	return fmt.Errorf("SRC_CONFIG_SOURCE: source %q not found", src.Name)
}
