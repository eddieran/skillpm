package resolver

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	"skillpm/internal/config"
	"skillpm/internal/source"
	"skillpm/internal/store"
)

type ParsedRef struct {
	Source     string
	Skill      string
	Constraint string
	IsURL      bool
	URL        string
	Branch     string
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

func parseURLRef(raw string) (ParsedRef, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return ParsedRef{}, err
	}
	if u.Host == "clawhub.ai" || u.Host == "www.clawhub.ai" {
		path := strings.TrimPrefix(u.Path, "/")
		if path == "" {
			return ParsedRef{}, fmt.Errorf("INS_REF_PARSE: invalid clawhub URL %q", raw)
		}
		return ParsedRef{Source: "clawhub", Skill: path}, nil
	}
	if u.Host == "github.com" || u.Host == "www.github.com" {
		parts := strings.Split(strings.TrimPrefix(u.Path, "/"), "/")
		if len(parts) < 2 {
			return ParsedRef{}, fmt.Errorf("INS_REF_PARSE: invalid github repo URL %q", raw)
		}
		org := parts[0]
		repo := parts[1]
		branch := "main"
		skillPath := ""
		if len(parts) > 3 && (parts[2] == "tree" || parts[2] == "blob") {
			branch = parts[3]
			skillPath = strings.Join(parts[4:], "/")
		}
		// Strip trailing file name (e.g., SKILL.md)
		if skillPath != "" {
			base := filepath.Base(skillPath)
			if strings.Contains(base, ".") {
				skillPath = filepath.Dir(skillPath)
				if skillPath == "." {
					skillPath = ""
				}
			}
		}
		sourceName := fmt.Sprintf("%s_%s", org, repo)
		repoURL := fmt.Sprintf("https://github.com/%s/%s.git", org, repo)
		skill := skillPath
		if skill == "" {
			skill = repo
		}
		return ParsedRef{
			Source: sourceName,
			Skill:  skill,
			IsURL:  true,
			URL:    repoURL,
			Branch: branch,
		}, nil
	}
	return ParsedRef{}, fmt.Errorf("INS_REF_PARSE: unsupported URL format %q", raw)
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
	if strings.HasPrefix(left, "http://") || strings.HasPrefix(left, "https://") {
		pr, err := parseURLRef(left)
		if err != nil {
			return ParsedRef{}, err
		}
		pr.Constraint = constraint
		return pr, nil
	}
	seg := strings.SplitN(left, "/", 2)
	if len(seg) != 2 || strings.TrimSpace(seg[0]) == "" || strings.TrimSpace(seg[1]) == "" {
		return ParsedRef{}, fmt.Errorf("INS_REF_PARSE: expected <source>/<skill>[@constraint] or URL, got %q", raw)
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
			if pr.IsURL {
				src = config.SourceConfig{
					Name:      pr.Source,
					Kind:      "git",
					URL:       pr.URL,
					Branch:    pr.Branch,
					ScanPaths: []string{"."},
					TrustTier: "review",
				}
			} else {
				return nil, fmt.Errorf("SRC_RESOLVE: source %q not found", pr.Source)
			}
		}

		skillRef := pr.Source + "/" + pr.Skill
		if pr.Constraint == "" || strings.EqualFold(pr.Constraint, "latest") {
			if entry, ok := findLock(lock, skillRef); ok {
				pr.Constraint = entry.ResolvedVersion
			}
		}

		resolved, err := s.Sources.Resolve(ctx, src, source.ResolveRequest{Skill: pr.Skill, Constraint: pr.Constraint})
		if err != nil {
			// If the URL path is a scan-path directory containing skills,
			// expand into individual skill resolutions.
			var scanErr *source.ScanPathError
			if errors.As(err, &scanErr) && pr.IsURL {
				for _, skillName := range scanErr.AvailableSkills {
					r, rErr := s.Sources.Resolve(ctx, src, source.ResolveRequest{Skill: skillName, Constraint: pr.Constraint})
					if rErr != nil {
						return nil, rErr
					}
					out = append(out, ResolvedSkill{
						SkillRef:         r.SkillRef,
						Source:           r.Source,
						Skill:            r.Skill,
						ResolvedVersion:  r.ResolvedVersion,
						Checksum:         r.Checksum,
						Content:          r.Content,
						Files:            r.Files,
						SourceRef:        r.SourceRef,
						ResolverHash:     r.ResolverHash,
						TrustTier:        src.TrustTier,
						IsSuspicious:     r.Moderation.IsSuspicious,
						IsMalwareBlocked: r.Moderation.IsMalwareBlocked,
					})
				}
				continue
			}
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
