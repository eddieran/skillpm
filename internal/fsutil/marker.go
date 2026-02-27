package fsutil

import "bytes"

// ManagedMarkerPrefix is the prefix for all skillpm ownership markers.
const ManagedMarkerPrefix = "<!-- skillpm:managed"

// ManagedMarkerSimple is the simple marker without attributes.
const ManagedMarkerSimple = "<!-- skillpm:managed -->"

// IsManagedFile checks if data contains a skillpm managed marker.
func IsManagedFile(data []byte) bool {
	return bytes.Contains(data, []byte(ManagedMarkerPrefix))
}
