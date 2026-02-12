package source

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"skillpm/internal/config"
)

type gitProvider struct{}

func (p *gitProvider) Update(_ context.Context, src config.SourceConfig) (UpdateResult, error) {
	if src.URL == "" {
		return UpdateResult{}, fmt.Errorf("SRC_GIT_UPDATE: source %q missing url", src.Name)
	}
	return UpdateResult{Source: src, Note: "git source metadata refreshed"}, nil
}

func (p *gitProvider) Search(_ context.Context, src config.SourceConfig, query string) ([]SearchResult, error) {
	// v1 baseline returns empty list for git sources until repository indexing is enabled.
	return []SearchResult{}, nil
}

func (p *gitProvider) Resolve(_ context.Context, src config.SourceConfig, req ResolveRequest) (ResolveResult, error) {
	if req.Skill == "" {
		return ResolveResult{}, fmt.Errorf("SRC_GIT_RESOLVE: empty skill")
	}
	version := req.Constraint
	if version == "" || version == "latest" {
		version = "0.0.0+git.latest"
	}
	h := sha256.Sum256([]byte(src.URL + ":" + req.Skill + "@" + version))
	checksum := "sha256:" + hex.EncodeToString(h[:])
	return ResolveResult{
		SkillRef:        fmt.Sprintf("%s/%s", src.Name, req.Skill),
		ResolvedVersion: version,
		Checksum:        checksum,
		SourceRef:       fmt.Sprintf("%s@%s", src.URL, version),
		Source:          src.Name,
		Skill:           req.Skill,
	}, nil
}
