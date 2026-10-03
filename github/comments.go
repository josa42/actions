package github

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/josa42/actions/markdown"
)

// MaxCommentLength is the maximum length of a comment body, see markdown.Len.
const MaxCommentLength = 65536

// Comment is an issue or pull request comment.
type Comment struct {
	ID      int64  `json:"id"`
	HTMLURL string `json:"html_url"`
	Body    string `json:"body"`
	User    struct {
		Login string `json:"login"`
	} `json:"user"`
}

// Comments lists the comments of an issue or pull request, oldest first.
func (c *Client) Comments(ctx context.Context, repo Repo, issue int) ([]Comment, error) {
	return getAll[Comment](ctx, c, fmt.Sprintf("/repos/%s/issues/%d/comments", repo, issue))
}

// CreateComment comments on an issue or pull request.
func (c *Client) CreateComment(ctx context.Context, repo Repo, issue int, body string) (*Comment, error) {
	var out Comment
	path := fmt.Sprintf("/repos/%s/issues/%d/comments", repo, issue)
	_, _, err := c.do(ctx, http.MethodPost, path, map[string]string{"body": body}, &out)
	return &out, err
}

// UpdateComment replaces the body of a comment.
func (c *Client) UpdateComment(ctx context.Context, repo Repo, id int64, body string) (*Comment, error) {
	var out Comment
	path := fmt.Sprintf("/repos/%s/issues/comments/%d", repo, id)
	_, _, err := c.do(ctx, http.MethodPatch, path, map[string]string{"body": body}, &out)
	return &out, err
}

// DeleteComment deletes a comment.
func (c *Client) DeleteComment(ctx context.Context, repo Repo, id int64) error {
	_, _, err := c.do(ctx, http.MethodDelete, fmt.Sprintf("/repos/%s/issues/comments/%d", repo, id), nil, nil)
	return err
}

// CommentTarget selects the comments managed under an identifier.
type CommentTarget struct {
	Repo  Repo
	Issue int
	// Identifier is stored as a hidden marker in the comment body. Comments
	// without it are never changed.
	Identifier string
	// Author limits matches to comments by this user, if set.
	Author string
}

// marker returns the hidden marker for the identifier. ">" and newlines are
// escaped so an identifier can't end the HTML comment.
func (t CommentTarget) marker() string {
	id := strings.NewReplacer("%", "%25", ">", "%3E", "\r", "%0D", "\n", "%0A").Replace(t.Identifier)
	return "<!-- comment-identifier: " + id + " -->"
}

func (t CommentTarget) matches(c Comment) bool {
	return strings.Contains(c.Body, t.marker()) && (t.Author == "" || c.User.Login == t.Author)
}

func (c *Client) find(ctx context.Context, t CommentTarget) ([]Comment, error) {
	comments, err := c.Comments(ctx, t.Repo, t.Issue)
	if err != nil {
		return nil, err
	}
	var matched []Comment
	for _, cm := range comments {
		if t.matches(cm) {
			matched = append(matched, cm)
		}
	}
	return matched, nil
}

// PostComment posts body, truncated to fit MaxCommentLength. With an
// identifier, it updates the newest comment carrying it and deletes older
// ones instead of adding another comment. It reports whether body was
// truncated.
func (c *Client) PostComment(ctx context.Context, t CommentTarget, body string) (*Comment, bool, error) {
	if t.Identifier == "" {
		body, truncated := markdown.Truncate(body, MaxCommentLength)
		cm, err := c.CreateComment(ctx, t.Repo, t.Issue, body)
		return cm, truncated, err
	}

	// Truncate first, so the marker is never cut off.
	suffix := "\n\n" + t.marker()
	body, truncated := markdown.Truncate(body, MaxCommentLength-markdown.Len(suffix))
	body += suffix

	existing, err := c.find(ctx, t)
	if err != nil {
		return nil, truncated, err
	}
	if len(existing) == 0 {
		cm, err := c.CreateComment(ctx, t.Repo, t.Issue, body)
		return cm, truncated, err
	}

	cm := &existing[len(existing)-1]
	if cm.Body != body {
		if cm, err = c.UpdateComment(ctx, t.Repo, cm.ID, body); err != nil {
			return nil, truncated, err
		}
	}
	for _, old := range existing[:len(existing)-1] {
		if err := c.DeleteComment(ctx, t.Repo, old.ID); err != nil && !IsNotFound(err) {
			return cm, truncated, err
		}
	}
	return cm, truncated, nil
}

// DeleteComments deletes all comments carrying the identifier and returns
// how many were deleted.
func (c *Client) DeleteComments(ctx context.Context, t CommentTarget) (int, error) {
	if t.Identifier == "" {
		return 0, fmt.Errorf("identifier is required to delete comments")
	}

	existing, err := c.find(ctx, t)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, cm := range existing {
		// Another run may have deleted it already.
		if err := c.DeleteComment(ctx, t.Repo, cm.ID); err != nil && !IsNotFound(err) {
			return n, err
		}
		n++
	}
	return n, nil
}
