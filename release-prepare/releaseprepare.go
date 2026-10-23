// Package releaseprepare implements the release-prepare action.
package releaseprepare

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"slices"
	"strings"
	"time"

	"github.com/josa42/actions/changelog"
	"github.com/josa42/actions/conventional"
	"github.com/josa42/actions/git"
	"github.com/josa42/actions/github"
	"github.com/josa42/actions/semver"
	"github.com/josa42/actions/toolkit"
	"github.com/josa42/actions/versionfile"
)

type inputs struct {
	version       string
	files         []versionfile.File
	npmPackages   []string
	onlyIfChanged []string
	changelog     string
	tagFormat     string
	commitMessage string
	allowBranch   bool
	commitNotes   bool
}

func readInputs() (inputs, error) {
	in := inputs{
		version:       toolkit.Input("version"),
		changelog:     toolkit.Input("changelog"),
		tagFormat:     toolkit.Input("tag-format"),
		commitMessage: toolkit.Input("commit-message"),
		npmPackages:   toolkit.InputList("npm-packages"),
		onlyIfChanged: toolkit.InputList("only-if-changed"),
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
	if in.commitNotes, err = toolkit.InputBool("commit-notes"); err != nil {
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

	since := ""
	if hasLatest {
		since = strings.ReplaceAll(in.tagFormat, "{version}", latest.String())
	}
	h, err := readHistory(git.Repo{Dir: "."}, in, since)
	if err != nil {
		return err
	}

	p, err := plan(in, version, time.Now().UTC().Format("2006-01-02"), h)
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
		"changed": strings.Join(p.changed, "\n"),
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

// history is what happened since the latest release.
type history struct {
	// changed are the only-if-changed directories that changed.
	changed []string
	// subjects of the commits, oldest first. Only read for commit-notes.
	subjects []string
}

// readHistory compares the checkout with since, the tag of the latest
// release. Without one, every directory counts as changed and every commit
// is new.
func readHistory(repo git.Repo, in inputs, since string) (history, error) {
	var h history
	if in.commitNotes {
		var err error
		if h.subjects, err = repo.Subjects(since); err != nil {
			return h, fmt.Errorf("%w. commit-notes needs the history, check out with fetch-depth: 0", err)
		}
	}
	for _, dir := range in.onlyIfChanged {
		changed := since == ""
		if !changed {
			var err error
			if changed, err = repo.ChangedSince(since, dir); err != nil {
				return h, fmt.Errorf("%w. only-if-changed needs the tags, check out with fetch-depth: 0", err)
			}
		}
		if changed {
			h.changed = append(h.changed, dir)
		}
	}
	return h, nil
}

type edit struct {
	path    string
	content string
}

type releasePlan struct {
	edits []edit
	notes string
	// changed are the changed only-if-changed directories with a file that
	// got the version.
	changed []string
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
func plan(in inputs, version, date string, h history) (releasePlan, error) {
	var p releasePlan

	promoted := false
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
			p.notes, promoted = notes, true
			fmt.Printf("%s: Unreleased → %s - %s\n", in.changelog, version, date)
		}
	}
	if !promoted && in.commitNotes {
		p.notes = commitNotes(h.subjects)
		fmt.Printf("Release notes from %d commits\n", len(h.subjects))
	}

	for _, f := range in.files {
		if dir, ok := unchanged(in, h, f.Path); ok {
			fmt.Printf("%s: %s did not change, skipped\n", f.Path, dir)
			continue
		}
		content, err := current(p.edits, f.Path)
		if err != nil {
			return p, err
		}
		updated, old, err := versionfile.Replace(content, f.Template, version)
		if err != nil {
			return p, fmt.Errorf("%s: %w", f.Path, err)
		}
		p.edits = setEdit(p.edits, edit{f.Path, updated})
		fmt.Printf("%s: %s → %s\n", f.Path, old, version)
	}

	for _, dir := range in.npmPackages {
		pkg := path.Join(dir, "package.json")
		if d, ok := unchanged(in, h, dir); ok {
			fmt.Printf("%s: %s did not change, skipped\n", pkg, d)
			continue
		}
		content, err := current(p.edits, pkg)
		if err != nil {
			return p, err
		}
		updated, old, err := versionfile.Replace(content, `"version": "{version}"`, version)
		if err != nil {
			return p, fmt.Errorf("%s: %w", pkg, err)
		}
		p.edits = setEdit(p.edits, edit{pkg, updated})
		fmt.Printf("%s: %s → %s\n", pkg, old, version)

		lock := path.Join(dir, "package-lock.json")
		content, err = current(p.edits, lock)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return p, err
		}
		if updated, old, err = versionfile.ReplaceNPMLock(content, version); err != nil {
			return p, fmt.Errorf("%s: %w", lock, err)
		}
		p.edits = setEdit(p.edits, edit{lock, updated})
		fmt.Printf("%s: %s → %s\n", lock, old, version)
	}

	for _, dir := range h.changed {
		for _, e := range p.edits {
			if e.path != in.changelog && inside(e.path, dir) {
				p.changed = append(p.changed, dir)
				break
			}
		}
	}

	return p, nil
}

// unchanged returns the only-if-changed directory holding name if that did
// not change since the latest release.
func unchanged(in inputs, h history, name string) (string, bool) {
	for _, dir := range in.onlyIfChanged {
		if inside(name, dir) && !slices.Contains(h.changed, dir) {
			return dir, true
		}
	}
	return "", false
}

// inside reports whether name is dir or in it.
func inside(name, dir string) bool {
	name, dir = path.Clean(name), path.Clean(dir)
	return dir == "." || name == dir || strings.HasPrefix(name, dir+"/")
}

// noteSections are the commit types in generated release notes, in order.
var noteSections = []struct{ typ, title string }{
	{"feat", "Features"},
	{"fix", "Fixes"},
	{"perf", "Performance"},
}

// commitNotes generates release notes from Conventional Commits subjects,
// grouped by type. Other types and subjects that are no Conventional Commits
// are left out.
func commitNotes(subjects []string) string {
	var sections []string
	for _, s := range noteSections {
		var lines []string
		for _, subject := range subjects {
			c, ok := conventional.Parse(subject)
			if !ok || c.Type != s.typ {
				continue
			}
			line := "- "
			if c.Scope != "" {
				line += "**" + c.Scope + ":** "
			}
			line += c.Description
			if c.Breaking {
				line += " **(breaking)**"
			}
			lines = append(lines, line)
		}
		if len(lines) > 0 {
			sections = append(sections, "### "+s.title+"\n\n"+strings.Join(lines, "\n"))
		}
	}
	return strings.Join(sections, "\n\n")
}

// current returns the content of a file, with the edits planned so far.
func current(edits []edit, name string) (string, error) {
	for _, e := range edits {
		if e.path == name {
			return e.content, nil
		}
	}
	data, err := os.ReadFile(name)
	return string(data), err
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
