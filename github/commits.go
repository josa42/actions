package github

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

// MaxPullRequestCommits is the maximum number of commits the API lists for
// a pull request.
const MaxPullRequestCommits = 250

// Commit is a commit as listed by the API.
type Commit struct {
	SHA     string `json:"sha"`
	HTMLURL string `json:"html_url"`
	Commit  struct {
		Message string `json:"message"`
	} `json:"commit"`
	Parents []struct {
		SHA string `json:"sha"`
	} `json:"parents"`
}

// IsMerge reports whether the commit has more than one parent.
func (c Commit) IsMerge() bool {
	return len(c.Parents) > 1
}

// PullRequestCommits lists the commits of a pull request, oldest first.
func (c *Client) PullRequestCommits(ctx context.Context, repo Repo, number int) ([]Commit, error) {
	return getAll[Commit](ctx, c, fmt.Sprintf("/repos/%s/pulls/%d/commits", repo, number))
}

// CompareCommits lists the commits reachable from head but not from base,
// oldest first.
func (c *Client) CompareCommits(ctx context.Context, repo Repo, base, head string) ([]Commit, error) {
	var all []Commit
	next := fmt.Sprintf("/repos/%s/compare/%s...%s?per_page=100", repo, url.PathEscape(base), url.PathEscape(head))
	for next != "" {
		var page struct {
			Commits []Commit `json:"commits"`
		}
		_, n, err := c.do(ctx, http.MethodGet, next, nil, &page)
		if err != nil {
			return nil, err
		}
		all = append(all, page.Commits...)
		next = n
	}
	return all, nil
}

// PushCommits lists the commits of a push event: from before to after, or
// from the default branch to after for a new branch. A push deleting a branch
// and other events have none.
func (c *Client) PushCommits(ctx context.Context, gh *Context) ([]Commit, error) {
	if gh.EventName != "push" || isNullSHA(gh.Event.After) {
		return nil, nil
	}
	base := gh.Event.Before
	if isNullSHA(base) {
		base = gh.Event.Repository.DefaultBranch
	}
	if base == "" {
		return nil, nil
	}
	return c.CompareCommits(ctx, gh.Repo, base, gh.Event.After)
}
