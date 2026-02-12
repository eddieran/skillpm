package config

func Normalize(cfg Config) Config {
	if cfg.Version == 0 {
		cfg.Version = SchemaVersion
	}
	if cfg.Sync.Mode == "" {
		cfg.Sync.Mode = "system"
	}
	if cfg.Sync.Interval == "" {
		cfg.Sync.Interval = "6h"
	}
	if cfg.Security.Profile == "" {
		cfg.Security.Profile = "strict"
	}
	if cfg.Storage.Root == "" {
		cfg.Storage.Root = "~/.skillpm"
	}
	if cfg.Logging.Level == "" {
		cfg.Logging.Level = "info"
	}
	if cfg.Logging.Format == "" {
		cfg.Logging.Format = "text"
	}
	for i := range cfg.Sources {
		if cfg.Sources[i].TrustTier == "" {
			cfg.Sources[i].TrustTier = "review"
		}
		switch cfg.Sources[i].Kind {
		case "clawhub":
			if cfg.Sources[i].Site == "" {
				cfg.Sources[i].Site = "https://clawhub.ai/"
			}
			if cfg.Sources[i].Registry == "" {
				cfg.Sources[i].Registry = cfg.Sources[i].Site
			}
			if len(cfg.Sources[i].WellKnown) == 0 {
				cfg.Sources[i].WellKnown = []string{"/.well-known/clawhub.json", "/.well-known/clawdhub.json"}
			}
			if cfg.Sources[i].APIVersion == "" {
				cfg.Sources[i].APIVersion = "v1"
			}
		case "git":
			if len(cfg.Sources[i].ScanPaths) == 0 {
				cfg.Sources[i].ScanPaths = []string{"skills"}
			}
		}
	}
	return cfg
}
