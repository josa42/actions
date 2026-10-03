// Package changelog edits markdown changelogs with one "## <version>" section
// per release, such as Keep a Changelog.
package changelog

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var (
	unreleasedRe = regexp.MustCompile(`(?im)^##[ \t]+(\[)?unreleased(\])?[ \t\r]*$`)
	sectionRe    = regexp.MustCompile(`(?m)^##[ \t]`)
)

// Promote turns the "## Unreleased" section into "## <version> - <date>".
// "## [Unreleased]" becomes "## [<version>] - <date>". It returns the new
// changelog and the body of the section, trimmed.
//
// It fails if there is no Unreleased section, more than one, if it is
// empty, or if the changelog already has a section for version.
func Promote(content, version, date string) (string, string, error) {
	matches := unreleasedRe.FindAllStringSubmatchIndex(content, -1)
	switch len(matches) {
	case 0:
		return "", "", errors.New("no ## Unreleased section")
	case 1:
	default:
		return "", "", fmt.Errorf("%d ## Unreleased sections", len(matches))
	}

	versionRe := regexp.MustCompile(`(?m)^##[ \t]+\[?` + regexp.QuoteMeta(version) + `(?:[^0-9A-Za-z.+-]|$)`)
	if versionRe.MatchString(content) {
		return "", "", fmt.Errorf("already has a ## %s section", version)
	}

	m := matches[0]
	start, end := m[0], m[1]

	bodyEnd := len(content)
	if loc := sectionRe.FindStringIndex(content[end:]); loc != nil {
		bodyEnd = end + loc[0]
	}
	notes := strings.TrimSpace(content[end:bodyEnd])
	if notes == "" {
		return "", "", errors.New("the ## Unreleased section is empty")
	}

	heading := "## " + version + " - " + date
	if m[2] >= 0 && m[4] >= 0 {
		heading = "## [" + version + "] - " + date
	}
	if strings.HasSuffix(content[start:end], "\r") {
		heading += "\r"
	}

	return content[:start] + heading + content[end:], notes, nil
}
