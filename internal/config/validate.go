package config

import (
	"fmt"
	"strings"
)

var allowedTrustTiers = map[string]struct{}{
	"trusted":   {},
	"review":    {},
	"untrusted": {},
}

var allowedSourceKinds = map[string]struct{}{
	"git":     {},
	"clawhub": {},
	"dir":     {},
}

func Validate(cfg Config) error {
	if cfg.Version != SchemaVersion {
		return fmt.Errorf("DOC_CONFIG_VERSION: unsupported version %d", cfg.Version)
	}
	if cfg.Sync.Mode == "" || cfg.Sync.Interval == "" {
		return fmt.Errorf("DOC_CONFIG_SYNC: missing sync mode/interval")
	}
	if cfg.Security.Profile == "" {
		return fmt.Errorf("SEC_CONFIG_SECURITY: missing security profile")
	}
	if cfg.Storage.Root == "" {
		return fmt.Errorf("DOC_CONFIG_STORAGE: missing storage root")
	}
	if cfg.Logging.Level == "" || cfg.Logging.Format == "" {
		return fmt.Errorf("DOC_CONFIG_LOGGING: missing logging level/format")
	}

	names := map[string]struct{}{}
	for i := range cfg.Sources {
		s := &cfg.Sources[i]
		if s.Name == "" {
			return fmt.Errorf("SRC_CONFIG_SOURCE: source name is required")
		}
		if _, ok := names[s.Name]; ok {
			return fmt.Errorf("SRC_CONFIG_SOURCE: duplicate source name %q", s.Name)
		}
		names[s.Name] = struct{}{}
		if _, ok := allowedSourceKinds[s.Kind]; !ok {
			return fmt.Errorf("SRC_CONFIG_SOURCE: unsupported source kind %q", s.Kind)
		}
		if s.TrustTier == "" {
			s.TrustTier = "review"
		}
		if _, ok := allowedTrustTiers[s.TrustTier]; !ok {
			return fmt.Errorf("SEC_CONFIG_TRUST: invalid trust tier %q", s.TrustTier)
		}
		switch s.Kind {
		case "git":
			if s.URL == "" {
				return fmt.Errorf("SRC_CONFIG_SOURCE: git source %q missing url", s.Name)
			}
			if len(s.ScanPaths) == 0 {
				s.ScanPaths = []string{"skills"}
			}
		case "clawhub":
			if s.Site == "" {
				s.Site = "https://clawhub.ai/"
			}
			if s.Registry == "" {
				s.Registry = s.Site
			}
			if len(s.WellKnown) == 0 {
				s.WellKnown = []string{"/.well-known/clawhub.json", "/.well-known/clawdhub.json"}
			}
			if s.APIVersion == "" {
				s.APIVersion = "v1"
			}
		case "dir":
			if s.URL == "" {
				return fmt.Errorf("SRC_CONFIG_SOURCE: dir source %q missing path", s.Name)
			}
		}
	}

	adapterNames := map[string]struct{}{}
	for _, a := range cfg.Adapters {
		if strings.TrimSpace(a.Name) == "" {
			return fmt.Errorf("ADP_CONFIG_ADAPTER: adapter name is required")
		}
		if _, ok := adapterNames[a.Name]; ok {
			return fmt.Errorf("ADP_CONFIG_ADAPTER: duplicate adapter %q", a.Name)
		}
		adapterNames[a.Name] = struct{}{}
	}

	return nil
}
