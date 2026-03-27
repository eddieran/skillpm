package audit

import (
	"sync"
	"time"

	"skillpm/internal/fsutil"
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
	return fsutil.AppendJSONL(l.path, &l.mu, ev)
}
