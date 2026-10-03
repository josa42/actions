// Package git runs the git CLI.
package git

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Repo is a git working tree.
type Repo struct {
	Dir string
}

// Signature is the author and committer of commits and tags.
type Signature struct {
	Name  string
	Email string
}

// Bot is the signature of the GitHub Actions bot.
var Bot = Signature{"github-actions[bot]", "41898282+github-actions[bot]@users.noreply.github.com"}

func (r Repo) run(sig *Signature, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = r.Dir
	cmd.Env = os.Environ()
	if sig != nil {
		cmd.Env = append(cmd.Env,
			"GIT_AUTHOR_NAME="+sig.Name, "GIT_AUTHOR_EMAIL="+sig.Email,
			"GIT_COMMITTER_NAME="+sig.Name, "GIT_COMMITTER_EMAIL="+sig.Email,
		)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", &Error{Args: args, Msg: msg, err: err}
	}
	return strings.TrimSpace(stdout.String()), nil
}

// Error is a failed git command.
type Error struct {
	Args []string
	Msg  string
	err  error
}

func (e *Error) Error() string { return fmt.Sprintf("git %s: %s", strings.Join(e.Args, " "), e.Msg) }
func (e *Error) Unwrap() error { return e.err }

func exitCode(err error) int {
	var e *exec.ExitError
	if errors.As(err, &e) {
		return e.ExitCode()
	}
	return -1
}

// Head returns the SHA of HEAD.
func (r Repo) Head() (string, error) {
	return r.run(nil, "rev-parse", "HEAD")
}

// Add stages paths.
func (r Repo) Add(paths ...string) error {
	_, err := r.run(nil, append([]string{"add", "--"}, paths...)...)
	return err
}

// HasStagedChanges reports whether the index differs from HEAD.
func (r Repo) HasStagedChanges() (bool, error) {
	_, err := r.run(nil, "diff", "--cached", "--quiet")
	if err == nil {
		return false, nil
	}
	if exitCode(err) == 1 {
		return true, nil
	}
	return false, err
}

// Commit commits the index.
func (r Repo) Commit(msg string, sig Signature) error {
	_, err := r.run(&sig, "commit", "--no-verify", "-m", msg)
	return err
}

// Tag creates an annotated tag on HEAD.
func (r Repo) Tag(name, msg string, sig Signature) error {
	_, err := r.run(&sig, "tag", "-a", name, "-m", msg)
	return err
}

// PushAtomic pushes refspecs to remote, all or none.
func (r Repo) PushAtomic(remote string, refspecs ...string) error {
	_, err := r.run(nil, append([]string{"push", "--atomic", remote}, refspecs...)...)
	return err
}
