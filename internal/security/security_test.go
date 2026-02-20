package security

import (
	"context"
	"path/filepath"
	"testing"

	"skillpm/internal/config"
)

func TestSafeJoinPreventsTraversal(t *testing.T) {
	base := filepath.Join(t.TempDir(), "root")
	if _, err := SafeJoin(base, "../../etc/passwd"); err == nil {
		t.Fatalf("expected traversal to fail")
	}
	okPath, err := SafeJoin(base, "skills/foo")
	if err != nil {
		t.Fatalf("expected safe join to succeed: %v", err)
	}
	expected := filepath.Join(base, "skills", "foo")
	if okPath != expected {
		t.Fatalf("unexpected path %q != %q", okPath, expected)
	}
}

func TestModerationPolicy(t *testing.T) {
	engine := New(config.SecurityConfig{Profile: "strict"})
	if err := engine.CheckModeration(Moderation{IsMalwareBlocked: true}, false); err == nil {
		t.Fatalf("expected malware blocked policy to fail")
	}
	if err := engine.CheckModeration(Moderation{IsSuspicious: true}, false); err == nil {
		t.Fatalf("expected suspicious policy to require force")
	}
	if err := engine.CheckModeration(Moderation{IsSuspicious: true}, true); err != nil {
		t.Fatalf("expected suspicious with force to pass: %v", err)
	}
}

func TestTrustTierPolicy(t *testing.T) {
	engine := New(config.SecurityConfig{Profile: "strict"})
	if err := engine.CheckTrustTier("untrusted"); err == nil {
		t.Fatalf("expected strict profile to deny untrusted")
	}
	if err := engine.CheckTrustTier("review"); err != nil {
		t.Fatalf("review should be allowed: %v", err)
	}
}

func TestEngineScannerCreation(t *testing.T) {
	// Scanner nil when scan disabled
	engine := New(config.SecurityConfig{Profile: "strict"})
	if engine.Scanner != nil {
		t.Fatal("expected nil scanner when scan not enabled")
	}

	// Scanner nil when explicitly disabled
	engine = New(config.SecurityConfig{
		Profile: "strict",
		Scan:    config.ScanConfig{Enabled: false},
	})
	if engine.Scanner != nil {
		t.Fatal("expected nil scanner when scan explicitly disabled")
	}

	// Scanner created when enabled
	engine = New(config.SecurityConfig{
		Profile: "strict",
		Scan:    config.ScanConfig{Enabled: true, BlockSeverity: "high"},
	})
	if engine.Scanner == nil {
		t.Fatal("expected scanner to be created when scan enabled")
	}

	// Scanner respects disabled rules (behavioral check)
	engine = New(config.SecurityConfig{
		Profile: "strict",
		Scan: config.ScanConfig{
			Enabled:       true,
			BlockSeverity: "critical",
			DisabledRules: []string{"SCAN_DANGEROUS_PATTERN"},
		},
	})
	if engine.Scanner == nil {
		t.Fatal("expected scanner with disabled rules")
	}
	// Verify disabled rule doesn't produce findings
	report := engine.Scanner.Scan(context.Background(), []SkillContent{{
		SkillRef: "test/rm-check",
		Content:  "# Evil\nrm -rf / all data\n",
	}})
	for _, f := range report.Findings {
		if f.RuleID == "SCAN_DANGEROUS_PATTERN" {
			t.Fatal("disabled rule should not produce findings")
		}
	}
	// Verify block severity is critical (high-severity should pass without force)
	highReport := engine.Scanner.Scan(context.Background(), []SkillContent{{
		SkillRef: "test/high-check",
		Content:  "# Suspicious\nos.environ harvesting\n",
	}})
	if err := engine.Scanner.Enforce(highReport, false); err != nil {
		t.Fatalf("expected high severity to pass when block severity is critical: %v", err)
	}
}
