package tui

import "unicode/utf8"

// skipANSI advances past one ANSI/VT escape sequence starting at position i in s.
// Returns the index of the first byte after the sequence.
func skipANSI(s string, i int) int {
	if i >= len(s) || s[i] != '\x1b' {
		return i + 1
	}
	i++ // skip ESC
	if i >= len(s) {
		return i
	}
	switch s[i] {
	case '[': // CSI: ESC [ <params> <final-letter>
		i++
		for i < len(s) {
			c := s[i]
			i++
			if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') {
				break
			}
		}
	case ']': // OSC: ESC ] ... ST  (ST = ESC \ or BEL)
		i++
		for i < len(s) {
			if s[i] == '\x07' {
				i++
				break
			}
			if s[i] == '\x1b' && i+1 < len(s) && s[i+1] == '\\' {
				i += 2
				break
			}
			i++
		}
	default:
		i++ // other two-byte sequences
	}
	return i
}

// ansiTrimLeft returns s with the first n visible (non-escape) characters removed.
// ANSI escape sequences at or before the cut point are also removed.
func ansiTrimLeft(s string, n int) string {
	skipped := 0
	i := 0
	for i < len(s) && skipped < n {
		if s[i] == '\x1b' {
			i = skipANSI(s, i)
		} else {
			_, size := utf8.DecodeRuneInString(s[i:])
			i += size
			skipped++
		}
	}
	return s[i:]
}

// ansiTrimRight returns s truncated to the first n visible characters,
// preserving ANSI escape sequences within the retained portion.
func ansiTrimRight(s string, n int) string {
	kept := 0
	i := 0
	for i < len(s) {
		if kept >= n {
			break
		}
		if s[i] == '\x1b' {
			i = skipANSI(s, i)
		} else {
			_, size := utf8.DecodeRuneInString(s[i:])
			i += size
			kept++
		}
	}
	return s[:i]
}
