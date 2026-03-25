package e2e

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestUnattendedPreflightReportsMissingGoToolchain(t *testing.T) {
	stubDir := t.TempDir()
	writeExecutable(t, stubDir, "gh", successGHStub)
	writeExecutable(t, stubDir, "python3", successPythonProbeStub)

	repoDir := createTempGoRepo(t)
	out, err := runPreflightScript(t, stubDir, repoDir, "--no-default-hosts", "--host", "github.com")
	if err == nil {
		t.Fatalf("expected preflight to fail when go toolchain is missing\noutput=%s", out)
	}

	assertContains(t, out, ">>> binary:go")
	assertContains(t, out, "go not found on PATH")
	assertContains(t, out, ">>> binary:gofmt")
	assertContains(t, out, "gofmt not found on PATH")
	assertContains(t, out, "PREFLIGHT_STATUS=fail")
	assertContains(t, out, "PREFLIGHT_FAILURES=missing-go,missing-gofmt")
}

func TestUnattendedPreflightReportsInvalidGitHubAuth(t *testing.T) {
	stubDir := t.TempDir()
	writeExecutable(t, stubDir, "go", successToolStub)
	writeExecutable(t, stubDir, "gofmt", successToolStub)
	writeExecutable(t, stubDir, "gh", invalidGHStub)
	writeExecutable(t, stubDir, "python3", successPythonProbeStub)

	repoDir := createTempGoRepo(t)
	out, err := runPreflightScript(t, stubDir, repoDir, "--no-default-hosts", "--host", "github.com")
	if err == nil {
		t.Fatalf("expected preflight to fail when gh auth is invalid\noutput=%s", out)
	}

	assertContains(t, out, ">>> gh-auth")
	assertContains(t, out, "token in default is invalid")
	assertContains(t, out, "PREFLIGHT_STATUS=fail")
	assertContains(t, out, "PREFLIGHT_FAILURES=gh-auth")
}

func TestUnattendedPreflightReportsHostReachabilityFailure(t *testing.T) {
	stubDir := t.TempDir()
	writeExecutable(t, stubDir, "go", successToolStub)
	writeExecutable(t, stubDir, "gofmt", successToolStub)
	writeExecutable(t, stubDir, "gh", successGHStub)
	writeExecutable(t, stubDir, "python3", failingPythonProbeStub)

	repoDir := createTempGoRepo(t)
	out, err := runPreflightScript(t, stubDir, repoDir, "--no-default-hosts", "--host", "github.com")
	if err == nil {
		t.Fatalf("expected preflight to fail when host reachability fails\noutput=%s", out)
	}

	assertContains(t, out, ">>> host:github.com")
	assertContains(t, out, "tcp connect failed for github.com:443")
	assertContains(t, out, "PREFLIGHT_STATUS=fail")
	assertContains(t, out, "PREFLIGHT_FAILURES=host:github.com")
}

func TestUnattendedPreflightPassesWhenChecksSucceed(t *testing.T) {
	stubDir := t.TempDir()
	writeExecutable(t, stubDir, "go", successToolStub)
	writeExecutable(t, stubDir, "gofmt", successToolStub)
	writeExecutable(t, stubDir, "gh", successGHStub)
	writeExecutable(t, stubDir, "python3", successPythonProbeStub)

	repoDir := createTempGoRepo(t)
	out, err := runPreflightScript(t, stubDir, repoDir, "--no-default-hosts", "--host", "github.com")
	if err != nil {
		t.Fatalf("expected preflight to pass, got error: %v\noutput=%s", err, out)
	}

	assertContains(t, out, "PREFLIGHT_STATUS=pass")
	assertContains(t, out, "PREFLIGHT_FAILURES=none")
}

func createTempGoRepo(t *testing.T) string {
	t.Helper()

	repoDir := t.TempDir()
	goMod := "module example.com/preflight\n\ngo 1.26.1\n"
	if err := os.WriteFile(filepath.Join(repoDir, "go.mod"), []byte(goMod), 0o644); err != nil {
		t.Fatalf("write go.mod failed: %v", err)
	}
	return repoDir
}

func runPreflightScript(t *testing.T, stubDir, repoDir string, args ...string) (string, error) {
	t.Helper()

	script := filepath.Join(repoRoot(t), "tools", "unattended-preflight.sh")
	cmdArgs := append([]string{script, "--repo-root", repoDir}, args...)
	cmd := exec.Command("/bin/bash", cmdArgs...)
	cmd.Dir = repoRoot(t)
	cmd.Env = mergeEnv(os.Environ(), map[string]string{
		"PATH": stubDir + string(os.PathListSeparator) + preflightUtilityPath(t),
	})
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func preflightUtilityPath(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	for _, name := range []string{"awk", "cksum", "tr", "uname"} {
		target, err := exec.LookPath(name)
		if err != nil {
			t.Fatalf("look up %s failed: %v", name, err)
		}
		if err := os.Symlink(target, filepath.Join(dir, name)); err != nil {
			t.Fatalf("symlink %s failed: %v", name, err)
		}
	}
	return dir
}

func writeExecutable(t *testing.T, dir, name, body string) {
	t.Helper()

	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatalf("write executable %s failed: %v", name, err)
	}
}

const successToolStub = `#!/bin/bash
exit 0
`

const successGHStub = `#!/bin/bash
if [ "$1" = "auth" ] && [ "$2" = "status" ]; then
  echo "github.com"
  echo "  ✓ Logged in to github.com account preflight-test"
  exit 0
fi
echo "unexpected gh args: $*" >&2
exit 2
`

const invalidGHStub = `#!/bin/bash
if [ "$1" = "auth" ] && [ "$2" = "status" ]; then
  echo "github.com"
  echo "  X The token in default is invalid."
  exit 1
fi
echo "unexpected gh args: $*" >&2
exit 2
`

const successPythonProbeStub = `#!/bin/bash
host="${!#}"
if [[ "$host" != *:* ]]; then
  host="${host}:443"
fi
echo "resolved ${host} -> 127.0.0.1"
echo "tcp connect ok ${host}"
`

const failingPythonProbeStub = `#!/bin/bash
host="${!#}"
if [[ "$host" != *:* ]]; then
  host="${host}:443"
fi
echo "resolved ${host} -> 127.0.0.1"
echo "tcp connect failed for ${host}: connection refused"
exit 1
`
