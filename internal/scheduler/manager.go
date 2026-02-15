package scheduler

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type Result struct {
	Backend   string   `json:"backend"`
	Mode      string   `json:"mode"`
	Interval  string   `json:"interval"`
	Installed bool     `json:"installed"`
	Files     []string `json:"files,omitempty"`
	Notes     []string `json:"notes,omitempty"`
}

type Runner interface {
	Run(ctx context.Context, name string, args ...string) error
}

type execRunner struct{}

func (r execRunner) Run(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			return err
		}
		return fmt.Errorf("%w: %s", err, msg)
	}
	return nil
}

type Manager struct {
	home        string
	osName      string
	runner      Runner
	runCommands bool
}

func New() *Manager {
	home, _ := os.UserHomeDir()
	runCommands := true
	if os.Getenv("SKILLPM_SCHEDULER_SKIP_COMMANDS") == "1" {
		runCommands = false
	}
	return &Manager{home: home, osName: runtime.GOOS, runner: execRunner{}, runCommands: runCommands}
}

func (m *Manager) withOverrideRoot() string {
	return os.Getenv("SKILLPM_SCHEDULER_ROOT")
}

func (m *Manager) Install(ctx context.Context, interval string) (Result, error) {
	if interval == "" {
		interval = "6h"
	}
	seconds, err := parseInterval(interval)
	if err != nil {
		return Result{}, fmt.Errorf("SYNC_SCHEDULE_INTERVAL: %w", err)
	}
	switch m.osName {
	case "darwin":
		return m.installLaunchd(ctx, interval, seconds)
	case "linux":
		return m.installSystemd(ctx, interval)
	default:
		return Result{}, fmt.Errorf("SYNC_SCHEDULE_BACKEND: unsupported OS %q", m.osName)
	}
}

func (m *Manager) Remove(ctx context.Context) (Result, error) {
	switch m.osName {
	case "darwin":
		return m.removeLaunchd(ctx)
	case "linux":
		return m.removeSystemd(ctx)
	default:
		return Result{}, fmt.Errorf("SYNC_SCHEDULE_BACKEND: unsupported OS %q", m.osName)
	}
}

func (m *Manager) List() (Result, error) {
	switch m.osName {
	case "darwin":
		return m.listLaunchd(), nil
	case "linux":
		return m.listSystemd(), nil
	default:
		return Result{}, fmt.Errorf("SYNC_SCHEDULE_BACKEND: unsupported OS %q", m.osName)
	}
}

func parseInterval(interval string) (int, error) {
	d, err := time.ParseDuration(interval)
	if err != nil {
		return 0, err
	}
	if d < time.Minute {
		return 0, fmt.Errorf("minimum interval is 1m")
	}
	return int(d.Seconds()), nil
}

func (m *Manager) scheduleExecutable() string {
	if p := os.Getenv("SKILLPM_SCHEDULER_EXEC"); p != "" {
		return p
	}
	exe, err := os.Executable()
	if err != nil {
		return "skillpm"
	}
	return exe
}

func (m *Manager) launchAgentsDir() string {
	if root := m.withOverrideRoot(); root != "" {
		return filepath.Join(root, "LaunchAgents")
	}
	return filepath.Join(m.home, "Library", "LaunchAgents")
}

func (m *Manager) launchdPlistPath() string {
	return filepath.Join(m.launchAgentsDir(), "com.skillpm.sync.plist")
}

