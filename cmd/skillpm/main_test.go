package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"skillpm/internal/app"
)

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	fn()
	_ = w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	_ = r.Close()
	return buf.String()
}

func TestNewRootCmdIncludesCoreCommands(t *testing.T) {
	cmd := newRootCmd()
	got := map[string]bool{}
	for _, c := range cmd.Commands() {
		got[c.Name()] = true
	}
	for _, want := range []string{"source", "search", "install", "uninstall", "upgrade", "inject", "remove", "sync", "schedule", "harvest", "validate", "doctor", "self"} {
		if !got[want] {
			t.Fatalf("expected command %q", want)
		}
	}
}

func TestInjectRequiresAgentBeforeService(t *testing.T) {
	called := false
	cmd := newInjectCmd(func() (*app.Service, error) {
		called = true
		return nil, errors.New("should not be called")
	}, boolPtr(false))
	cmd.SetArgs([]string{"demo/skill"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "--agent is required") {
		t.Fatalf("expected agent required error, got %v", err)
	}
	if called {
		t.Fatalf("newSvc should not be called when --agent missing")
	}
}

func TestRemoveRequiresAgentBeforeService(t *testing.T) {
	called := false
	cmd := newRemoveCmd(func() (*app.Service, error) {
		called = true
		return nil, errors.New("should not be called")
	}, boolPtr(false))
	cmd.SetArgs([]string{"demo/skill"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "--agent is required") {
		t.Fatalf("expected agent required error, got %v", err)
	}
	if called {
		t.Fatalf("newSvc should not be called when --agent missing")
	}
}

func TestHarvestRequiresAgentBeforeService(t *testing.T) {
	called := false
	cmd := newHarvestCmd(func() (*app.Service, error) {
		called = true
		return nil, errors.New("should not be called")
	}, boolPtr(false))
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "--agent is required") {
		t.Fatalf("expected agent required error, got %v", err)
	}
	if called {
		t.Fatalf("newSvc should not be called when --agent missing")
	}
}

func TestPrintMessageAndJSON(t *testing.T) {
	msgOut := captureStdout(t, func() {
		if err := print(false, nil, "ok-message"); err != nil {
			t.Fatalf("print message failed: %v", err)
		}
	})
	if !strings.Contains(msgOut, "ok-message") {
		t.Fatalf("expected message output, got %q", msgOut)
	}

	jsonOut := captureStdout(t, func() {
		if err := print(true, map[string]string{"k": "v"}, "ignored"); err != nil {
			t.Fatalf("print json failed: %v", err)
		}
	})
	var parsed map[string]string
	if err := json.Unmarshal([]byte(jsonOut), &parsed); err != nil {
		t.Fatalf("expected valid json output, got %q: %v", jsonOut, err)
	}
	if parsed["k"] != "v" {
		t.Fatalf("unexpected json payload: %+v", parsed)
	}
}

func boolPtr(v bool) *bool { return &v }
