package source

import "skillpm/internal/config"

type UpdateResult struct {
	Source config.SourceConfig
	Note   string
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
	Moderation      Moderation
	ResolverHash    string
}

type Moderation struct {
	IsMalwareBlocked bool
	IsSuspicious     bool
}
