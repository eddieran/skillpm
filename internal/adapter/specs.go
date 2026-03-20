package adapter

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	hyphenSkillNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)
)

type skillContract struct {
	requireName        bool
	requireDescription bool
	directoryMustMatch bool
	namePattern        *regexp.Regexp
	warnMissingName    bool
	warnMissingDesc    bool
}

type agentLayout struct {
	skillsDir string
	targetDir string
	rootPaths []string
	contract  skillContract
}

func resolveAgentLayout(name, home, projectRoot string) agentLayout {
	layout := agentLayout{
		targetDir: agentTargetDir(name, home, projectRoot),
		contract:  skillContractFor(name),
	}

	switch name {
	case "claude":
		if projectRoot != "" {
			layout.skillsDir = filepath.Join(projectRoot, ".claude", "skills")
			layout.rootPaths = []string{layout.skillsDir}
		} else {
			layout.skillsDir = filepath.Join(home, ".claude", "skills")
			layout.rootPaths = []string{layout.skillsDir}
		}
	case "codex":
		if projectRoot != "" {
			layout.skillsDir = filepath.Join(projectRoot, ".agents", "skills")
		} else {
			layout.skillsDir = filepath.Join(home, ".agents", "skills")
		}
		layout.rootPaths = []string{layout.skillsDir}
	case "cursor":
		if projectRoot != "" {
			layout.skillsDir = filepath.Join(projectRoot, ".cursor", "skills")
		} else {
			layout.skillsDir = filepath.Join(home, ".cursor", "skills")
		}
		layout.rootPaths = []string{layout.skillsDir}
	case "gemini", "antigravity":
		if projectRoot != "" {
			layout.skillsDir = filepath.Join(projectRoot, ".gemini", "skills")
			layout.rootPaths = []string{
				layout.skillsDir,
				filepath.Join(projectRoot, ".agents", "skills"),
			}
		} else {
			layout.skillsDir = filepath.Join(home, ".gemini", "skills")
			layout.rootPaths = []string{
				layout.skillsDir,
				filepath.Join(home, ".agents", "skills"),
			}
		}
	case "copilot", "vscode":
		if projectRoot != "" {
			layout.skillsDir = filepath.Join(projectRoot, ".github", "skills")
			layout.rootPaths = []string{
				layout.skillsDir,
				filepath.Join(projectRoot, ".claude", "skills"),
			}
		} else {
			layout.skillsDir = filepath.Join(home, ".copilot", "skills")
			layout.rootPaths = []string{
				layout.skillsDir,
				filepath.Join(home, ".claude", "skills"),
			}
		}
	case "trae":
		if projectRoot != "" {
			layout.skillsDir = filepath.Join(projectRoot, ".trae", "skills")
		} else {
			layout.skillsDir = filepath.Join(home, ".trae", "skills")
		}
		layout.rootPaths = []string{layout.skillsDir}
	case "opencode":
		if projectRoot != "" {
			layout.skillsDir = filepath.Join(projectRoot, ".opencode", "skills")
			layout.rootPaths = []string{
				layout.skillsDir,
				filepath.Join(projectRoot, ".claude", "skills"),
				filepath.Join(projectRoot, ".agents", "skills"),
			}
		} else {
			layout.skillsDir = filepath.Join(home, ".config", "opencode", "skills")
			layout.rootPaths = []string{
				layout.skillsDir,
				filepath.Join(home, ".claude", "skills"),
				filepath.Join(home, ".agents", "skills"),
			}
		}
	case "kiro":
		if projectRoot != "" {
			layout.skillsDir = filepath.Join(projectRoot, ".kiro", "skills")
		} else {
			layout.skillsDir = filepath.Join(home, ".kiro", "skills")
		}
		layout.rootPaths = []string{layout.skillsDir}
	case "openclaw":
		if projectRoot != "" {
			layout.skillsDir = filepath.Join(projectRoot, "skills")
			layout.rootPaths = []string{layout.skillsDir}
		} else {
			layout.skillsDir = filepath.Join(openClawWorkspaceDir(home), "skills")
			layout.rootPaths = []string{
				layout.skillsDir,
				filepath.Join(home, ".openclaw", "skills"),
				openClawStateDir(home),
				openClawConfigPath(home),
			}
		}
	default:
		if projectRoot != "" {
			layout.skillsDir = filepath.Join(projectRoot, "."+name, "skills")
		} else {
			layout.skillsDir = filepath.Join(home, "."+name, "skills")
		}
		layout.rootPaths = []string{layout.skillsDir}
	}

	layout.rootPaths = uniquePaths(layout.rootPaths...)
	return layout
}

func skillContractFor(name string) skillContract {
	switch name {
	case "copilot", "vscode":
		return skillContract{
			requireName:        true,
			requireDescription: true,
			namePattern:        hyphenSkillNamePattern,
		}
	case "gemini", "antigravity":
		return skillContract{
			requireName:        true,
			requireDescription: true,
		}
	case "kiro":
		return skillContract{
			requireName:        true,
			requireDescription: true,
			directoryMustMatch: true,
			namePattern:        hyphenSkillNamePattern,
		}
	case "opencode":
		return skillContract{
			requireName:        true,
			requireDescription: true,
			directoryMustMatch: true,
		}
	case "openclaw":
		return skillContract{
			requireName:        true,
			requireDescription: true,
		}
	case "codex":
		return skillContract{
			warnMissingName: true,
			warnMissingDesc: true,
		}
	case "claude":
		return skillContract{
			warnMissingDesc: true,
		}
	default:
		return skillContract{}
	}
}

func agentTargetDir(name, home, projectRoot string) string {
	if projectRoot != "" {
		switch name {
		case "antigravity":
			return filepath.Join(projectRoot, ".gemini", "skillpm-antigravity")
		case "vscode":
			return filepath.Join(projectRoot, ".copilot", "skillpm-vscode")
		default:
			return filepath.Join(projectRoot, "."+name, "skillpm")
		}
	}

	switch name {
	case "openclaw":
		return filepath.Join(openClawStateDir(home), "skillpm")
	case "opencode":
		return filepath.Join(home, ".config", "opencode", "skillpm")
	case "antigravity":
		return filepath.Join(home, ".gemini", "skillpm-antigravity")
	case "vscode":
		return filepath.Join(home, ".copilot", "skillpm-vscode")
	default:
		return filepath.Join(home, "."+name, "skillpm")
	}
}

func openClawStateDir(home string) string {
	if stateDir := os.Getenv("OPENCLAW_STATE_DIR"); stateDir != "" {
		return stateDir
	}
	return filepath.Join(home, ".openclaw", "state")
}

func openClawConfigPath(home string) string {
	if configPath := os.Getenv("OPENCLAW_CONFIG_PATH"); configPath != "" {
		return configPath
	}
	return filepath.Join(home, ".openclaw", "config.toml")
}

func openClawWorkspaceDir(home string) string {
	if workspaceDir := os.Getenv("OPENCLAW_WORKSPACE_DIR"); workspaceDir != "" {
		return workspaceDir
	}
	return filepath.Join(filepath.Dir(openClawStateDir(home)), "workspace")
}

func uniquePaths(paths ...string) []string {
	out := make([]string, 0, len(paths))
	seen := make(map[string]struct{}, len(paths))
	for _, path := range paths {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		out = append(out, path)
	}
	return out
}
