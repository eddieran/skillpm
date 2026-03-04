package hooks

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestRunner_Run_Success(t *testing.T) {
	r := NewRunner(5 * time.Second)
	results, err := r.Run(context.Background(), PhasePostInstall, []string{"echo hello"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Error != nil {
		t.Errorf("unexpected error: %v", results[0].Error)
	}
}

func TestRunner_Run_Failure(t *testing.T) {
	r := NewRunner(5 * time.Second)
	_, err := r.Run(context.Background(), PhasePreInstall, []string{"false"}, nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRunner_Run_EmptyCommands(t *testing.T) {
	r := NewRunner(5 * time.Second)
	results, err := r.Run(context.Background(), PhasePostInstall, []string{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestRunner_Run_SkipsBlankCommands(t *testing.T) {
	r := NewRunner(5 * time.Second)
	results, err := r.Run(context.Background(), PhasePostInstall, []string{"", "  ", "echo ok"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result (only echo ok), got %d", len(results))
	}
}

func TestRunner_Run_StopsOnFirstFailure(t *testing.T) {
	r := NewRunner(5 * time.Second)
	_, err := r.Run(context.Background(), PhasePreInstall, []string{"echo first", "false", "echo never"}, nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRunner_Run_Timeout(t *testing.T) {
	r := NewRunner(100 * time.Millisecond)
	_, err := r.Run(context.Background(), PhasePreInstall, []string{"sleep 10"}, nil)
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestRunner_Run_WithEnv(t *testing.T) {
	r := NewRunner(5 * time.Second)
	env := map[string]string{"SKILLPM_TEST_VAR": "hello"}
	results, err := r.Run(context.Background(), PhasePostInstall, []string{"echo $SKILLPM_TEST_VAR"}, env)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if got := strings.TrimSpace(results[0].Output); got != "hello" {
		t.Errorf("expected output %q, got %q", "hello", got)
	}
}

func TestNewRunner_DefaultTimeout(t *testing.T) {
	r := NewRunner(0)
	if r.Timeout != 30*time.Second {
		t.Errorf("expected 30s default timeout, got %v", r.Timeout)
	}
}
