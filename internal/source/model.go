package source

import "skillpm/internal/config"

type UpdateResult struct {
	Source config.SourceConfig `json:"source"`
	Note   string              `json:"note"`
}

type SearchResult struct {
	Source      string `json:"source"`
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

type ResolveRequest struct {
	Skill      string
	Constraint string
}

type ResolveResult struct {
	SkillRef        string
	ResolvedVersion string
	Checksum        string
	SourceRef       string
	Source          string
	Skill           string
	Content         string
	Files           map[string]string // relative-path -> content (ancillary files beyond SKILL.md)
	Moderation      Moderation
	ResolverHash    string
}

type Moderation struct {
	IsMalwareBlocked bool
	IsSuspicious     bool
}

// PublishRequest describes a skill to be published to a registry.
type PublishRequest struct {
	Slug        string
	Version     string
	Content     string            // SKILL.md content
	Files       map[string]string // ancillary files
	Description string
}

// PublishResult is returned after a successful publish.
type PublishResult struct {
	Slug    string `json:"slug"`
	Version string `json:"version"`
	URL     string `json:"url"`
}
