// Package getpr implements the get-pr action.
package getpr

import (
	"context"
	"strconv"
	"strings"

	"github.com/josa42/actions/internal/pullrequest"
	"github.com/josa42/actions/toolkit"
)

func Run(ctx context.Context) error {
	run, err := pullrequest.Resolve(ctx)
	if err != nil {
		return err
	}

	pr := run.PR
	if pr == nil {
		return toolkit.SetOutput("found", "false")
	}

	var labels []string
	for _, l := range pr.Labels {
		labels = append(labels, l.Name)
	}

	for name, value := range map[string]string{
		"found":    "true",
		"number":   strconv.Itoa(pr.Number),
		"url":      pr.HTMLURL,
		"title":    pr.Title,
		"head-ref": pr.Head.Ref,
		"head-sha": pr.Head.SHA,
		"base-ref": pr.Base.Ref,
		"base-sha": pr.Base.SHA,
		"draft":    strconv.FormatBool(pr.Draft),
		"labels":   strings.Join(labels, "\n"),
		"json":     string(pr.Raw),
	} {
		if err := toolkit.SetOutput(name, value); err != nil {
			return err
		}
	}
	return nil
}
