package hooks

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// HookPhase identifies when a hook runs.
type HookPhase string

const (
	PhasePreInstall  HookPhase = "pre_install"
	PhasePostInstall HookPhase = "post_install"
	PhasePreInject   HookPhase = "pre_inject"
	PhasePostInject  HookPhase = "post_inject"
	PhasePreRemove   HookPhase = "pre_remove"
	PhasePostRemove  HookPhase = "post_remove"
)

// Result captures the outcome of a hook execution.
type Result struct {
	Phase   HookPhase
	Command string
	Output  string
	Error   error
}

// Runner executes lifecycle hook commands.
type Runner struct {
	Timeout time.Duration
}

// NewRunner creates a Runner with the given timeout.
func NewRunner(timeout time.Duration) *Runner {
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	return &Runner{Timeout: timeout}
}

// Run executes a list of commands for a given phase.
// Returns on first failure.
func (r *Runner) Run(ctx context.Context, phase HookPhase, commands []string, env map[string]string) ([]Result, error) {
	results := make([]Result, 0, len(commands))
	for _, cmdStr := range commands {
		if strings.TrimSpace(cmdStr) == "" {
			continue
		}
		res := r.runOne(ctx, phase, cmdStr, env)
		results = append(results, res)
		if res.Error != nil {
			return results, fmt.Errorf("HOOK_%s: command %q failed: %w", strings.ToUpper(string(phase)), cmdStr, res.Error)
		}
	}
	return results, nil
}

func (r *Runner) runOne(ctx context.Context, phase HookPhase, cmdStr string, env map[string]string) Result {
	ctx, cancel := context.WithTimeout(ctx, r.Timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", cmdStr)
	for k, v := range env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	out, err := cmd.CombinedOutput()
	return Result{
		Phase:   phase,
		Command: cmdStr,
		Output:  string(out),
		Error:   err,
	}
}
