package launcher

import (
	"errors"
	"fmt"
	"testing"
)

// --- tuiPanesToKill ---

func TestTuiPanesToKill(t *testing.T) {
	tests := []struct {
		name   string
		output string
		myPane string
		want   []string
	}{
		{
			name:   "kills taskboard panes excluding own",
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

// --- fmtCmdErr ---

func TestFmtCmdErr(t *testing.T) {
	base := errors.New("exit status 1")

	tests := []struct {
		name    string
		cmdName string
		err     error
		stderr  string
		wantMsg string
	}{
		{
			name:    "no stderr",
			cmdName: "tmux split-window",
			err:     base,
			stderr:  "",
			wantMsg: "tmux split-window: exit status 1",
		},
		{
			name:    "with stderr trimmed",
			cmdName: "tmux split-window",
			err:     base,
			stderr:  "  no server running on /tmp/tmux-1000/default  \n",
			wantMsg: "tmux split-window: exit status 1: no server running on /tmp/tmux-1000/default",
		},
		{
			name:    "stderr whitespace only",
			cmdName: "tmux split-window",
			err:     base,
			stderr:  "   \n\t  ",
			wantMsg: "tmux split-window: exit status 1",
		},
		{
			name:    "zellij variant",
			cmdName: "zellij new-pane",
			err:     base,
			stderr:  "zellij: session not found",
			wantMsg: "zellij new-pane: exit status 1: zellij: session not found",
		},
		{
			name:    "underlying error is wrapped (errors.Is works)",
			cmdName: "tmux split-window",
			err:     base,
			stderr:  "",
			wantMsg: "tmux split-window: exit status 1",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := fmtCmdErr(tc.cmdName, tc.err, tc.stderr)
			if got.Error() != tc.wantMsg {
				t.Errorf("got %q, want %q", got.Error(), tc.wantMsg)
			}
			if !errors.Is(got, tc.err) {
				t.Errorf("errors.Is: wrapped error not reachable via %v", tc.err)
			}
		})
	}
}

// --- Open routing ---

func TestOpenNoMultiplexer(t *testing.T) {
	orig := tmuxAvailable
	tmuxAvailable = func() bool { return false }
	defer func() { tmuxAvailable = orig }()
	t.Setenv("ZELLIJ_SESSION_NAME", "")

	// Should not error — just prints a message when no multiplexer is detected.
	if err := Open("testproj", 50); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFmtCmdErrFormat(t *testing.T) {
	// Verify the exact format strings used in error messages are stable.
	err := fmtCmdErr("tmux split-window", fmt.Errorf("exit status 1"), "session not found")
	want := "tmux split-window: exit status 1: session not found"
	if err.Error() != want {
		t.Errorf("got %q, want %q", err.Error(), want)
	}
}
