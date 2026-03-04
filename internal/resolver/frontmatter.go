package resolver

import (
	"strings"
)

// ParseSkillDeps extracts the "deps" field from SKILL.md YAML frontmatter.
// Frontmatter is delimited by --- lines at the start of the file.
//
// Supported formats:
//
//	deps: [skill-a, skill-b]
//	deps: skill-a, skill-b
//	deps:
//	  - skill-a
//	  - skill-b
func ParseSkillDeps(content string) []string {
	lines := strings.Split(content, "\n")
	if len(lines) < 2 || strings.TrimSpace(lines[0]) != "---" {
		return nil
	}
	inFrontmatter := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "---" {
			if inFrontmatter {
				// closing delimiter
				break
			}
			inFrontmatter = true
			continue
		}
		if !inFrontmatter {
			continue
		}
		if strings.HasPrefix(trimmed, "deps:") {
			val := strings.TrimPrefix(trimmed, "deps:")
			val = strings.TrimSpace(val)

			// Inline: deps: [a, b, c] or deps: a, b, c
			if val != "" {
				val = strings.Trim(val, "[]")
				if val == "" {
					continue
				}
				parts := strings.Split(val, ",")
				var deps []string
				for _, p := range parts {
					p = strings.TrimSpace(p)
					p = strings.Trim(p, `"'`)
					if p != "" {
						deps = append(deps, p)
					}
				}
				return deps
			}

			// Block list: deps:\n  - a\n  - b
			var deps []string
			for j := i + 1; j < len(lines); j++ {
				blockLine := lines[j]
				blockTrimmed := strings.TrimSpace(blockLine)
				if blockTrimmed == "---" {
					break
				}
				// Stop if we hit a non-indented line (next YAML key)
				if blockTrimmed != "" && !strings.HasPrefix(blockLine, " ") && !strings.HasPrefix(blockLine, "\t") {
					break
				}
				if strings.HasPrefix(blockTrimmed, "- ") {
					dep := strings.TrimPrefix(blockTrimmed, "- ")
					dep = strings.TrimSpace(dep)
					dep = strings.Trim(dep, `"'`)
					if dep != "" {
						deps = append(deps, dep)
					}
				}
			}
			if len(deps) > 0 {
				return deps
			}
			return nil
		}
	}
	return nil
}
