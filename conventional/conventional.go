// Package conventional checks commit messages against Conventional Commits
// 1.0.0, see https://www.conventionalcommits.org/en/v1.0.0/.
package conventional

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

// DefaultTypes are the commit types allowed by default.
var DefaultTypes = []string{"feat", "fix", "docs", "refactor", "test", "chore", "build", "ci", "perf", "style"}

// Config holds the rules on top of the specification.
type Config struct {
	// Types allowed. Empty means DefaultTypes.
	Types []string
	// Scopes allowed. Empty allows any scope. A scope is always optional.
	Scopes []string
	// MaxLength of the header in characters. 0 disables the check.
	MaxLength int
	// Lowercase requires the description to start with a lowercase letter.
	Lowercase bool
	// NoPeriod forbids a period at the end of the description.
	NoPeriod bool
}

var (
	headerRe   = regexp.MustCompile(`^(\w+)(?:\(([^()]*)\))?(!)?:(\s*)(.*)$`)
	revertRe   = regexp.MustCompile(`^(?:Revert|Reapply) "(.*)"$`)
	autosquash = regexp.MustCompile(`^(?:fixup|squash|amend)! `)
	breakingRe = regexp.MustCompile(`(?i)^breaking[ -]change:`)
)

// Check returns the rules message breaks, or nil if it is valid. git's
// default revert message, Revert "<header>", is valid if the quoted header
// is.
func Check(message string, cfg Config) []string {
	lines := strings.Split(strings.ReplaceAll(strings.TrimSpace(message), "\r\n", "\n"), "\n")
	header := strings.TrimSpace(lines[0])

	var v []string
	if len(lines) > 1 && strings.TrimSpace(lines[1]) != "" {
		v = append(v, "a blank line must separate the header from the body")
	}
	for _, line := range lines[1:] {
		if m := breakingRe.FindString(line); m != "" && m != "BREAKING CHANGE:" && m != "BREAKING-CHANGE:" {
			v = append(v, "BREAKING CHANGE must be uppercase")
		}
	}

	if m := revertRe.FindStringSubmatch(header); m != nil {
		return append(v, CheckHeader(m[1], cfg)...)
	}
	return append(v, CheckHeader(header, cfg)...)
}

// CheckHeader checks a single line, such as a pull request title.
func CheckHeader(header string, cfg Config) []string {
	types := cfg.Types
	if len(types) == 0 {
		types = DefaultTypes
	}

	if autosquash.MatchString(header) {
		return []string{"fixup!, squash! and amend! commits must be squashed before merging"}
	}

	idx := headerRe.FindStringSubmatchIndex(header)
	if idx == nil {
		return []string{fmt.Sprintf("the header must be type(scope): description, with type one of %s", strings.Join(types, ", "))}
	}
	group := func(i int) string {
		if idx[2*i] < 0 {
			return ""
		}
		return header[idx[2*i]:idx[2*i+1]]
	}
	typ, scope, hasScope, space, desc := group(1), group(2), idx[4] >= 0, group(4), group(5)

	var v []string
	if !slices.Contains(types, typ) {
		v = append(v, fmt.Sprintf("type %q is not one of %s", typ, strings.Join(types, ", ")))
	}
	switch {
	case hasScope && strings.TrimSpace(scope) == "":
		v = append(v, "the scope is empty")
	case hasScope && len(cfg.Scopes) > 0 && !slices.Contains(cfg.Scopes, scope):
		v = append(v, fmt.Sprintf("scope %q is not one of %s", scope, strings.Join(cfg.Scopes, ", ")))
	}
	if space != " " {
		v = append(v, "a single space must follow the colon")
	}

	if strings.TrimSpace(desc) == "" {
		return append(v, "the description is empty")
	}
	if r, _ := utf8.DecodeRuneInString(desc); cfg.Lowercase && unicode.IsUpper(r) {
		v = append(v, "the description must start with a lowercase letter")
	}
	if cfg.NoPeriod && strings.HasSuffix(desc, ".") {
		v = append(v, "the description must not end with a period")
	}
	if n := utf8.RuneCountInString(header); cfg.MaxLength > 0 && n > cfg.MaxLength {
		v = append(v, fmt.Sprintf("the header is %d characters, at most %d are allowed", n, cfg.MaxLength))
	}
	return v
}
