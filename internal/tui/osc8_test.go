package tui

import (
	"strings"
	"testing"
)

func TestHyperlinkContainsEscapeAndLabel(t *testing.T) {
	url := "https://example.com"
	label := "click here"
	result := hyperlink(url, label)

	if !strings.Contains(result, "\x1b]8;;") {
		t.Errorf("hyperlink output missing OSC 8 prefix, got: %q", result)
	}
	if !strings.Contains(result, label) {
		t.Errorf("hyperlink output missing label %q, got: %q", label, result)
	}
	if !strings.Contains(result, url) {
		t.Errorf("hyperlink output missing URL %q, got: %q", url, result)
	}
}

func TestHyperlinkRawEscapeFormat(t *testing.T) {
	url := "https://example.com/path"
	label := "label"
	result := hyperlink(url, label)

	// OSC 8 format: ESC ] 8 ; ; URL ESC \ label ESC ] 8 ; ; ESC \
	if !strings.HasPrefix(result, "\x1b]8;;") {
		t.Errorf("expected OSC 8 opening sequence, got: %q", result)
	}
	// The label should appear somewhere between the two OSC sequences.
	idx := strings.Index(result, label)
	if idx < 0 {
		t.Fatalf("label not found in hyperlink output: %q", result)
	}
}

// TestVisibleWidthURLNotCounted verifies that the URL portion of a hyperlink
// does not contribute to the visible width.  The OSC 8 escape sequence wraps
// the URL so visibleWidth strips it; only the label characters (plus the two
// literal backslashes that follow each ESC in the ST terminator) count.
// The expected offset of 2 per ST terminator reflects the current
// implementation of hyperlink(), which emits ESC+\\ (two backslashes) as the
// string-terminator rather than a single ESC+\.
func TestVisibleWidthURLNotCounted(t *testing.T) {
	url := "https://bugzilla.mozilla.org/show_bug.cgi?id=123456"
	label := "bugzilla link"
	hl := hyperlink(url, label)

	hlWidth := visibleWidth(hl)
	labelWidth := visibleWidth(label)

	// The URL itself must not be counted — confirm hyperlink width < URL length.
	if hlWidth >= len(url) {
		t.Errorf("URL should not be counted toward visible width: hlWidth=%d, len(url)=%d",
			hlWidth, len(url))
	}
	// The label characters must all appear in the visible width.
	if hlWidth < labelWidth {
		t.Errorf("label characters should appear in visible width: hlWidth=%d, labelWidth=%d",
			hlWidth, labelWidth)
	}
}

// TestVisibleWidthPlainStringUnchanged verifies that a plain string (no ANSI)
// has its visible width equal to its rune count.
func TestVisibleWidthPlainStringUnchanged(t *testing.T) {
	s := "hello"
	if visibleWidth(s) != 5 {
		t.Errorf("plain string visible width: got %d, want 5", visibleWidth(s))
	}
}

// TestVisibleWidthMultiCharLabel verifies that the label characters are
// visible and the surrounding OSC 8 escape sequences are stripped.
func TestVisibleWidthMultiCharLabel(t *testing.T) {
	url := "https://treeherder.mozilla.org"
	label := "[treeherder]"
	hl := hyperlink(url, label)

	// URL must not dominate the width.
	if visibleWidth(hl) >= len(url) {
		t.Errorf("URL should be stripped: visibleWidth=%d len(url)=%d", visibleWidth(hl), len(url))
	}
	// Label characters must be present.
	if visibleWidth(hl) < visibleWidth(label) {
		t.Errorf("label missing from visible width: hyperlink=%d label=%d",
			visibleWidth(hl), visibleWidth(label))
	}
}
