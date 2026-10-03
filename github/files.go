package github

import (
	"context"
	"fmt"
	"net/url"
)

// File limits of the API. Lists of this length are probably incomplete.
const (
	MaxPullRequestFiles = 3000
	MaxCompareFiles     = 300
)

// ChangedFile is a file changed by a pull request or between two commits.
type ChangedFile struct {
	Path         string `json:"filename"`
	PreviousPath string `json:"previous_filename"`
	// Status is added, removed, modified, renamed, copied, changed or
	// unchanged.
	Status string `json:"status"`
}

// Exists reports whether the file exists after the change.
func (f ChangedFile) Exists() bool {
	return f.Status != "removed"
}

// PullRequestFiles lists the files changed by a pull request.
func (c *Client) PullRequestFiles(ctx context.Context, repo Repo, number int) ([]ChangedFile, error) {
	return getAll[ChangedFile](ctx, c, fmt.Sprintf("/repos/%s/pulls/%d/files", repo, number))
}

// CompareFiles lists the files changed from the merge base of base and head
// to head.
func (c *Client) CompareFiles(ctx context.Context, repo Repo, base, head string) ([]ChangedFile, error) {
	var res struct {
		Files []ChangedFile `json:"files"`
	}
	path := fmt.Sprintf("/repos/%s/compare/%s...%s", repo, url.PathEscape(base), url.PathEscape(head))
	if _, err := c.get(ctx, path, &res); err != nil {
		return nil, err
	}
	return res.Files, nil
}

// ChangedFiles lists the files the workflow run is about:
//
//   - with a pull request, the files it changes
//   - on push, the files changed between before and after, or between the
//     default branch and after for a new branch
//   - otherwise none
//
// It returns the maximum number of files the source can return, see
// MaxPullRequestFiles and MaxCompareFiles.
func (c *Client) ChangedFiles(ctx context.Context, gh *Context, pr *PullRequest) ([]ChangedFile, int, error) {
	if pr != nil {
		files, err := c.PullRequestFiles(ctx, gh.Repo, pr.Number)
		return files, MaxPullRequestFiles, err
	}

	if gh.EventName != "push" || isNullSHA(gh.Event.After) {
		return nil, 0, nil
	}

	base := gh.Event.Before
	if isNullSHA(base) {
		base = gh.Event.Repository.DefaultBranch
	}
	if base == "" {
		return nil, 0, nil
	}

	files, err := c.CompareFiles(ctx, gh.Repo, base, gh.Event.After)
	return files, MaxCompareFiles, err
}
