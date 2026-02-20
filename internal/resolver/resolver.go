package resolver

import (
	"context"
	"fmt"
	"strings"

	"skillpm/internal/config"
	"skillpm/internal/source"
	"skillpm/internal/store"
)

type ParsedRef struct {
	Source     string
	Skill      string
	Constraint string
}

type ResolvedSkill struct {
	SkillRef         string
	Source           string
	Skill            string
	ResolvedVersion  string
	Checksum         string
	Content          string
	Files            map[string]string
	SourceRef        string
	ResolverHash     string
	TrustTier        string
	IsSuspicious     bool
	IsMalwareBlocked bool
}

type Service struct {
	Sources *source.Manager
}

func ParseRef(raw string) (ParsedRef, error) {
	in := strings.TrimSpace(raw)
	if in == "" {
		return ParsedRef{}, fmt.Errorf("INS_REF_PARSE: empty skill reference")
	}
	parts := strings.SplitN(in, "@", 2)
	left := parts[0]
	constraint := ""
	if len(parts) == 2 {
		constraint = strings.TrimSpace(parts[1])
	}
	seg := strings.SplitN(left, "/", 2)
	if len(seg) != 2 || strings.TrimSpace(seg[0]) == "" || strings.TrimSpace(seg[1]) == "" {
		return ParsedRef{}, fmt.Errorf("INS_REF_PARSE: expected <source>/<skill>[@constraint], got %q", raw)
	}
	return ParsedRef{Source: seg[0], Skill: seg[1], Constraint: constraint}, nil
}

func (s *Service) ResolveMany(ctx context.Context, cfg config.Config, refs []string, lock store.Lockfile) ([]ResolvedSkill, error) {
	if s == nil || s.Sources == nil {
		return nil, fmt.Errorf("SRC_RESOLVE: source manager not configured")
	}
	out := make([]ResolvedSkill, 0, len(refs))
	for _, raw := range refs {
		pr, err := ParseRef(raw)
		if err != nil {
			return nil, err
		}
		src, ok := config.FindSource(cfg, pr.Source)
		if !ok {
			return nil, fmt.Errorf("SRC_RESOLVE: source %q not found", pr.Source)
		}

		skillRef := pr.Source + "/" + pr.Skill
		if pr.Constraint == "" || strings.EqualFold(pr.Constraint, "latest") {
			if entry, ok := findLock(lock, skillRef); ok {
				pr.Constraint = entry.ResolvedVersion
			}
		}

		resolved, err := s.Sources.Resolve(ctx, src, source.ResolveRequest{Skill: pr.Skill, Constraint: pr.Constraint})
		if err != nil {
			return nil, err
		}
		out = append(out, ResolvedSkill{
			SkillRef:         resolved.SkillRef,
			Source:           resolved.Source,
			Skill:            resolved.Skill,
			ResolvedVersion:  resolved.ResolvedVersion,
			Checksum:         resolved.Checksum,
			Content:          resolved.Content,
			Files:            resolved.Files,
			SourceRef:        resolved.SourceRef,
			ResolverHash:     resolved.ResolverHash,
			TrustTier:        src.TrustTier,
			IsSuspicious:     resolved.Moderation.IsSuspicious,
			IsMalwareBlocked: resolved.Moderation.IsMalwareBlocked,
		})
	}
	return out, nil
}

func findLock(lock store.Lockfile, skillRef string) (store.LockSkill, bool) {
	for _, s := range lock.Skills {
		if s.SkillRef == skillRef {
			return s, true
		}
	}
	return store.LockSkill{}, false
}
