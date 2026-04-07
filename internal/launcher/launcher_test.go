package launcher

import (
	"testing"
)

func TestTuiPanesToKill(t *testing.T) {
	tests := []struct {
		name   string
		output string
		myPane string
		want   []string
	}{
		{
			name: "kills taskboard panes excluding own",
			output: "%3 taskboard\n%4 zsh\n%5 taskboard\n%6 vim",
			myPane: "%3",
			want:   []string{"%5"},
		},
		{
			name:   "no taskboard panes",
			output: "%1 zsh\n%2 vim",
			myPane: "%1",
			want:   nil,
		},
		{
			name:   "empty output",
			output: "",
			myPane: "%1",
			want:   nil,
		},
		{
			name:   "own pane is the only taskboard pane",
			output: "%1 taskboard",
			myPane: "%1",
			want:   nil,
		},
		{
			name:   "multiple taskboard panes none are own",
			output: "%2 taskboard\n%3 taskboard",
			myPane: "%1",
			want:   []string{"%2", "%3"},
		},
		{
			name:   "malformed lines are skipped",
			output: "\n  \n%2 taskboard",
			myPane: "%1",
			want:   []string{"%2"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tuiPanesToKill(tc.output, tc.myPane)
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
