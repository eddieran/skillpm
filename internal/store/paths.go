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
