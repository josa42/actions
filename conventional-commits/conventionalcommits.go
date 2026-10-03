// Package conventionalcommits implements the conventional-commits action.
package conventionalcommits

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/josa42/actions/conventional"
	"github.com/josa42/actions/github"
	"github.com/josa42/actions/toolkit"
)

const commentIdentifier = "conventional-commits"

type inputs struct {
	cfg     conventional.Config
	comment bool
}

func readInputs() (inputs, error) {
	in := inputs{cfg: conventional.Config{
		Types:  toolkit.InputList("types"),
		Scopes: toolkit.InputList("scopes"),
	}}

	if s := toolkit.Input("max-length"); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil || n < 0 {
			return in, fmt.Errorf("input max-length: %q is not a number", s)
		}
		in.cfg.MaxLength = n
	}

	for name, dst := range map[string]*bool{
		"lowercase":          &in.cfg.Lowercase,
		"no-trailing-period": &in.cfg.NoPeriod,
		"comment":            &in.comment,
	} {
		v, err := toolkit.InputBool(name)
		if err != nil {
			return in, err
		}
		*dst = v
	}
	return in, nil
}

// item is a message to check: a commit or the pull request title.
type item struct {
	label   string
	url     string
	message string
	title   bool
}

func (it item) header() string {
	h, _, _ := strings.Cut(strings.TrimSpace(it.message), "\n")
	return strings.TrimSpace(h)
}

type result struct {
	item
	violations []string
}

func Run(ctx context.Context) error {
	in, err := readInputs()
	if err != nil {
		return err
	}
	client, err := github.NewClient(toolkit.Input("github-token"))
	if err != nil {
		return err
	}
	gh, err := github.LoadContext()
	if err != nil {
		return err
	}

	var items []item
	var commits []github.Commit
	var pr *github.PullRequest

	switch gh.EventName {
	case "pull_request", "pull_request_target":
		if gh.Event.PullRequest == nil {
			return fmt.Errorf("%s event without a pull request", gh.EventName)
		}
		if pr, err = client.PullRequest(ctx, gh.Repo, gh.Event.PullRequest.Number); err != nil {
			return err
		}
		items = append(items, item{label: "PR title", url: pr.HTMLURL, message: pr.Title, title: true})
		if commits, err = client.PullRequestCommits(ctx, gh.Repo, pr.Number); err != nil {
			return err
		}
		if len(commits) >= github.MaxPullRequestCommits {
			toolkit.Warning(fmt.Sprintf("The GitHub API lists at most %d commits of a pull request, later ones are not checked.", github.MaxPullRequestCommits))
		}
	case "push":
		if commits, err = client.PushCommits(ctx, gh); err != nil {
			return err
		}
	default:
		toolkit.Notice(fmt.Sprintf("Nothing to check for %s events.", gh.EventName))
		return nil
	}

	for _, c := range commits {
		if c.IsMerge() {
			fmt.Printf("Skipping merge commit %s\n", c.SHA[:min(7, len(c.SHA))])
			continue
		}
		items = append(items, item{label: c.SHA[:min(7, len(c.SHA))], url: c.HTMLURL, message: c.Commit.Message})
	}

	results := check(items, in.cfg)
	invalid := report(results)

	if in.comment && pr != nil {
		comment(ctx, client, gh.Repo, pr.Number, results, invalid)
	}

	if invalid > 0 {
		return fmt.Errorf("%d of %d messages don't follow Conventional Commits", invalid, len(results))
	}
	fmt.Printf("All %d messages follow Conventional Commits\n", len(results))
	return nil
}

func check(items []item, cfg conventional.Config) []result {
	results := make([]result, len(items))
	for i, it := range items {
		results[i].item = it
		if it.title {
			results[i].violations = conventional.CheckHeader(it.header(), cfg)
		} else {
			results[i].violations = conventional.Check(it.message, cfg)
		}
	}
	return results
}

// report logs an annotation per invalid message, writes the job summary and
// returns the number of invalid messages.
func report(results []result) int {
	invalid := 0
	for _, r := range results {
		if len(r.violations) > 0 {
			invalid++
			toolkit.Error(strings.Join(r.violations, "\n"), toolkit.Annotation{Title: r.label + ": " + r.header()})
		}
	}

	var b strings.Builder
	b.WriteString("### Conventional commits\n\n")
	if len(results) == 0 {
		b.WriteString("Nothing to check.\n")
	} else {
		b.WriteString(table(results, true))
	}
	if err := toolkit.AddSummary(b.String()); err != nil {
		toolkit.Warning("Could not write the job summary: " + err.Error())
	}
	return invalid
}

// comment posts the invalid messages to the pull request, or deletes an
// earlier comment once all are valid. Failing to comment, e.g. with the
// read-only token of a fork, is a warning: the check result stands either
// way.
func comment(ctx context.Context, client *github.Client, repo github.Repo, number int, results []result, invalid int) {
	target := github.CommentTarget{Repo: repo, Issue: number, Identifier: commentIdentifier}

	if invalid == 0 {
		if _, err := client.DeleteComments(ctx, target); err != nil {
			toolkit.Warning("Could not delete the pull request comment: " + err.Error())
		}
		return
	}

	var bad []result
	for _, r := range results {
		if len(r.violations) > 0 {
			bad = append(bad, r)
		}
	}
	body := fmt.Sprintf("### Conventional commits\n\n%d of %d messages don't follow [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/):\n\n%s\nFix commit messages with `git rebase -i` and force-push. The title can be edited on the pull request.",
		invalid, len(results), table(bad, false))
	if _, _, err := client.PostComment(ctx, target, body); err != nil {
		toolkit.Warning("Could not comment on the pull request: " + err.Error())
	}
}

func table(results []result, status bool) string {
	var b strings.Builder
	if status {
		b.WriteString("| | Message | Header | Problems |\n| --- | --- | --- | --- |\n")
	} else {
		b.WriteString("| Message | Header | Problems |\n| --- | --- | --- |\n")
	}
	for _, r := range results {
		if status {
			if len(r.violations) == 0 {
				b.WriteString("| ✅ ")
			} else {
				b.WriteString("| ❌ ")
			}
		}
		label := r.label
		if r.url != "" {
			label = "[" + label + "](" + r.url + ")"
		}
		fmt.Fprintf(&b, "| %s | %s | %s |\n", label, code(r.header()), cell(strings.Join(r.violations, "<br>")))
	}
	return b.String()
}

// code formats s as inline code in a table cell.
func code(s string) string {
	if s == "" {
		return ""
	}
	s = cell(s)
	if strings.Contains(s, "`") {
		return "`` " + s + " ``"
	}
	return "`" + s + "`"
}

func cell(s string) string {
	return strings.NewReplacer("|", `\|`, "\n", " ").Replace(s)
}
