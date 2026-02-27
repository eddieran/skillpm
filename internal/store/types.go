package store

import "time"

const StateVersion = 1

type State struct {
	Version    int              `toml:"version"`
	Installed  []InstalledSkill `toml:"installed"`
	Injections []InjectionState `toml:"injections"`
}

type InstalledSkill struct {
	SkillRef         string    `toml:"skill_ref" json:"skillRef"`
	Source           string    `toml:"source" json:"source"`
	Skill            string    `toml:"skill" json:"skill"`
	ResolvedVersion  string    `toml:"resolved_version" json:"resolvedVersion"`
	Checksum         string    `toml:"checksum" json:"checksum"`
	SourceRef        string    `toml:"source_ref" json:"sourceRef"`
	InstalledAt      time.Time `toml:"installed_at" json:"installedAt"`
	TrustTier        string    `toml:"trust_tier" json:"trustTier"`
	IsSuspicious     bool      `toml:"is_suspicious,omitempty" json:"isSuspicious,omitempty"`
	IsMalwareBlocked bool      `toml:"is_malware_blocked,omitempty" json:"isMalwareBlocked,omitempty"`
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
