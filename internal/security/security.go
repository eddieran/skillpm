package security

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"skillpm/internal/config"
)

type Moderation struct {
	IsMalwareBlocked bool
	IsSuspicious     bool
}

type Engine struct {
	strict bool
}

func New(cfg config.SecurityConfig) *Engine {
	return &Engine{strict: strings.EqualFold(cfg.Profile, "strict")}
}

func (e *Engine) CheckTrustTier(tier string) error {
	switch tier {
	case "trusted", "review":
		return nil
	case "untrusted":
		if e.strict {
			return fmt.Errorf("SEC_TRUST_DENY: strict profile denies install from untrusted source")
		}
		return nil
	default:
		return fmt.Errorf("SEC_TRUST_DENY: invalid trust tier %q", tier)
	}
}

func (e *Engine) CheckModeration(mod Moderation, force bool) error {
	if mod.IsMalwareBlocked {
		return fmt.Errorf("SEC_MALWARE_BLOCKED: provider marked skill as blocked malware")
	}
	if mod.IsSuspicious && !force {
		return fmt.Errorf("SEC_SUSPICIOUS_CONFIRM: provider marked skill suspicious; use --force to proceed")
	}
	return nil
}

func SafeJoin(base, rel string) (string, error) {
	if filepath.IsAbs(rel) {
		return "", fmt.Errorf("SEC_PATH_TRAVERSAL: absolute path not allowed")
	}
	cleanRel := filepath.Clean(rel)
	if cleanRel == ".." || strings.HasPrefix(cleanRel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("SEC_PATH_TRAVERSAL: path escapes base")
	}
	joined := filepath.Join(base, cleanRel)
	baseClean := filepath.Clean(base)
	joinedClean := filepath.Clean(joined)
	if joinedClean != baseClean {
		prefix := baseClean + string(filepath.Separator)
		if !strings.HasPrefix(joinedClean, prefix) {
			return "", fmt.Errorf("SEC_PATH_TRAVERSAL: path escapes base")
		}
	}
	return joinedClean, nil
}

// ValidateNoSymlinkPath checks each path component under base and denies symlink traversal.
func ValidateNoSymlinkPath(base, target string) error {
	rel, err := filepath.Rel(base, target)
	if err != nil {
		return fmt.Errorf("SEC_PATH_TRAVERSAL: %w", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("SEC_PATH_TRAVERSAL: path escapes base")
	}
	current := filepath.Clean(base)
	parts := strings.Split(rel, string(filepath.Separator))
	for _, p := range parts {
		if p == "." || p == "" {
			continue
		}
		current = filepath.Join(current, p)
		info, err := os.Lstat(current)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("SEC_SYMLINK_ESCAPE: symlink component %q is not allowed", current)
		}
	}
	return nil
}