func (m *Manager) installLaunchd(ctx context.Context, interval string, seconds int) (Result, error) {
	plist := m.launchdPlistPath()
	if err := os.MkdirAll(filepath.Dir(plist), 0o755); err != nil {
		return Result{}, err
	}
	exe := m.scheduleExecutable()
	content := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key><string>com.skillpm.sync</string>
  <key>ProgramArguments</key>
  <array>
    <string>%s</string>
    <string>sync</string>
  </array>
  <key>StartInterval</key><integer>%d</integer>
  <key>RunAtLoad</key><true/>
  <key>StandardOutPath</key><string>%s</string>
  <key>StandardErrorPath</key><string>%s</string>
</dict>
</plist>
`, xmlEscape(exe), seconds, filepath.Join(m.launchAgentsDir(), "skillpm-sync.log"), filepath.Join(m.launchAgentsDir(), "skillpm-sync.err.log"))
	if err := os.WriteFile(plist, []byte(content), 0o644); err != nil {
		return Result{}, err
	}
	res := Result{Backend: "launchd", Mode: "system", Interval: interval, Installed: true, Files: []string{plist}}
	if m.runCommands && m.withOverrideRoot() == "" {
		_ = m.runner.Run(ctx, "launchctl", "unload", plist)
		if err := m.runner.Run(ctx, "launchctl", "load", plist); err != nil {
			res.Notes = append(res.Notes, "launchctl load failed: "+err.Error())
		}
	} else {
		res.Notes = append(res.Notes, "scheduler commands skipped")
	}
	return res, nil
}

func (m *Manager) removeLaunchd(ctx context.Context) (Result, error) {
	plist := m.launchdPlistPath()
	res := Result{Backend: "launchd", Mode: "off", Installed: false, Files: []string{plist}}
	if m.runCommands && m.withOverrideRoot() == "" {
		_ = m.runner.Run(ctx, "launchctl", "unload", plist)
	} else {
		res.Notes = append(res.Notes, "scheduler commands skipped")
	}
	if err := os.Remove(plist); err != nil && !os.IsNotExist(err) {
		return Result{}, err
	}
	return res, nil
}

func (m *Manager) listLaunchd() Result {
	plist := m.launchdPlistPath()
	_, err := os.Stat(plist)
	installed := err == nil
	res := Result{Backend: "launchd", Installed: installed, Files: []string{plist}, Mode: "off"}
	if !installed {
		return res
	}
	res.Mode = "system"
	if interval, parseErr := launchdIntervalFromPlist(plist); parseErr == nil {
		res.Interval = interval
	}
	return res
}

func (m *Manager) systemdDir() string {
	if root := m.withOverrideRoot(); root != "" {
		return filepath.Join(root, "systemd", "user")
	}
	return filepath.Join(m.home, ".config", "systemd", "user")
}

func (m *Manager) systemdServicePath() string {
	return filepath.Join(m.systemdDir(), "skillpm-sync.service")
}

func (m *Manager) systemdTimerPath() string {
	return filepath.Join(m.systemdDir(), "skillpm-sync.timer")
}

func (m *Manager) installSystemd(ctx context.Context, interval string) (Result, error) {
	dir := m.systemdDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Result{}, err
	}
	exe := m.scheduleExecutable()
	service := fmt.Sprintf(`[Unit]
Description=skillpm sync service

[Service]
Type=oneshot
ExecStart=%s sync
`, shellEscape(exe))
	timer := fmt.Sprintf(`[Unit]
Description=Run skillpm sync every %s

[Timer]
OnBootSec=2m
OnUnitActiveSec=%s
Persistent=true
Unit=skillpm-sync.service

[Install]
WantedBy=timers.target
`, interval, interval)
	servicePath := m.systemdServicePath()
	timerPath := m.systemdTimerPath()
	if err := os.WriteFile(servicePath, []byte(service), 0o644); err != nil {
		return Result{}, err
	}
	if err := os.WriteFile(timerPath, []byte(timer), 0o644); err != nil {
		return Result{}, err
	}
	res := Result{Backend: "systemd", Mode: "system", Interval: interval, Installed: true, Files: []string{servicePath, timerPath}}
	if m.runCommands && m.withOverrideRoot() == "" {
		if err := m.runner.Run(ctx, "systemctl", "--user", "daemon-reload"); err != nil {
			res.Notes = append(res.Notes, "systemctl daemon-reload failed: "+err.Error())
		}
		if err := m.runner.Run(ctx, "systemctl", "--user", "enable", "--now", "skillpm-sync.timer"); err != nil {
			res.Notes = append(res.Notes, "systemctl enable --now failed: "+err.Error())
		}
	} else {
		res.Notes = append(res.Notes, "scheduler commands skipped")
	}
	return res, nil
}

func (m *Manager) removeSystemd(ctx context.Context) (Result, error) {
	servicePath := m.systemdServicePath()
	timerPath := m.systemdTimerPath()
	res := Result{Backend: "systemd", Mode: "off", Installed: false, Files: []string{servicePath, timerPath}}
	if m.runCommands && m.withOverrideRoot() == "" {
		_ = m.runner.Run(ctx, "systemctl", "--user", "disable", "--now", "skillpm-sync.timer")
		_ = m.runner.Run(ctx, "systemctl", "--user", "daemon-reload")
	} else {
		res.Notes = append(res.Notes, "scheduler commands skipped")
	}
	for _, path := range []string{timerPath, servicePath} {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return Result{}, err
		}
	}
	return res, nil
}

func (m *Manager) listSystemd() Result {
	servicePath := m.systemdServicePath()
	timerPath := m.systemdTimerPath()
	_, sErr := os.Stat(servicePath)
	_, tErr := os.Stat(timerPath)
	installed := sErr == nil && tErr == nil
	res := Result{Backend: "systemd", Installed: installed, Files: []string{servicePath, timerPath}, Mode: "off"}
	if !installed {
		return res
	}
	res.Mode = "system"
	if interval, parseErr := systemdIntervalFromTimer(timerPath); parseErr == nil {
		res.Interval = interval
	}
	return res
}

func launchdIntervalFromPlist(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	re := regexp.MustCompile(`<key>StartInterval</key>\s*<integer>(\d+)</integer>`)
	m := re.FindStringSubmatch(string(content))
	if len(m) != 2 {
		return "", fmt.Errorf("StartInterval not found")
	}
	seconds, err := strconv.Atoi(m[1])
	if err != nil {
		return "", err
	}
	return intervalFromSeconds(seconds), nil
}

func systemdIntervalFromTimer(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	re := regexp.MustCompile(`(?m)^OnUnitActiveSec=(.+)$`)
	m := re.FindStringSubmatch(string(content))
	if len(m) != 2 {
		return "", fmt.Errorf("OnUnitActiveSec not found")
	}
	return strings.TrimSpace(m[1]), nil
}

func intervalFromSeconds(seconds int) string {
	if seconds <= 0 {
		return ""
	}
	if seconds%3600 == 0 {
		return fmt.Sprintf("%dh", seconds/3600)
	}
	if seconds%60 == 0 {
		return fmt.Sprintf("%dm", seconds/60)
	}
	return fmt.Sprintf("%ds", seconds)
}

func xmlEscape(v string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", "\"", "&quot;", "'", "&apos;")
	return r.Replace(v)
}

func shellEscape(v string) string {
	if strings.ContainsAny(v, " \t\n\"'") {
		return strconv.Quote(v)
	}
	return v
}
