package config

const (
	SchemaVersion = 1
)

// DefaultConfig returns a fully-populated v1 config document.
func DefaultConfig() Config {
	return Config{
		Version: SchemaVersion,
		Sync: SyncConfig{
			Mode:     "system",
			Interval: "6h",
		},
		Security: SecurityConfig{
			Profile:           "strict",
			RequireSignatures: true,
		},
		Storage: StorageConfig{
			Root: "~/.skillpm",
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "text",
		},
		Sources: []SourceConfig{
			{
				Name:      "anthropic",
				Kind:      "git",
				URL:       "https://github.com/anthropics/skills.git",
				Branch:    "main",
				ScanPaths: []string{"skills"},
				TrustTier: "review",
			},
			{
				Name:       "clawhub",
				Kind:       "clawhub",
				Site:       "https://clawhub.ai/",
				Registry:   "https://clawhub.ai/",
				AuthBase:   "https://clawhub.ai/",
				WellKnown:  []string{"/.well-known/clawhub.json", "/.well-known/clawdhub.json"},
				APIVersion: "v1",
				TrustTier:  "review",
			},
		},
		Adapters: []AdapterConfig{
			{Name: "codex", Enabled: true, Scope: "global"},
			{Name: "openclaw", Enabled: true, Scope: "global"},
		},
	}
}
