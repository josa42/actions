// Package versionfile updates versions in files of any format, using
// templates such as `"version": "{version}"`.
package versionfile

import (
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
