package app

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

type localSkillPackage struct {
	Slug        string
	Version     string
	Description string
	Content     string
	Files       map[string]string
}

type skillFrontmatter struct {
	Name        string
	Version     string
	Description string
}

func loadLocalSkillPackage(skillDir string) (localSkillPackage, error) {
	absDir, err := filepath.Abs(skillDir)
	if err != nil {
		return localSkillPackage{}, fmt.Errorf("PUB_PUBLISH: resolve skill dir %q: %w", skillDir, err)
	}

	skillMDPath := filepath.Join(absDir, "SKILL.md")
	skillMD, err := os.ReadFile(skillMDPath)
	if err != nil {
		return localSkillPackage{}, fmt.Errorf("PUB_PUBLISH: cannot read SKILL.md in %q: %w", absDir, err)
	}

	meta := parseSkillFrontmatter(string(skillMD))
	files := make(map[string]string)
	if err := filepath.WalkDir(absDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(absDir, path)
		if err != nil {
			return err
		}
		relPath = filepath.ToSlash(relPath)
		if relPath == "SKILL.md" {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("PUB_PUBLISH: unsupported non-regular file %q", relPath)
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("PUB_PUBLISH: cannot read file %q: %w", relPath, err)
		}
		files[relPath] = string(data)
		return nil
	}); err != nil {
		return localSkillPackage{}, err
	}

	version := meta.Version
	if version == "" {
		version = "0.1.0"
	}

	description := meta.Description
	if description == "" {
		description = extractSkillSummary(string(skillMD))
	}

	return localSkillPackage{
		Slug:        filepath.Base(absDir),
		Version:     version,
		Description: description,
		Content:     string(skillMD),
		Files:       files,
	}, nil
}

func parseSkillFrontmatter(content string) skillFrontmatter {
	lines := strings.Split(content, "\n")
	if len(lines) < 3 || strings.TrimSpace(lines[0]) != "---" {
		return skillFrontmatter{}
	}

	end := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			end = i
			break
		}
	}
	if end < 0 {
		return skillFrontmatter{}
	}

	meta := skillFrontmatter{}
	for _, line := range lines[1:end] {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "- ") {
			continue
		}
		key, val, ok := strings.Cut(trimmed, ":")
		if !ok {
			continue
		}
		val = strings.Trim(strings.TrimSpace(val), `"'`)
		switch strings.TrimSpace(key) {
		case "name":
			meta.Name = val
		case "version":
			meta.Version = val
		case "description":
			meta.Description = val
		}
	}
	return meta
}

func extractSkillSummary(content string) string {
	lines := strings.Split(content, "\n")
	start := 0
	if len(lines) > 0 && strings.TrimSpace(lines[0]) == "---" {
		for i := 1; i < len(lines); i++ {
			if strings.TrimSpace(lines[i]) == "---" {
				start = i + 1
				break
			}
		}
	}

	inCodeBlock := false
	var paragraph []string
	for _, raw := range lines[start:] {
		line := strings.TrimSpace(raw)
		if strings.HasPrefix(line, "```") {
			inCodeBlock = !inCodeBlock
			continue
		}
		if inCodeBlock || line == "" {
			if len(paragraph) > 0 {
				break
			}
			continue
		}
		if strings.HasPrefix(line, "#") {
			if len(paragraph) > 0 {
				break
			}
			continue
		}
		paragraph = append(paragraph, line)
	}
	return strings.Join(paragraph, " ")
}
