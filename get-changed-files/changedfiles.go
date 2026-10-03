// Package changedfiles implements the get-changed-files action.
package changedfiles

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/josa42/actions/github"
	"github.com/josa42/actions/internal/pullrequest"
	"github.com/josa42/actions/pathmatch"
	"github.com/josa42/actions/toolkit"
)

func Run(ctx context.Context) error {
	m, err := pathmatch.Compile(toolkit.InputList("paths"))
	if err != nil {
		return fmt.Errorf("input paths: %w", err)
	}

	run, err := pullrequest.Resolve(ctx)
	if err != nil {
		return err
	}

	files, limit, err := run.Client.ChangedFiles(ctx, run.Context, run.PR)
	if err != nil {
		return err
	}
	if limit > 0 && len(files) >= limit {
		toolkit.Warning(fmt.Sprintf("The GitHub API returned %d files, its maximum. The list is probably incomplete.", len(files)))
	}

	paths, matched := filter(m, files)

	toolkit.Group(fmt.Sprintf("%d of %d changed files match", len(paths), len(files)), func() error {
		for _, p := range paths {
			fmt.Println(p)
		}
		return nil
	})

	data, err := json.Marshal(paths)
	if err != nil {
		return err
	}
	for name, value := range map[string]string{
		"files":   string(data),
		"matched": strconv.FormatBool(matched),
		"count":   strconv.Itoa(len(paths)),
	} {
		if err := toolkit.SetOutput(name, value); err != nil {
			return err
		}
	}
	return nil
}

// filter returns the matching paths of files that exist after the change, and
// whether any change matches. Deleted files and the old path of renamed files
// count as changes, so moving a file out of a directory matches it.
func filter(m *pathmatch.Matcher, files []github.ChangedFile) ([]string, bool) {
	paths := []string{}
	matched := false
	for _, f := range files {
		match := m.Match(f.Path)
		if match && f.Exists() {
			paths = append(paths, f.Path)
		}
		if match || (f.PreviousPath != "" && m.Match(f.PreviousPath)) {
			matched = true
		}
	}
	return paths, matched
}
