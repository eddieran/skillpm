package audit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLogNoopForNilLoggerAndEmptyPath(t *testing.T) {
	var nilLogger *Logger
	if err := nilLogger.Log(Event{Operation: "op"}); err != nil {
		t.Fatalf("nil logger should be noop: %v", err)
	}
	if err := New("").Log(Event{Operation: "op"}); err != nil {
		t.Fatalf("empty-path logger should be noop: %v", err)
	}
}

func TestLogWritesJSONLines(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "audit", "events.log")
	logger := New(logPath)

	first := Event{
		Operation: "install",
		Phase:     "resolve",
		Status:    "ok",
		Code:      "DOC_OK",
		Message:   "resolved",
		Fields: map[string]string{
			"skill": "example/skill",
		},
	}
	second := Event{
		Operation: "install",
		Phase:     "apply",
		Status:    "ok",
	}

	if err := logger.Log(first); err != nil {
		t.Fatalf("log first event: %v", err)
	}
	if err := logger.Log(second); err != nil {
		t.Fatalf("log second event: %v", err)
	}

	blob, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(blob)), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 log lines, got %d", len(lines))
	}

	var gotFirst Event
	if err := json.Unmarshal([]byte(lines[0]), &gotFirst); err != nil {
		t.Fatalf("unmarshal first event: %v", err)
	}
	if gotFirst.Timestamp == "" {
		t.Fatalf("expected timestamp to be set")
	}
	if _, err := time.Parse(time.RFC3339Nano, gotFirst.Timestamp); err != nil {
		t.Fatalf("timestamp should be RFC3339Nano: %v", err)
	}
	if gotFirst.Operation != first.Operation || gotFirst.Phase != first.Phase || gotFirst.Status != first.Status {
		t.Fatalf("unexpected first event body: %+v", gotFirst)
	}
	if gotFirst.Code != first.Code || gotFirst.Message != first.Message {
		t.Fatalf("unexpected first event metadata: %+v", gotFirst)
	}
	if gotFirst.Fields["skill"] != "example/skill" {
		t.Fatalf("unexpected first event fields: %+v", gotFirst.Fields)
	}

	var gotSecond Event
	if err := json.Unmarshal([]byte(lines[1]), &gotSecond); err != nil {
		t.Fatalf("unmarshal second event: %v", err)
	}
	if gotSecond.Operation != second.Operation || gotSecond.Phase != second.Phase || gotSecond.Status != second.Status {
		t.Fatalf("unexpected second event body: %+v", gotSecond)
	}
}

func TestLogMkdirAllFailure(t *testing.T) {
	tmp := t.TempDir()
	blockedPath := filepath.Join(tmp, "blocked")
	if err := os.WriteFile(blockedPath, []byte("x"), 0o644); err != nil {
		t.Fatalf("create blocking file: %v", err)
	}

	logger := New(filepath.Join(blockedPath, "events.log"))
	if err := logger.Log(Event{Operation: "install"}); err == nil {
		t.Fatalf("expected mkdir failure")
	}
}

func TestLogOpenFileFailure(t *testing.T) {
	tmp := t.TempDir()
	dirPath := filepath.Join(tmp, "log-dir")
	if err := os.MkdirAll(dirPath, 0o755); err != nil {
		t.Fatalf("create directory path: %v", err)
	}

	logger := New(dirPath)
	if err := logger.Log(Event{Operation: "install"}); err == nil {
		t.Fatalf("expected open file failure")
	}
}
