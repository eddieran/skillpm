package security

import (
	"context"
	"fmt"
	"strings"
	"time"

	"skillpm/internal/config"
)

// Severity levels for scan findings, ordered by impact.
type Severity int

const (
	SeverityInfo     Severity = iota // Informational, never blocks
	SeverityLow                      // Minor concern
	SeverityMedium                   // Requires --force to proceed
	SeverityHigh                     // Blocks install by default
	SeverityCritical                 // Always blocks, even with --force
)

func (s Severity) String() string {
	switch s {
	case SeverityInfo:
		return "info"
	case SeverityLow:
		return "low"
	case SeverityMedium:
		return "medium"
	case SeverityHigh:
		return "high"
	case SeverityCritical:
		return "critical"
	default:
		return "unknown"
	}
}

// ParseSeverity converts a severity string to its typed value.
func ParseSeverity(s string) Severity {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "critical":
		return SeverityCritical
	case "high":
		return SeverityHigh
	case "medium":
		return SeverityMedium
	case "low":
		return SeverityLow
	case "info":
		return SeverityInfo
	default:
		return SeverityHigh // safe default
	}
}

// Finding represents a single issue detected by a rule.
type Finding struct {
	RuleID      string   `json:"ruleId"`
	Severity    Severity `json:"severity"`
	SkillRef    string   `json:"skillRef"`
	File        string   `json:"file"`
	Line        int      `json:"line,omitempty"`
	Pattern     string   `json:"pattern,omitempty"`
	Description string   `json:"description"`
}

// ScanReport aggregates all findings across all skills.
type ScanReport struct {
	Skills    []string      `json:"skills"`
	Findings  []Finding     `json:"findings"`
	ScannedAt time.Time     `json:"scannedAt"`
	Duration  time.Duration `json:"duration"`
}

// MaxSeverity returns the highest severity across all findings.
func (r ScanReport) MaxSeverity() Severity {
	max := SeverityInfo
	for _, f := range r.Findings {
		if f.Severity > max {
			max = f.Severity
		}
	}
	return max
}

// FindingsBySkill groups findings by skill ref.
func (r ScanReport) FindingsBySkill() map[string][]Finding {
	out := make(map[string][]Finding)
	for _, f := range r.Findings {
		out[f.SkillRef] = append(out[f.SkillRef], f)
	}
	return out
}

// Rule is the interface all scan rules implement.
type Rule interface {
	ID() string
	Description() string
	Scan(ctx context.Context, skill SkillContent) []Finding
}

// SkillContent is the input to each rule -- flattened view of a resolved skill.
type SkillContent struct {
	SkillRef  string
	Content   string            // SKILL.md content
	Files     map[string]string // ancillary files: relative-path -> content
	Source    string
	TrustTier string
	Version   string
}

// Scanner orchestrates rule execution.
type Scanner struct {
	rules         []Rule
	disabledRules map[string]bool
	blockSeverity Severity
}

// NewScanner creates a scanner with built-in rules.
func NewScanner(cfg config.ScanConfig) *Scanner {
	disabled := make(map[string]bool, len(cfg.DisabledRules))
	for _, id := range cfg.DisabledRules {
		disabled[id] = true
	}
	s := &Scanner{
		disabledRules: disabled,
		blockSeverity: ParseSeverity(cfg.BlockSeverity),
	}
	s.rules = builtinRules()
	return s
}

// Scan runs all enabled rules against each skill.
func (s *Scanner) Scan(ctx context.Context, skills []SkillContent) ScanReport {
	start := time.Now()
	report := ScanReport{
		ScannedAt: start,
	}
	for _, skill := range skills {
		report.Skills = append(report.Skills, skill.SkillRef)
		for _, rule := range s.rules {
			if s.disabledRules[rule.ID()] {
				continue
			}
			findings := rule.Scan(ctx, skill)
			report.Findings = append(report.Findings, findings...)
		}
	}
	report.Duration = time.Since(start)
	return report
}

// Enforce checks the report against policy and returns an error if blocked.
// force=true allows medium severity through but never bypasses critical.
func (s *Scanner) Enforce(report ScanReport, force bool) error {
	max := report.MaxSeverity()
	if max == SeverityCritical {
		return fmt.Errorf("SEC_SCAN_CRITICAL: %s", formatFindings(report, SeverityCritical))
	}
	if max >= s.blockSeverity && !force {
		return fmt.Errorf("SEC_SCAN_BLOCKED: %s; use --force to proceed", formatFindings(report, s.blockSeverity))
	}
	if max >= SeverityMedium && !force {
		return fmt.Errorf("SEC_SCAN_BLOCKED: %s; use --force to proceed", formatFindings(report, SeverityMedium))
	}
	return nil
}

// FormatReport returns a human-readable summary of scan findings.
func FormatReport(report ScanReport) string {
	if len(report.Findings) == 0 {
		return ""
	}
	var b strings.Builder
	bySkill := report.FindingsBySkill()
	for _, skill := range report.Skills {
		findings := bySkill[skill]
		if len(findings) == 0 {
			continue
		}
		fmt.Fprintf(&b, "\nSecurity scan found %d issue(s) in %s:\n", len(findings), skill)
		for _, f := range findings {
			fmt.Fprintf(&b, "\n  %-10s %s\n", strings.ToUpper(f.Severity.String()), f.RuleID)
			if f.File != "" {
				if f.Line > 0 {
					fmt.Fprintf(&b, "            File: %s, Line: %d\n", f.File, f.Line)
				} else {
					fmt.Fprintf(&b, "            File: %s\n", f.File)
				}
			}
			if f.Pattern != "" {
				fmt.Fprintf(&b, "            Pattern: %s\n", f.Pattern)
			}
			fmt.Fprintf(&b, "            %s\n", f.Description)
		}
	}
	return b.String()
}

func formatFindings(report ScanReport, minSeverity Severity) string {
	var parts []string
	for _, f := range report.Findings {
		if f.Severity >= minSeverity {
			desc := f.Description
			if f.File != "" {
				desc = f.File + ": " + desc
			}
			parts = append(parts, fmt.Sprintf("[%s] %s (%s)", strings.ToUpper(f.Severity.String()), f.RuleID, desc))
		}
	}
	count := len(parts)
	if count == 0 {
		return "no findings"
	}
	if count == 1 {
		return parts[0]
	}
	return fmt.Sprintf("%d findings: %s", count, strings.Join(parts, "; "))
}
