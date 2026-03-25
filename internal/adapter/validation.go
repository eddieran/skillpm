package adapter

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type skillMetadata struct {
	Name        string
	Description string
}

type skillCopyPlan struct {
	Ref          string
	SrcDir       string
	DestDir      string
	Meta         skillMetadata
	SkillContent string
}

func (f *fileAdapter) buildCopyPlan(skillRefs []string) ([]skillCopyPlan, []string, error) {
	plans := make([]skillCopyPlan, 0, len(skillRefs))
	warnings := make([]string, 0)
	problems := make([]string, 0)
	seen := make(map[string]struct{}, len(skillRefs))

	for _, ref := range skillRefs {
		if _, ok := seen[ref]; ok {
			continue
		}
		seen[ref] = struct{}{}

		srcDir := f.findInstalledSkillDir(ref)
		if srcDir == "" {
			problems = append(problems, fmt.Sprintf("%s: installed files not found", ref))
			continue
		}

		meta, content, err := readSkillDocument(srcDir)
		if err != nil {
			problems = append(problems, fmt.Sprintf("%s: %v", ref, err))
			continue
		}

		destName := ExtractSkillName(ref)
		destDir := filepath.Join(f.skillsDir, destName)
		meta, content, normalizeWarnings := f.normalizeSkillDocument(ref, destName, meta, content)
		warnings = append(warnings, normalizeWarnings...)
		metaWarnings, metaProblems := f.validateSkillMetadata(ref, destName, meta)
		warnings = append(warnings, metaWarnings...)
		problems = append(problems, metaProblems...)
		plans = append(plans, skillCopyPlan{
			Ref:          ref,
			SrcDir:       srcDir,
			DestDir:      destDir,
			Meta:         meta,
			SkillContent: content,
		})
	}

	sort.Strings(warnings)
	if len(problems) > 0 {
		sort.Strings(problems)
		return nil, warnings, fmt.Errorf("ADP_INJECT_VALIDATE: %s", strings.Join(problems, "; "))
	}
	return plans, warnings, nil
}

func (f *fileAdapter) validateSkillMetadata(ref, destName string, meta skillMetadata) ([]string, []string) {
	contract := f.contract
	warnings := []string{}
	problems := []string{}

	if contract.requireName && meta.Name == "" {
		problems = append(problems, fmt.Sprintf("%s: missing required frontmatter field \"name\"", ref))
	}
	if contract.requireDescription && meta.Description == "" {
		problems = append(problems, fmt.Sprintf("%s: missing required frontmatter field \"description\"", ref))
	}
	if contract.directoryMustMatch && meta.Name != "" && meta.Name != destName {
		problems = append(problems, fmt.Sprintf("%s: directory name %q does not match skill name %q", ref, destName, meta.Name))
	}
	if contract.namePattern != nil && meta.Name != "" && !contract.namePattern.MatchString(meta.Name) {
		problems = append(problems, fmt.Sprintf("%s: skill name %q does not match required pattern %s", ref, meta.Name, contract.namePattern.String()))
	}
	if contract.warnMissingName && meta.Name == "" {
		warnings = append(warnings, fmt.Sprintf("%s: missing frontmatter field \"name\"; discoverability depends on the agent inferring the directory name", ref))
	}
	if contract.warnMissingDesc && meta.Description == "" {
		warnings = append(warnings, fmt.Sprintf("%s: missing frontmatter field \"description\"; automatic skill activation may be degraded", ref))
	}

	return warnings, problems
}

func (f *fileAdapter) normalizeSkillDocument(ref, destName string, meta skillMetadata, content string) (skillMetadata, string, []string) {
	effective := meta
	warnings := []string{}

	needsName := f.contract.requireName || f.contract.directoryMustMatch || f.contract.namePattern != nil
	if needsName && effective.Name == "" {
		effective.Name = destName
		warnings = append(warnings, fmt.Sprintf("%s: synthesized frontmatter field \"name\" from the injected skill directory", ref))
	}

	if f.contract.requireDescription && effective.Description == "" {
		effective.Description = inferSkillDescription(content, destName)
		warnings = append(warnings, fmt.Sprintf("%s: synthesized frontmatter field \"description\" for agent compatibility", ref))
	}

	if effective == meta {
		return meta, content, warnings
	}
	return effective, upsertSkillFrontmatter(content, effective), warnings
}

