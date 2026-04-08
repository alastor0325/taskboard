package launcher

import (
	"testing"
)

func TestTuiWindowsToKill(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   []string
	}{
		{
			name:   "kills taskboard-tui windows",
			output: "@1 taskboard-tui\n@2 zsh\n@3 taskboard-tui",
			want:   []string{"@1", "@3"},
		},
		{
			name:   "no taskboard-tui windows",
			output: "@1 zsh\n@2 vim",
			want:   nil,
		},
		{
			name:   "empty output",
			output: "",
			want:   nil,
		},
		{
			name:   "single taskboard-tui window",
			output: "@1 taskboard-tui",
			want:   []string{"@1"},
		},
		{
			name:   "malformed lines are skipped",
			output: "\n  \n@2 taskboard-tui",
			want:   []string{"@2"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tuiWindowsToKill(tc.output)
			if len(got) != len(tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("index %d: got %q, want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}
