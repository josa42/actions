// Package versionfile updates versions in files of any format, using
// templates such as `"version": "{version}"`.
package versionfile

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

const placeholder = "{version}"

// File is a file with a version to update.
type File struct {
	Path string
	// Template is the text around the version, with a {version}
	// placeholder. It matches from the start of a line, after indentation,
	// and may be followed by more text on that line.
	Template string
}

// ParseList parses "path: template" lines. Blank lines are ignored.
func ParseList(lines []string) ([]File, error) {
	var files []File
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		path, template, ok := strings.Cut(line, ": ")
		path, template = strings.TrimSpace(path), strings.TrimSpace(template)
		if !ok || path == "" {
			return nil, fmt.Errorf("%q is not path: template", line)
		}
		if strings.Count(template, placeholder) != 1 {
			return nil, fmt.Errorf("%q: the template needs exactly one %s", line, placeholder)
		}
		files = append(files, File{path, template})
	}
	return files, nil
}

// Replace sets the version in content. The template must match exactly
// once. It returns the new content and the version it replaced.
func Replace(content, template, version string) (string, string, error) {
	prefix, suffix, ok := strings.Cut(template, placeholder)
	if !ok {
		return "", "", fmt.Errorf("template %q has no %s", template, placeholder)
	}

	re := regexp.MustCompile(`(?m)^[ \t]*` + regexp.QuoteMeta(prefix) + `([0-9A-Za-z.+_-]*)` + regexp.QuoteMeta(suffix))
	matches := re.FindAllStringSubmatchIndex(content, -1)
	if len(matches) != 1 {
		return "", "", fmt.Errorf("template %q matches %d times, expected once", template, len(matches))
	}

	start, end := matches[0][2], matches[0][3]
	return content[:start] + version + content[end:], content[start:end], nil
}

var indentRe = regexp.MustCompile(`(?m)^([ \t]+)"`)

// ReplaceNPMLock sets the version in a package-lock.json: the top-level
// "version" and the one of the root package, packages[""]. The versions of
// dependencies are left alone. It returns the new content and the top-level
// version it replaced.
func ReplaceNPMLock(content, version string) (string, string, error) {
	// npm keeps the indentation of package.json, two spaces by default.
	m := indentRe.FindStringSubmatch(content)
	if m == nil {
		return "", "", errors.New("not a package-lock.json")
	}
	indent := regexp.QuoteMeta(m[1])

	content, old, err := replaceOnce(content, `(?m)^`+indent+`"version": "([0-9A-Za-z.+_-]*)"`, version, `the top-level "version"`)
	if err != nil {
		return "", "", err
	}

	// The root package runs from `"": {` to the closing brace at the same
	// indentation.
	entries := regexp.MustCompile(`(?m)^`+indent+indent+`"": \{\r?$`).FindAllStringIndex(content, -1)
	if len(entries) != 1 {
		return "", "", fmt.Errorf(`packages[""] found %d times, expected once`, len(entries))
	}
	start := entries[0][1]
	end := len(content)
	if loc := regexp.MustCompile(`(?m)^` + indent + indent + `\}`).FindStringIndex(content[start:]); loc != nil {
		end = start + loc[0]
	}
	entry, _, err := replaceOnce(content[start:end], `(?m)^`+indent+indent+indent+`"version": "([0-9A-Za-z.+_-]*)"`, version, `the "version" of packages[""]`)
	if err != nil {
		return "", "", err
	}
	return content[:start] + entry + content[end:], old, nil
}

// replaceOnce replaces the first group of pattern, which must match exactly
// once, with version.
func replaceOnce(content, pattern, version, what string) (string, string, error) {
	matches := regexp.MustCompile(pattern).FindAllStringSubmatchIndex(content, -1)
	if len(matches) != 1 {
		return "", "", fmt.Errorf("%s found %d times, expected once", what, len(matches))
	}
	start, end := matches[0][2], matches[0][3]
	return content[:start] + version + content[end:], content[start:end], nil
}
