// Package prcomment implements the pr-comment action.
package prcomment

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/josa42/actions/github"
	"github.com/josa42/actions/internal/pullrequest"
	"github.com/josa42/actions/toolkit"
)

type inputs struct {
	action     string
	body       string
	identifier string
	author     string
}

func readInputs() (inputs, error) {
	in := inputs{
		action:     toolkit.Input("action"),
		identifier: toolkit.Input("identifier"),
		author:     toolkit.Input("author"),
	}

	body, bodyPath := toolkit.Input("body"), toolkit.Input("body-path")
	if body != "" && bodyPath != "" {
		return in, errors.New("inputs body and body-path are mutually exclusive")
	}
	if bodyPath != "" {
		data, err := os.ReadFile(bodyPath)
		if err != nil {
			return in, fmt.Errorf("input body-path: %w", err)
		}
		body = string(data)
	}
	in.body = strings.TrimSpace(body)

	switch in.action {
	case "post":
		if in.body == "" {
			return in, errors.New("input body or body-path is required to post a comment")
		}
	case "delete":
		if in.identifier == "" {
			return in, errors.New("input identifier is required to delete comments")
		}
	default:
		return in, fmt.Errorf("input action: %q is not post or delete", in.action)
	}
	return in, nil
}

func Run(ctx context.Context) error {
	in, err := readInputs()
	if err != nil {
		return err
	}

	run, err := pullrequest.Resolve(ctx)
	if err != nil {
		return err
	}
	if run.PR == nil {
		toolkit.Notice("No pull request found, skipping the comment.")
		return nil
	}

	target := github.CommentTarget{
		Repo:       run.Context.Repo,
		Issue:      run.PR.Number,
		Identifier: in.identifier,
		Author:     in.author,
	}

	if in.action == "delete" {
		n, err := run.Client.DeleteComments(ctx, target)
		if err != nil {
			return err
		}
		fmt.Printf("Deleted %d comments\n", n)
		return nil
	}

	cm, truncated, err := run.Client.PostComment(ctx, target, in.body)
	if err != nil {
		return err
	}
	if truncated {
		toolkit.Warning("The comment exceeds the maximum length and was truncated.")
	}
	fmt.Printf("Comment: %s\n", cm.HTMLURL)

	if err := toolkit.SetOutput("comment-id", strconv.FormatInt(cm.ID, 10)); err != nil {
		return err
	}
	return toolkit.SetOutput("comment-url", cm.HTMLURL)
}
