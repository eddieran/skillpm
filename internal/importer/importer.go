package importer

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Descriptor struct {
	Name      string
	RootPath  string
	SkillFile string
}

func ValidateSkillDir(path string) (Descriptor, error) {
	clean := filepath.Clean(path)
	skillFile := filepath.Join(clean, "SKILL.md")
	info, err := os.Stat(skillFile)
	if err != nil {
		if os.IsNotExist(err) {
			return Descriptor{}, fmt.Errorf("IMP_SKILL_SHAPE: missing SKILL.md in %q", clean)
		}
		return Descriptor{}, err
	}
	if info.IsDir() {
		return Descriptor{}, fmt.Errorf("IMP_SKILL_SHAPE: SKILL.md is a directory")
	}
	name := filepath.Base(clean)
	if strings.TrimSpace(name) == "" || name == "." || name == string(filepath.Separator) {
		return Descriptor{}, fmt.Errorf("IMP_SKILL_SHAPE: invalid skill directory name")
	}
	return Descriptor{Name: name, RootPath: clean, SkillFile: skillFile}, nil
}

func NormalizeName(name string) string {
	name = strings.TrimSpace(strings.ToLower(name))
	name = strings.ReplaceAll(name, " ", "-")
	return name
}
