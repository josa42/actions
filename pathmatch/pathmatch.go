// Package pathmatch matches repository paths against glob patterns, following
// the rules of the paths filter of GitHub workflow triggers:
//
//   - *  matches any characters except /
//   - ** matches any characters including /
//   - ?  matches one character except /
//   - [abc], [a-z], [!abc] match one character of a class
//   - \  escapes the next character
//   - a leading ! negates a pattern
//   - later patterns override earlier ones
//
// In addition:
//
//   - a pattern without wildcards matches the path itself and everything
//     below it, as does a pattern with a trailing /
//   - a leading ./ or / is ignored
//   - dot files are matched like any other file
package pathmatch

import (
	"fmt"
	"regexp"
	"strings"
)

// Matcher is a compiled list of patterns.
type Matcher struct {
	rules []rule
}

type rule struct {
	negate bool
	re     *regexp.Regexp
}

// Compile compiles patterns. Empty patterns are ignored.
func Compile(patterns []string) (*Matcher, error) {
	m := &Matcher{}
	for _, p := range patterns {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}

		negate := strings.HasPrefix(p, "!")
		re, err := compile(strings.TrimPrefix(p, "!"))
		if err != nil {
			return nil, fmt.Errorf("pattern %q: %w", p, err)
		}
		m.rules = append(m.rules, rule{negate, re})
	}
	return m, nil
}

// Empty reports whether the matcher has no patterns.
func (m *Matcher) Empty() bool {
	return len(m.rules) == 0
}

// Match reports whether path matches. An empty matcher matches every path.
// If the first pattern is negated, paths start out matched, so a list of
// only negated patterns excludes from everything.
func (m *Matcher) Match(path string) bool {
	if m.Empty() {
		return true
	}

	path = normalize(path)
	matched := m.rules[0].negate
	for _, r := range m.rules {
		if r.re.MatchString(path) {
			matched = !r.negate
		}
	}
	return matched
}

// Filter returns the paths that match.
func (m *Matcher) Filter(paths []string) []string {
	var out []string
	for _, p := range paths {
		if m.Match(p) {
			out = append(out, p)
		}
	}
	return out
}

func normalize(p string) string {
	p = strings.TrimPrefix(p, "./")
	return strings.TrimPrefix(p, "/")
}

func compile(pattern string) (*regexp.Regexp, error) {
	p := normalize(pattern)
	if p == "" {
		return nil, fmt.Errorf("empty pattern")
	}

	dir := strings.HasSuffix(p, "/") || !hasWildcard(p)
	r := []rune(strings.TrimSuffix(p, "/"))

	var re strings.Builder
	re.WriteString("^")
	for i := 0; i < len(r); i++ {
		switch r[i] {
		case '*':
			if i+1 < len(r) && r[i+1] == '*' {
				i++
				// "**/" also matches zero directories.
				if i+1 < len(r) && r[i+1] == '/' {
					i++
					re.WriteString("(?:.*/)?")
				} else {
					re.WriteString(".*")
				}
			} else {
				re.WriteString("[^/]*")
			}
		case '?':
			re.WriteString("[^/]")
		case '[':
			end, class, err := charClass(r, i)
			if err != nil {
				return nil, err
			}
			re.WriteString(class)
			i = end
		case '\\':
			if i+1 == len(r) {
				return nil, fmt.Errorf("trailing \\")
			}
			i++
			re.WriteString(quote(r[i]))
		default:
			re.WriteString(quote(r[i]))
		}
	}
	if dir {
		re.WriteString("(?:/.*)?")
	}
	re.WriteString("$")

	return regexp.Compile(re.String())
}

func hasWildcard(p string) bool {
	return strings.ContainsAny(p, "*?[")
}

func quote(c rune) string {
	return regexp.QuoteMeta(string(c))
}

// charClass translates the class starting at r[start] == '[' and returns the
// index of its closing ']'. Classes never match /.
func charClass(r []rune, start int) (int, string, error) {
	var b strings.Builder
	b.WriteString("[")

	i := start + 1
	negate := i < len(r) && (r[i] == '!' || r[i] == '^')
	if negate {
		b.WriteString("^/")
		i++
	}
	for first := true; i < len(r); i, first = i+1, false {
		switch c := r[i]; {
		case c == ']' && !first:
			b.WriteString("]")
			return i, b.String(), nil
		case c == '/':
			return 0, "", fmt.Errorf("/ in character class")
		case c == '-':
			b.WriteString("-")
		case c == '\\' && i+1 < len(r):
			i++
			b.WriteString(quote(r[i]))
		default:
			b.WriteString(quote(c))
		}
	}
	return 0, "", fmt.Errorf("unterminated [")
}
