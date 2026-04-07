package tui

import "testing"

func TestSkipANSI_CSI(t *testing.T) {
	s := "\x1b[92mhello"
	end := skipANSI(s, 0)
	if s[end:] != "hello" {
		t.Errorf("skipANSI CSI: got %q after skip, want %q", s[end:], "hello")
	}
}

func TestSkipANSI_OSC(t *testing.T) {
	// OSC 8 hyperlink terminated by ESC \
	s := "\x1b]8;;http://example.com\x1b\\" + "label"
	end := skipANSI(s, 0)
	if s[end:] != "label" {
		t.Errorf("skipANSI OSC: got %q after skip, want %q", s[end:], "label")
	}
}

func TestAnsiTrimLeft_PlainText(t *testing.T) {
	got := ansiTrimLeft("hello world", 6)
	if got != "world" {
		t.Errorf("got %q, want %q", got, "world")
	}
}

func TestAnsiTrimLeft_WithColor(t *testing.T) {
	// "\x1b[92m" is green CSI sequence (4 bytes), "ab" are visible
	s := "\x1b[92mab\x1b[0mcd"
	// Trim 2 visible chars ("ab") — should land after \x1b[0m or at "cd"
	got := ansiTrimLeft(s, 2)
	if got != "\x1b[0mcd" {
		t.Errorf("got %q, want %q", got, "\x1b[0mcd")
	}
}

func TestAnsiTrimLeft_ZeroN(t *testing.T) {
	s := "\x1b[92mhello"
	got := ansiTrimLeft(s, 0)
	if got != s {
		t.Errorf("trimLeft(0) changed string: got %q", got)
	}
}

func TestAnsiTrimRight_PlainText(t *testing.T) {
	got := ansiTrimRight("hello world", 5)
	if got != "hello" {
		t.Errorf("got %q, want %q", got, "hello")
	}
}

func TestAnsiTrimRight_WithColor(t *testing.T) {
	s := "ab\x1b[92mcd\x1b[0m"
	// Keep 3 visible chars: "abc"
	got := ansiTrimRight(s, 3)
	// Should contain "ab" + "\x1b[92m" + "c" (color starts before d)
	if got != "ab\x1b[92mc" {
		t.Errorf("got %q, want %q", got, "ab\x1b[92mc")
	}
}

func TestAnsiTrimRight_FullString(t *testing.T) {
	s := "\x1b[92mhello\x1b[0m"
	got := ansiTrimRight(s, 100)
	if got != s {
		t.Errorf("trimRight beyond length changed string: got %q", got)
	}
}

func TestAnsiTrimLeft_ThenRight_RoundTrip(t *testing.T) {
	s := "abcdefgh"
	// Take chars [2:5] = "cde"
	got := ansiTrimRight(ansiTrimLeft(s, 2), 3)
	if got != "cde" {
		t.Errorf("round trip: got %q, want %q", got, "cde")
	}
}
