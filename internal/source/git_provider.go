package source

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"skillpm/internal/config"
)

type gitExecFunc func(ctx context.Context, dir string, args ...string) ([]byte, error)

type gitProvider struct {
	cacheRoot string
	execGit   gitExecFunc
}

func defaultGitExec(ctx context.Context, dir string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git %s: %w\n%s", strings.Join(args, " "), err, string(out))
	}
	return out, nil
}

func (p *gitProvider) Update(ctx context.Context, src config.SourceConfig) (UpdateResult, error) {
	if src.URL == "" {
		return UpdateResult{}, fmt.Errorf("SRC_GIT_UPDATE: source %q missing url", src.Name)
	}
	branch := src.Branch
	if branch == "" {
		branch = "main"
	}

	cacheDir := p.repoCacheDir(src)
	if err := os.MkdirAll(filepath.Dir(cacheDir), 0o755); err != nil {
		return UpdateResult{}, fmt.Errorf("SRC_GIT_UPDATE: %w", err)
	}

	if isGitRepo(cacheDir) {
		if _, err := p.execGit(ctx, cacheDir, "fetch", "origin", branch, "--depth", "1"); err != nil {
			return UpdateResult{}, fmt.Errorf("SRC_GIT_UPDATE: fetch failed: %w", err)
		}
		if _, err := p.execGit(ctx, cacheDir, "reset", "--hard", "origin/"+branch); err != nil {
			return UpdateResult{}, fmt.Errorf("SRC_GIT_UPDATE: reset failed: %w", err)
		}
	} else {
		if _, err := p.execGit(ctx, "", "clone", "--depth", "1", "--single-branch", "--branch", branch, src.URL, cacheDir); err != nil {
			return UpdateResult{}, fmt.Errorf("SRC_GIT_UPDATE: clone failed: %w", err)
		}
	}
	return UpdateResult{Source: src, Note: "git source updated"}, nil
}

func (p *gitProvider) Search(_ context.Context, src config.SourceConfig, query string) ([]SearchResult, error) {
	cacheDir := p.repoCacheDir(src)
	if !isGitRepo(cacheDir) {
		return nil, fmt.Errorf("SRC_GIT_SEARCH: source %q not cloned; run 'skillpm source update %s' first", src.Name, src.Name)
	}

	scanPaths := src.ScanPaths
	if len(scanPaths) == 0 {
		scanPaths = []string{"."}
	}

	var results []SearchResult
	for _, sp := range scanPaths {
		base := filepath.Join(cacheDir, sp)
		entries, err := os.ReadDir(base)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			skillMdPath := filepath.Join(base, entry.Name(), "SKILL.md")
			if _, err := os.Stat(skillMdPath); err != nil {
				continue
			}
			name := entry.Name()
			if query != "" && !strings.Contains(strings.ToLower(name), strings.ToLower(query)) {
				continue
			}
			results = append(results, SearchResult{
				Source:      src.Name,
				Slug:        src.Name + "/" + name,
				Name:        name,
				Description: readFirstHeading(skillMdPath),
			})
		}
	}
	sort.Slice(results, func(i, j int) bool { return results[i].Slug < results[j].Slug })
	return results, nil
}

func (p *gitProvider) Resolve(ctx context.Context, src config.SourceConfig, req ResolveRequest) (ResolveResult, error) {
	if req.Skill == "" {
		return ResolveResult{}, fmt.Errorf("SRC_GIT_RESOLVE: empty skill")
	}

	cacheDir := p.repoCacheDir(src)
	if !isGitRepo(cacheDir) {
		// Auto-clone if cache doesn't exist.
		if _, err := p.Update(ctx, src); err != nil {
			return ResolveResult{}, err
		}
	}

	skillDir, err := findSkillDir(cacheDir, src.ScanPaths, req.Skill)
	if err != nil {
		return ResolveResult{}, err
	}

	// Read SKILL.md
	skillMdPath := filepath.Join(skillDir, "SKILL.md")
	contentBytes, err := os.ReadFile(skillMdPath)
	if err != nil {
		return ResolveResult{}, fmt.Errorf("SRC_GIT_RESOLVE: reading SKILL.md: %w", err)
	}
	content := string(contentBytes)

	// Walk skill dir for ancillary files
	files := map[string]string{}
	var totalSize int64
	const maxFileSize = 1 << 20  // 1MB per file
	const maxTotalSize = 10 << 20 // 10MB total

	err = filepath.WalkDir(skillDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil // skip errors
		}
		if d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(skillDir, path)
		if rel == "SKILL.md" {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		if info.Size() > maxFileSize {
			return nil
		}
		if totalSize+info.Size() > maxTotalSize {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		totalSize += info.Size()
		files[filepath.ToSlash(rel)] = string(data)
		return nil
	})
	if err != nil {
		return ResolveResult{}, fmt.Errorf("SRC_GIT_RESOLVE: walking skill dir: %w", err)
	}

	// Version from git
	version := req.Constraint
	if version == "" || strings.EqualFold(version, "latest") {
		hash, gitErr := p.execGit(ctx, cacheDir, "rev-parse", "--short", "HEAD")
		if gitErr != nil {
			version = "0.0.0+git.unknown"
		} else {
			version = "0.0.0+git." + strings.TrimSpace(string(hash))
		}
	}

	checksum := computeChecksum(contentBytes, files)

	return ResolveResult{
		SkillRef:        fmt.Sprintf("%s/%s", src.Name, req.Skill),
		ResolvedVersion: version,
		Checksum:        checksum,
		SourceRef:       fmt.Sprintf("%s@%s", src.URL, version),
		Source:          src.Name,
		Skill:           req.Skill,
		Content:         content,
		Files:           files,
	}, nil
}

// repoCacheDir returns a deterministic cache directory for the source.
func (p *gitProvider) repoCacheDir(src config.SourceConfig) string {
	h := sha256.Sum256([]byte(src.URL))
	short := hex.EncodeToString(h[:])[:16]
	return filepath.Join(p.cacheRoot, src.Name+"-"+short)
}

// isGitRepo checks whether the directory contains a .git dir.
func isGitRepo(dir string) bool {
	info, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil && info.IsDir()
}

// findSkillDir locates the skill directory within the cached repo.
func findSkillDir(cacheDir string, scanPaths []string, skill string) (string, error) {
	if len(scanPaths) == 0 {
		scanPaths = []string{"."}
	}
	for _, sp := range scanPaths {
		candidate := filepath.Join(cacheDir, sp, skill)
		if _, err := os.Stat(filepath.Join(candidate, "SKILL.md")); err == nil {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("SRC_GIT_RESOLVE: skill %q not found in scan paths %v", skill, scanPaths)
}

// computeChecksum creates a deterministic SHA256 over SKILL.md content and all ancillary files.
func computeChecksum(content []byte, files map[string]string) string {
	h := sha256.New()
	h.Write(content)
	// Sort keys for determinism.
	keys := make([]string, 0, len(files))
	for k := range files {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		h.Write([]byte(k))
		h.Write([]byte(files[k]))
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}

// readFirstHeading extracts the first markdown heading from a file.
func readFirstHeading(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "# ") {
			return strings.TrimPrefix(line, "# ")
		}
	}
	return ""
}
