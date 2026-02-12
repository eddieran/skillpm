package config

import "fmt"

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