func (f *fileAdapter) verifyCopiedSkills(plans []skillCopyPlan) error {
	problems := make([]string, 0)
	for _, plan := range plans {
		skillPath := filepath.Join(plan.DestDir, "SKILL.md")
		if _, err := os.Stat(skillPath); err != nil {
			problems = append(problems, fmt.Sprintf("%s: missing copied SKILL.md at %s", plan.Ref, skillPath))
		}
	}
	if len(problems) == 0 {
		return nil
	}
	sort.Strings(problems)
	return fmt.Errorf("ADP_INJECT_DISCOVERY: %s", strings.Join(problems, "; "))
}

func readSkillDocument(skillDir string) (skillMetadata, string, error) {
	path := filepath.Join(skillDir, "SKILL.md")
	blob, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return skillMetadata{}, "", fmt.Errorf("required file SKILL.md not found")
		}
		return skillMetadata{}, "", err
	}
	content := string(blob)
	return parseSkillMetadata(content), content, nil
}

func parseSkillMetadata(content string) skillMetadata {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return skillMetadata{}
	}

	meta := skillMetadata{}
	currentKey := ""
	currentValue := ""

	flush := func() {
		switch currentKey {
		case "description":
			meta.Description = strings.TrimSpace(currentValue)
		}
		currentKey = ""
		currentValue = ""
	}

	for i := 1; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)
		if trimmed == "---" {
			flush()
			break
		}

		if currentKey != "" {
			if trimmed == "" {
				continue
			}
			if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
				if currentValue != "" {
					currentValue += "\n"
				}
				currentValue += strings.TrimSpace(line)
				continue
			}
			flush()
		}

		if strings.HasPrefix(trimmed, "name:") {
			meta.Name = trimYAMLScalar(strings.TrimSpace(strings.TrimPrefix(trimmed, "name:")))
			continue
		}
		if strings.HasPrefix(trimmed, "description:") {
			value := strings.TrimSpace(strings.TrimPrefix(trimmed, "description:"))
			value = trimYAMLScalar(value)
			if value == "" {
				currentKey = "description"
				continue
			}
			meta.Description = value
		}
	}

	return meta
}

func trimYAMLScalar(value string) string {
	value = strings.TrimSpace(value)
	if value == ">" || value == "|" {
		return ""
	}
	return strings.Trim(value, `"'`)
}

func upsertSkillFrontmatter(content string, meta skillMetadata) string {
	lines := strings.Split(content, "\n")
	if len(lines) > 0 && strings.TrimSpace(lines[0]) == "---" {
		for i := 1; i < len(lines); i++ {
			if strings.TrimSpace(lines[i]) != "---" {
				continue
			}

			frontmatter := append([]string{}, lines[1:i]...)
			hasName := false
			hasDescription := false
			for _, line := range frontmatter {
				trimmed := strings.TrimSpace(line)
				if strings.HasPrefix(trimmed, "name:") {
					hasName = true
				}
				if strings.HasPrefix(trimmed, "description:") {
					hasDescription = true
				}
			}
			if !hasName && meta.Name != "" {
				frontmatter = append(frontmatter, "name: "+yamlQuote(meta.Name))
			}
			if !hasDescription && meta.Description != "" {
				frontmatter = append(frontmatter, "description: "+yamlQuote(meta.Description))
			}

			out := []string{"---"}
			out = append(out, frontmatter...)
			out = append(out, "---")
			out = append(out, lines[i+1:]...)
			return strings.Join(out, "\n")
		}
	}

	frontmatter := []string{"---"}
	if meta.Name != "" {
		frontmatter = append(frontmatter, "name: "+yamlQuote(meta.Name))
	}
	if meta.Description != "" {
		frontmatter = append(frontmatter, "description: "+yamlQuote(meta.Description))
	}
	frontmatter = append(frontmatter, "---")
	if strings.TrimSpace(content) == "" {
		return strings.Join(frontmatter, "\n") + "\n"
	}
	return strings.Join(frontmatter, "\n") + "\n\n" + strings.TrimLeft(content, "\r\n")
}

func yamlQuote(value string) string {
	return strconv.Quote(strings.TrimSpace(value))
}

func inferSkillDescription(content, fallbackName string) string {
	body := stripSkillFrontmatter(content)
	lines := strings.Split(body, "\n")
	inFence := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == "":
			continue
		case strings.HasPrefix(trimmed, "```"), strings.HasPrefix(trimmed, "~~~"):
			inFence = !inFence
			continue
		case inFence:
			continue
		case strings.HasPrefix(trimmed, "#"):
			continue
		}

		if len(trimmed) > 120 {
			trimmed = trimmed[:117] + "..."
		}
		return trimmed
	}

	if fallbackName == "" {
		return "Imported skill"
	}
	return fallbackName + " skill"
}

func stripSkillFrontmatter(content string) string {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return content
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			return strings.Join(lines[i+1:], "\n")
		}
	}
	return content
}
