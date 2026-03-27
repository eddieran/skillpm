package fsutil

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
)

// AtomicWrite writes data to path using a tmp+rename strategy.
// If rename fails, the tmp file is cleaned up.
func AtomicWrite(path string, data []byte, perm os.FileMode) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, perm); err != nil {
		return fmt.Errorf("write tmp: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

// CopyDir copies a directory tree, skipping metadata.toml (skillpm internal).
func CopyDir(src, dst string) error {
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		if rel == "." {
			return nil
		}
		if rel == "metadata.toml" {
			return nil
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
}

// AppendJSONL appends a JSON-encoded line to a file, creating parent dirs as needed.
// The provided mutex is locked for the duration of the write. Pass nil to skip locking.
func AppendJSONL(path string, mu *sync.Mutex, v any) error {
	if mu != nil {
		mu.Lock()
		defer mu.Unlock()
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	blob, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = f.Write(append(blob, '\n'))
	return err
}
