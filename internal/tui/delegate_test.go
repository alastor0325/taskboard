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

func TestCardBorderStyleColors(t *testing.T) {
	statuses := []string{"failed", "waiting", "running", "done", "idle"}
	for _, status := range statuses {
		t.Run(status, func(t *testing.T) {
			s := cardBorderStyle(status)
			rendered := s.Width(20).Render("test")
			if rendered == "" {
				t.Errorf("cardBorderStyle(%q) rendered empty string", status)
			}
		})
	}
}

func TestCardBorderStyleIsBold(t *testing.T) {
	s := cardBorderStyle("running")
	if !s.GetBold() {
		t.Error("cardBorderStyle should always be bold (selected indicator)")
	}
}

func TestStatusColor(t *testing.T) {
	tests := []struct{ status, want string }{
		{"failed", "196"},
		{"waiting", "214"},
		{"running", "82"},
		{"done", "236"},
		{"idle", "238"},
	}
	for _, tc := range tests {
		t.Run(tc.status, func(t *testing.T) {
			got := statusColor(tc.status)
			if string(got) != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestBuildRow1Note(t *testing.T) {
	task := taskItem{status: "waiting", note: "blocked on review"}
	row := buildRow1(task)
	if !strings.Contains(row, "blocked on review") {
		t.Errorf("buildRow1 with note: got %q", row)
	}
	if !strings.Contains(row, ">>") {
		t.Errorf("buildRow1 waiting note should have >> prefix: got %q", row)
	}
}

func TestBuildRow1WorktreeFallback(t *testing.T) {
	task := taskItem{status: "running", worktree: "/home/user/firefox-123"}
	row := buildRow1(task)
	if !strings.Contains(row, "firefox-123") {
		t.Errorf("buildRow1 worktree fallback: got %q", row)
	}
}

func TestBuildRow1BtwFallback(t *testing.T) {
	task := taskItem{status: "running", btwMsg: "compiling"}
	row := buildRow1(task)
	if !strings.Contains(row, "compiling") {
		t.Errorf("buildRow1 btw fallback: got %q", row)
	}
}

func TestBuildRow1Empty(t *testing.T) {
	task := taskItem{status: "done"}
	row := buildRow1(task)
	if row != "" {
		t.Errorf("buildRow1 with no info: got %q, want empty", row)
	}
}

func TestStatusBadgeUnknown(t *testing.T) {
	got := statusBadge("unknown")
	if got != "UNKNOWN" {
		t.Errorf("statusBadge(%q) = %q, want %q", "unknown", got, "UNKNOWN")
	}
}

func TestIsBugID(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"2026875", true},
		{"1962876", true},
		{"0", true},
		{"", false},
		{"abc", false},
		{"123abc", false},
		{"my-task", false},
	}
	for _, c := range cases {
		if got := isBugID(c.in); got != c.want {
			t.Errorf("isBugID(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}
