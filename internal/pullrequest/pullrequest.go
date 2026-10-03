// Package pullrequest resolves the pull request for actions that take the
// github-token, pr and require-pr inputs.
package pullrequest

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/josa42/actions/github"
	"github.com/josa42/actions/toolkit"
)

// Run is the context of an action run.
type Run struct {
	Client  *github.Client
	Context *github.Context
	// PR is nil if the run has no pull request and require-pr is not set.
	PR *github.PullRequest
}

// Resolve reads the inputs and finds the pull request: the one given by the
// pr input, or else the one the workflow runs for.
func Resolve(ctx context.Context) (*Run, error) {
	required, err := toolkit.InputBool("require-pr")
	if err != nil {
		return nil, err
	}

	number := 0
	if s := toolkit.Input("pr"); s != "" {
		if number, err = strconv.Atoi(s); err != nil || number < 1 {
			return nil, fmt.Errorf("input pr: %q is not a pull request number", s)
		}
	}

	client, err := github.NewClient(toolkit.Input("github-token"))
	if err != nil {
		return nil, err
	}
	gh, err := github.LoadContext()
	if err != nil {
		return nil, err
	}

	run := &Run{Client: client, Context: gh}
	if number > 0 {
		run.PR, err = client.PullRequest(ctx, gh.Repo, number)
	} else {
		run.PR, err = client.FindPullRequest(ctx, gh)
	}
	if err != nil {
		return nil, err
	}

	if run.PR == nil {
		if required {
			return nil, errors.New("no pull request found for event " + gh.EventName)
		}
		fmt.Printf("No pull request found for event %s\n", gh.EventName)
	} else {
		fmt.Printf("Pull request #%d: %s\n", run.PR.Number, run.PR.HTMLURL)
	}
	return run, nil
}
