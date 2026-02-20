package config

// Config is the frozen v1 global schema.
type Config struct {
	Version  int             `toml:"version"`
	Sync     SyncConfig      `toml:"sync"`
	Security SecurityConfig  `toml:"security"`
	Storage  StorageConfig   `toml:"storage"`
	Logging  LoggingConfig   `toml:"logging"`
	Sources  []SourceConfig  `toml:"sources"`
	Adapters []AdapterConfig `toml:"adapters"`
}

type SyncConfig struct {
	Mode     string `toml:"mode"`
	Interval string `toml:"interval"`
}

type SecurityConfig struct {
	Profile           string     `toml:"profile"`
	RequireSignatures bool       `toml:"require_signatures"`
	Scan              ScanConfig `toml:"scan"`
}

type ScanConfig struct {
	Enabled       bool     `toml:"enabled"`
	BlockSeverity string   `toml:"block_severity"`
	DisabledRules []string `toml:"disabled_rules,omitempty"`
}

type StorageConfig struct {
	Root string `toml:"root"`
}

type LoggingConfig struct {
	Level  string `toml:"level"`
	Format string `toml:"format"`
}

type SourceConfig struct {
	Name           string   `toml:"name"`
	Kind           string   `toml:"kind"`
	URL            string   `toml:"url,omitempty"`
	Branch         string   `toml:"branch,omitempty"`
	ScanPaths      []string `toml:"scan_paths,omitempty"`
	TrustTier      string   `toml:"trust_tier"`
	Site           string   `toml:"site,omitempty"`
	Registry       string   `toml:"registry,omitempty"`
	AuthBase       string   `toml:"auth_base,omitempty"`
	WellKnown      []string `toml:"well_known,omitempty"`
	APIVersion     string   `toml:"api_version,omitempty"`
	CachedRegistry string   `toml:"cached_registry,omitempty"`
	MinCLIVersion  string   `toml:"min_cli_version,omitempty"`
}

type AdapterConfig struct {
	Name    string `toml:"name"`
	Enabled bool   `toml:"enabled"`
	Scope   string `toml:"scope"`
}
