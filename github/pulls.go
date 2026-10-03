package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
)

// PullRequest holds the fields of a pull request this package uses. Raw is
// the full API response.
type PullRequest struct {
	Number  int    `json:"number"`
	HTMLURL string `json:"html_url"`
	Title   string `json:"title"`
	State   string `json:"state"`
	Draft   bool   `json:"draft"`
	Head    Ref    `json:"head"`
	Base    Ref    `json:"base"`
	Labels  []struct {
		Name string `json:"name"`
	} `json:"labels"`

	Raw json.RawMessage `json:"-"`
}

// Ref is the head or base of a pull request.
type Ref struct {
	Ref string `json:"ref"`
	SHA string `json:"sha"`
}

// PullRequest fetches a pull request.
func (c *Client) PullRequest(ctx context.Context, repo Repo, number int) (*PullRequest, error) {
	var pr PullRequest
	data, err := c.get(ctx, fmt.Sprintf("/repos/%s/pulls/%d", repo, number), &pr)
	if err != nil {
		return nil, err
	}
	pr.Raw = data
	return &pr, nil
}

// OpenPullRequestForCommit returns the open pull request containing sha,
// preferring one whose head is sha. It returns nil if there is none.
func (c *Client) OpenPullRequestForCommit(ctx context.Context, repo Repo, sha string) (*PullRequest, error) {
	prs, err := getAll[PullRequest](ctx, c, fmt.Sprintf("/repos/%s/commits/%s/pulls", repo, url.PathEscape(sha)))
	if err != nil {
		return nil, err
	}

	var found *PullRequest
	for i, pr := range prs {
		if pr.State != "open" {
			continue
		}
		if pr.Head.SHA == sha {
			found = &prs[i]
			break
		}
		if found == nil {
			found = &prs[i]
		}
	}
	if found == nil {
		return nil, nil
	}
	return c.PullRequest(ctx, repo, found.Number)
}

// mergeQueueRe matches merge queue branches, e.g.
// gh-readonly-queue/main/pr-123-<sha>.
var mergeQueueRe = regexp.MustCompile(`(?:^|/)gh-readonly-queue/.+/pr-(\d+)-[0-9a-f]+$`)

// FindPullRequest returns the pull request the workflow runs for, or nil if
// there is none:
//
//   - pull_request, pull_request_target: the event's pull request
//   - issue_comment: the commented pull request
//   - merge_group: the pull request from the merge queue branch
//   - push, deployment, deployment_status: the open pull request containing
//     the commit
//
// The pull request is always fetched from the API, so it is up to date.
func (c *Client) FindPullRequest(ctx context.Context, gh *Context) (*PullRequest, error) {
	if n := pullRequestNumber(gh); n > 0 {
		return c.PullRequest(ctx, gh.Repo, n)
	}
	if sha := commitSHA(gh); !isNullSHA(sha) {
		return c.OpenPullRequestForCommit(ctx, gh.Repo, sha)
	}
	return nil, nil
}

func pullRequestNumber(gh *Context) int {
	ev := gh.Event
	switch gh.EventName {
	case "pull_request", "pull_request_target":
		if ev.PullRequest != nil {
			return ev.PullRequest.Number
		}
	case "issue_comment":
		if ev.Issue != nil && ev.Issue.PullRequest != nil {
			return ev.Issue.Number
		}
	case "merge_group":
		if ev.MergeGroup != nil {
			if m := mergeQueueRe.FindStringSubmatch(ev.MergeGroup.HeadRef); m != nil {
				n, _ := strconv.Atoi(m[1])
				return n
			}
		}
	}
	return 0
}

func commitSHA(gh *Context) string {
	switch gh.EventName {
	case "push":
		return gh.Event.After
	case "deployment", "deployment_status":
		if gh.Event.Deployment != nil {
			return gh.Event.Deployment.SHA
		}
	}
	return ""
}
