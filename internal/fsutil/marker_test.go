package fsutil

import "testing"

func TestIsManagedFile(t *testing.T) {
	tests := []struct {
		name string
		data string
		want bool
	}{
		{"simple marker", "some text\n<!-- skillpm:managed -->\n", true},
		{"marker with attrs", "<!-- skillpm:managed ref=x checksum=y -->\n", true},
		{"no marker", "regular markdown content", false},
		{"empty", "", false},
		{"partial match", "<!-- skillpm:manage", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsManagedFile([]byte(tt.data)); got != tt.want {
				t.Errorf("IsManagedFile = %v, want %v", got, tt.want)
			}
		})
	}
}
