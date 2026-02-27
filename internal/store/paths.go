package store

import "path/filepath"

func StatePath(root string) string {
	return filepath.Join(root, "state.toml")
}

func InstalledRoot(root string) string {
	return filepath.Join(root, "installed")
}

func StagingRoot(root string) string {
	return filepath.Join(root, "staging")
}

func SnapshotRoot(root string) string {
	return filepath.Join(root, "snapshots")
}

func InboxRoot(root string) string {
	return filepath.Join(root, "inbox")
}

func AuditPath(root string) string {
	return filepath.Join(root, "audit.log")
}

func AdapterStateRoot(root string) string {
	return filepath.Join(root, "adapters")
}

func MemoryRoot(root string) string { return filepath.Join(root, "memory") }
func EventLogPath(root string) string {
	return filepath.Join(MemoryRoot(root), "events.jsonl")
}
func FeedbackLogPath(root string) string {
	return filepath.Join(MemoryRoot(root), "feedback.jsonl")
}
func ScoresPath(root string) string { return filepath.Join(MemoryRoot(root), "scores.toml") }
func ConsolidationPath(root string) string {
	return filepath.Join(MemoryRoot(root), "consolidation.toml")
}
func ContextProfilePath(root string) string {
	return filepath.Join(MemoryRoot(root), "context.toml")
}
func LastScanPath(root string) string { return filepath.Join(MemoryRoot(root), "last_scan.toml") }
func ScanStatePath(root string) string {
	return filepath.Join(MemoryRoot(root), "scan_state.toml")
}
