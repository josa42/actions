// Package releaseprepare implements the release-prepare action.
package releaseprepare

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"time"

	"github.com/josa42/actions/changelog"
	"github.com/josa42/actions/git"
	"github.com/josa42/actions/github"
	"github.com/josa42/actions/semver"
	"github.com/josa42/actions/toolkit"
	"github.com/josa42/actions/versionfile"
)

type inputs struct {
	version       string
	files         []versionfile.File
	changelog     string
	tagFormat     string
	commitMessage string
	allowBranch   bool
}

func readInputs() (inputs, error) {
	in := inputs{
		version:       toolkit.Input("version"),
		changelog:     toolkit.Input("changelog"),
		tagFormat:     toolkit.Input("tag-format"),
		commitMessage: toolkit.Input("commit-message"),
	}
	if in.version == "" {
		return in, errors.New("input version is required")
	}
	if strings.Count(in.tagFormat, "{version}") != 1 {
		return in, fmt.Errorf("input tag-format: %q needs exactly one {version}", in.tagFormat)
	}
	if in.commitMessage == "" {
		return in, errors.New("input commit-message is required")
	}

	var err error
	if in.files, err = versionfile.ParseList(toolkit.InputList("version-files")); err != nil {
		return in, fmt.Errorf("input version-files: %w", err)
	}
	if in.allowBranch, err = toolkit.InputBool("allow-branch"); err != nil {
		return in, err
	}
	return in, nil
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

	branch, err := checkBranch(ctx, client, gh, in.allowBranch)
	if err != nil {
		return err
	}

	tags, err := client.Tags(ctx, gh.Repo)
	if err != nil {
		return err
	}
	latest, hasLatest := semver.Latest(tags, in.tagFormat)
	v, err := semver.Next(in.version, latest, hasLatest)
	if err != nil {
		return err
	}
	version := v.String()
	tag := strings.ReplaceAll(in.tagFormat, "{version}", version)
	if hasLatest {
		fmt.Printf("Releasing %s, the latest release is %s\n", version, latest)
	} else {
		fmt.Printf("Releasing %s, the first release\n", version)
	}

	exists, err := client.TagExists(ctx, gh.Repo, tag)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("tag %s already exists", tag)
	}

	p, err := plan(in, version, time.Now().UTC().Format("2006-01-02"))
	if err != nil {
		return err
	}
	for _, e := range p.edits {
		if err := os.WriteFile(e.path, []byte(e.content), 0o644); err != nil {
			return err
		}
	}

	sha, err := commitAndPush(in, version, tag, branch, p.paths())
	if err != nil {
		return err
	}

	for name, value := range map[string]string{
		"version": version,
		"tag":     tag,
		"sha":     sha,
		"notes":   p.notes,
	} {
		if err := toolkit.SetOutput(name, value); err != nil {
			return err
		}
	}
	return nil
}

// checkBranch returns the branch the workflow runs on, which must be the
// default branch unless allowed otherwise.
func checkBranch(ctx context.Context, client *github.Client, gh *github.Context, allow bool) (string, error) {
	if os.Getenv("GITHUB_REF_TYPE") != "branch" {
		return "", errors.New("releases must run on a branch, not a tag")
	}
	branch := os.Getenv("GITHUB_REF_NAME")

	def := gh.Event.Repository.DefaultBranch
	if def == "" {
		var err error
		if def, err = client.DefaultBranch(ctx, gh.Repo); err != nil {
			return "", err
		}
	}
	if branch != def && !allow {
		return "", fmt.Errorf("releases run on the default branch %s, not %s. Set allow-branch to release from another branch", def, branch)
	}
	return branch, nil
}

type edit struct {
	path    string
	content string
}

type releasePlan struct {
	edits []edit
	notes string
}

func (p releasePlan) paths() []string {
	var paths []string
	for _, e := range p.edits {
		paths = append(paths, e.path)
	}
	return paths
}

// plan computes every file change of the release without writing anything,
// so a failing check leaves the working tree untouched.
func plan(in inputs, version, date string) (releasePlan, error) {
	var p releasePlan

	if in.changelog != "" {
		data, err := os.ReadFile(in.changelog)
		switch {
		case errors.Is(err, fs.ErrNotExist):
			fmt.Printf("%s: not found, skipped\n", in.changelog)
		case err != nil:
			return p, err
		default:
			content, notes, err := changelog.Promote(string(data), version, date)
			if err != nil {
				return p, fmt.Errorf("%s: %w", in.changelog, err)
			}
			p.edits = append(p.edits, edit{in.changelog, content})
			p.notes = notes
			fmt.Printf("%s: Unreleased → %s - %s\n", in.changelog, version, date)
		}
	}

	for _, f := range in.files {
		content, found := "", false
		for _, e := range p.edits {
			if e.path == f.Path {
				content, found = e.content, true
			}
		}
		if !found {
			data, err := os.ReadFile(f.Path)
			if err != nil {
				return p, err
			}
			content = string(data)
		}

		updated, old, err := versionfile.Replace(content, f.Template, version)
		if err != nil {
			return p, fmt.Errorf("%s: %w", f.Path, err)
		}
		p.edits = setEdit(p.edits, edit{f.Path, updated})
		fmt.Printf("%s: %s → %s\n", f.Path, old, version)
	}

	return p, nil
}

// setEdit adds e, replacing an earlier edit of the same file, so several
// templates can apply to one file.
func setEdit(edits []edit, e edit) []edit {
	for i := range edits {
		if edits[i].path == e.path {
			edits[i] = e
			return edits
		}
	}
	return append(edits, e)
}

func commitAndPush(in inputs, version, tag, branch string, paths []string) (string, error) {
	repo := git.Repo{Dir: "."}

	if len(paths) > 0 {
		if err := repo.Add(paths...); err != nil {
			return "", err
		}
	}
	changed, err := repo.HasStagedChanges()
	if err != nil {
		return "", err
	}
	if changed {
		msg := strings.ReplaceAll(in.commitMessage, "{version}", version)
		if err := repo.Commit(msg, git.Bot); err != nil {
			return "", err
		}
		fmt.Printf("Committed %q\n", msg)
	} else {
		fmt.Println("Nothing to commit, tagging the current commit")
	}

	if err := repo.Tag(tag, "Release "+tag, git.Bot); err != nil {
		return "", err
	}
	if err := repo.PushAtomic("origin", "HEAD:refs/heads/"+branch, "refs/tags/"+tag); err != nil {
		return "", err
	}
	fmt.Printf("Pushed %s and %s\n", branch, tag)

	return repo.Head()
}
