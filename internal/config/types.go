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
	Mode     string `toml:"mode" json:"mode"`
	Interval string `toml:"interval" json:"interval"`
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
	Name           string   `toml:"name" json:"name"`
	Kind           string   `toml:"kind" json:"kind"`
	URL            string   `toml:"url,omitempty" json:"url,omitempty"`
	Branch         string   `toml:"branch,omitempty" json:"branch,omitempty"`
	ScanPaths      []string `toml:"scan_paths,omitempty" json:"scanPaths,omitempty"`
	TrustTier      string   `toml:"trust_tier" json:"trustTier"`
	Site           string   `toml:"site,omitempty" json:"site,omitempty"`
	Registry       string   `toml:"registry,omitempty" json:"registry,omitempty"`
	AuthBase       string   `toml:"auth_base,omitempty" json:"authBase,omitempty"`
	WellKnown      []string `toml:"well_known,omitempty" json:"wellKnown,omitempty"`
	APIVersion     string   `toml:"api_version,omitempty" json:"apiVersion,omitempty"`
	CachedRegistry string   `toml:"cached_registry,omitempty" json:"cachedRegistry,omitempty"`
	MinCLIVersion  string   `toml:"min_cli_version,omitempty" json:"minCliVersion,omitempty"`
}

type AdapterConfig struct {
	Name    string `toml:"name" json:"name"`
	Enabled bool   `toml:"enabled" json:"enabled"`
	Scope   string `toml:"scope" json:"scope"`
}

// BundleEntry defines a named group of skills that can be installed together.
type BundleEntry struct {
	Name   string   `toml:"name" json:"name"`
	Skills []string `toml:"skills" json:"skills"`
}

// Scope represents the installation scope: global or project.
type Scope string

const (
	ScopeGlobal  Scope = "global"
	ScopeProject Scope = "project"
)

// ProjectManifest is the schema for .skillpm/skills.toml at a project root.
type ProjectManifest struct {
	Version  int                 `toml:"version"`
	Sources  []SourceConfig      `toml:"sources,omitempty"`
	Skills   []ProjectSkillEntry `toml:"skills"`
	Adapters []AdapterConfig     `toml:"adapters,omitempty"`
	Bundles  []BundleEntry       `toml:"bundles,omitempty"`
}

// ProjectSkillEntry declares a skill dependency in a project manifest.
type ProjectSkillEntry struct {
	Ref        string   `toml:"ref"`
	Constraint string   `toml:"constraint"`
	Deps       []string `toml:"deps,omitempty"`
}
