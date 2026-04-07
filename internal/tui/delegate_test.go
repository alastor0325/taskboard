package tui

import (
	"strings"
	"testing"
)

func TestStatusBadge(t *testing.T) {
	tests := []struct {
		status  string
		wantSym string
	}{
		{"running", "▶"},
		{"waiting", "⏸"},
		{"done", "✓"},
		{"idle", "·"},
		{"failed", "✗"},
	}
	for _, tc := range tests {
		t.Run(tc.status, func(t *testing.T) {
			got := statusBadge(tc.status)
			if !strings.Contains(got, tc.wantSym) {
				t.Errorf("statusBadge(%q) = %q, want symbol %q", tc.status, got, tc.wantSym)
			}
			if !strings.Contains(got, strings.ToUpper(tc.status)) {
				t.Errorf("statusBadge(%q) = %q, want uppercase status text", tc.status, got)
			}
		})
	}
}

func TestStatusBadgeUnknown(t *testing.T) {
	got := statusBadge("unknown")
	if got != "UNKNOWN" {
		t.Errorf("statusBadge(%q) = %q, want %q", "unknown", got, "UNKNOWN")
	}
}
