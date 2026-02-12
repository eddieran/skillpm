package store

import "time"

const StateVersion = 1

type State struct {
	Version    int              `toml:"version"`
	Installed  []InstalledSkill `toml:"installed"`
	Injections []InjectionState `toml:"injections"`
}

type InstalledSkill struct {
	SkillRef         string    `toml:"skill_ref"`
	Source           string    `toml:"source"`
	Skill            string    `toml:"skill"`
	ResolvedVersion  string    `toml:"resolved_version"`
	Checksum         string    `toml:"checksum"`
	SourceRef        string    `toml:"source_ref"`
	InstalledAt      time.Time `toml:"installed_at"`
	TrustTier        string    `toml:"trust_tier"`
	IsSuspicious     bool      `toml:"is_suspicious,omitempty"`
	IsMalwareBlocked bool      `toml:"is_malware_blocked,omitempty"`
}

type InjectionState struct {
	Agent     string    `toml:"agent"`
	Skills    []string  `toml:"skills"`
	UpdatedAt time.Time `toml:"updated_at"`
}

type Lockfile struct {
	Version int         `toml:"version"`
	Skills  []LockSkill `toml:"skills"`
}

type LockSkill struct {
	SkillRef        string            `toml:"skillRef"`
	ResolvedVersion string            `toml:"resolvedVersion"`
	Checksum        string            `toml:"checksum"`
	SourceRef       string            `toml:"sourceRef"`
	Metadata        map[string]string `toml:"metadata,omitempty"`
}
