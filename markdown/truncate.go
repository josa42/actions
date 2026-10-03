// Package markdown contains helpers for GitHub flavored markdown.
package markdown

import (
	"regexp"
	"strings"
	"unicode/utf16"
)

// TruncatedNotice is appended to truncated text.
const TruncatedNotice = "\n\n> [!WARNING]\n> Truncated, the content exceeds the maximum length."

var (
	fenceRe        = regexp.MustCompile("^ {0,3}(`{3,}|~{3,})")
	detailsOpenRe  = regexp.MustCompile(`(?i)<details(?:\s[^>]*)?>`)
	detailsCloseRe = regexp.MustCompile(`(?i)</details\s*>`)
)

// Len returns the length of s as GitHub counts it, in UTF-16 code units.
func Len(s string) int {
	n := 0
	for _, r := range s {
		n += utf16.RuneLen(r)
	}
	return n
}

// Truncate shortens s to at most limit characters (see Len). It cuts at a line
// boundary, closes a code fence and <details> blocks left open by the cut, and
// appends TruncatedNotice. It reports whether s was truncated.
func Truncate(s string, limit int) (string, bool) {
	if Len(s) <= limit {
		return s, false
	}

	budget := limit - Len(TruncatedNotice)
	lines := strings.SplitAfter(s, "\n")

	var st state
	var b strings.Builder
	used := 0
	for _, line := range lines {
		next := st.after(line)
		// Drop the newline of the last kept line; closing adds its own.
		kept := strings.TrimSuffix(line, "\n")
		if used+Len(kept)+Len(next.closing()) > budget {
			break
		}
		b.WriteString(line)
		used += Len(line)
		st = next
	}

	out := strings.TrimRight(b.String(), "\n")
	if out == "" {
		// A single line longer than the budget: cut inside it.
		out = cut(lines[0], budget)
	}
	return out + st.closing() + TruncatedNotice, true
}

// state tracks the open blocks at a line boundary.
type state struct {
	fence   string
	details int
}

func (st state) after(line string) state {
	text := strings.TrimRight(line, "\r\n")

	if st.fence != "" {
		if m := fenceRe.FindStringSubmatch(text); m != nil &&
			m[1][0] == st.fence[0] && len(m[1]) >= len(st.fence) &&
			strings.TrimSpace(text[len(m[0]):]) == "" {
			st.fence = ""
		}
		return st
	}

	if m := fenceRe.FindStringSubmatch(text); m != nil {
		// A backtick fence's info string may not contain backticks.
		if m[1][0] != '`' || !strings.Contains(text[len(m[0]):], "`") {
			st.fence = m[1]
			return st
		}
	}

	st.details += len(detailsOpenRe.FindAllString(text, -1))
	st.details -= len(detailsCloseRe.FindAllString(text, -1))
	st.details = max(st.details, 0)
	return st
}

func (st state) closing() string {
	var b strings.Builder
	if st.fence != "" {
		b.WriteString("\n" + st.fence)
	}
	for range st.details {
		b.WriteString("\n</details>")
	}
	return b.String()
}

func cut(s string, limit int) string {
	n := 0
	for i, r := range s {
		if n+utf16.RuneLen(r) > limit {
			return s[:i]
		}
		n += utf16.RuneLen(r)
	}
	return s
}
