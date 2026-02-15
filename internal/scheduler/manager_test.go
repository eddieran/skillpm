package scheduler

import (
	"context"
	"os"
	"runtime"
	"testing"
)

func TestInstallListRemove(t *testing.T) {
	root := t.TempDir()
	t.Setenv("SKILLPM_SCHEDULER_ROOT", root)
	t.Setenv("SKILLPM_SCHEDULER_SKIP_COMMANDS", "1")
	m := New()

	res, err := m.Install(context.Background(), "2h")
	if err != nil {
		t.Fatalf("install failed: %v", err)
	}
	if !res.Installed || len(res.Files) == 0 {
		t.Fatalf("unexpected install result: %+v", res)
	}
	for _, path := range res.Files {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected schedule file %s to exist: %v", path, err)
		}
	}

	listed, err := m.List()
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if !listed.Installed {
		t.Fatalf("expected schedule to be installed")
	}
	if listed.Mode != "system" {
		t.Fatalf("expected list mode=system, got %q", listed.Mode)
	}
	if listed.Interval == "" {
		t.Fatalf("expected list interval to be populated")
	}

	removed, err := m.Remove(context.Background())
	if err != nil {
		t.Fatalf("remove failed: %v", err)
	}
	if removed.Installed {
		t.Fatalf("expected remove result installed=false")
	}
	for _, path := range removed.Files {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("expected schedule file removed: %s", path)
		}
	}
}

func TestIntervalValidation(t *testing.T) {
	m := New()
	_, err := m.Install(context.Background(), "10s")
	if runtime.GOOS == "darwin" || runtime.GOOS == "linux" {
		if err == nil {
			t.Fatalf("expected invalid interval error")
		}
	}
}
