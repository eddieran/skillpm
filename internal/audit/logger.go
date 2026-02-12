package audit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Logger struct {
	path string
	mu   sync.Mutex
}

type Event struct {
	Timestamp string            `json:"timestamp"`
	Operation string            `json:"operation"`
	Phase     string            `json:"phase"`
	Status    string            `json:"status"`
	Code      string            `json:"code,omitempty"`
	Message   string            `json:"message,omitempty"`
	Fields    map[string]string `json:"fields,omitempty"`
}

func New(path string) *Logger {
	return &Logger{path: path}
}

func (l *Logger) Log(ev Event) error {
	if l == nil || l.path == "" {
		return nil
	}
	ev.Timestamp = time.Now().UTC().Format(time.RFC3339Nano)
	blob, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(l.path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(l.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Write(append(blob, '\n')); err != nil {
		return err
	}
	return nil
}
