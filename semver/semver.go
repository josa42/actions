// Package semver handles MAJOR.MINOR.PATCH versions.
package semver

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Version is a MAJOR.MINOR.PATCH version.
type Version struct {
	Major, Minor, Patch int
}

var versionRe = regexp.MustCompile(`^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)$`)

// Parse parses a MAJOR.MINOR.PATCH version without prefix or pre-release.
func Parse(s string) (Version, error) {
	m := versionRe.FindStringSubmatch(s)
	if m == nil {
		return Version{}, fmt.Errorf("%q is not a MAJOR.MINOR.PATCH version", s)
	}
	var v Version
	for i, p := range []*int{&v.Major, &v.Minor, &v.Patch} {
		n, err := strconv.Atoi(m[i+1])
		if err != nil {
			return Version{}, fmt.Errorf("%q: %w", s, err)
		}
		*p = n
	}
	return v, nil
}

func (v Version) String() string {
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
}

// Compare returns -1, 0 or 1 if v is lower than, equal to or higher than o.
func (v Version) Compare(o Version) int {
	for _, d := range []int{v.Major - o.Major, v.Minor - o.Minor, v.Patch - o.Patch} {
		if d < 0 {
			return -1
		}
		if d > 0 {
			return 1
		}
	}
	return 0
}

// Bump returns the next major, minor or patch version.
func (v Version) Bump(part string) (Version, error) {
	switch part {
	case "major":
		return Version{v.Major + 1, 0, 0}, nil
	case "minor":
		return Version{v.Major, v.Minor + 1, 0}, nil
	case "patch":
		return Version{v.Major, v.Minor, v.Patch + 1}, nil
	}
	return Version{}, fmt.Errorf("%q is not major, minor or patch", part)
}

// Latest returns the highest version among tags that match format, a tag
// name with a {version} placeholder such as v{version}. ok is false if no tag
// matches.
func Latest(tags []string, format string) (latest Version, ok bool) {
	prefix, suffix, found := strings.Cut(format, "{version}")
	if !found {
		return Version{}, false
	}
	for _, tag := range tags {
		s, hasPrefix := strings.CutPrefix(tag, prefix)
		s, hasSuffix := strings.CutSuffix(s, suffix)
		if !hasPrefix || !hasSuffix {
			continue
		}
		if v, err := Parse(s); err == nil && (!ok || v.Compare(latest) > 0) {
			latest, ok = v, true
		}
	}
	return latest, ok
}

// Next resolves input, an explicit version or major, minor or patch, against
// the current version. Explicit versions must be higher than current. With
// no current version, bumps start from 0.0.0.
func Next(input string, current Version, hasCurrent bool) (Version, error) {
	switch input {
	case "major", "minor", "patch":
		return current.Bump(input)
	}

	v, err := Parse(input)
	if err != nil {
		return Version{}, fmt.Errorf("%w, or major, minor or patch", err)
	}
	if hasCurrent && v.Compare(current) <= 0 {
		return Version{}, fmt.Errorf("version %s is not higher than the latest release %s", v, current)
	}
	return v, nil
}
