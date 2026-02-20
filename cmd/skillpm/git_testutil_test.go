package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// setupBareRepo creates a local bare git repo with skill files and returns file:// URL.
func setupBareRepo(t *testing.T, skills map[string]map[string]string) string {
	t.Helper()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available on PATH")
	}

	workDir := filepath.Join(t.TempDir(), "work")
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		t.Fatalf("mkdir work failed: %v", err)
	}

	run := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test",
			"GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=test",
			"GIT_COMMITTER_EMAIL=test@test.com",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, string(out))
		}
	}

	run(workDir, "init", "-b", "main")

	for skillName, files := range skills {
		for relPath, content := range files {
			fullPath := filepath.Join(workDir, "skills", skillName, relPath)
			if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
				t.Fatalf("mkdir failed: %v", err)
			}
			if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
				t.Fatalf("write file failed: %v", err)
			}
		}
	}

	run(workDir, "add", "-A")
	run(workDir, "commit", "-m", "initial")

	bareDir := filepath.Join(t.TempDir(), "repo.git")
	run(workDir, "clone", "--bare", workDir, bareDir)

	return "file://" + bareDir
}
