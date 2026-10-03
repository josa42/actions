package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func setup(t *testing.T) (Repo, string) {
	t.Helper()
	dir := t.TempDir()
	remote := filepath.Join(dir, "remote.git")
	work := filepath.Join(dir, "work")

	for _, args := range [][]string{
		{"init", "-q", "--bare", "-b", "main", remote},
		{"init", "-q", "-b", "main", work},
		{"-C", work, "remote", "add", "origin", remote},
	} {
		if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s", args, out)
		}
	}

	r := Repo{Dir: work}
	write(t, r, "a.txt", "a")
	if err := r.Add("a.txt"); err != nil {
		t.Fatal(err)
	}
	if err := r.Commit("init", Bot); err != nil {
		t.Fatal(err)
	}
	return r, remote
}

func write(t *testing.T, r Repo, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(r.Dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %s", args, out)
	}
	return strings.TrimSpace(string(out))
}

func TestCommitTagPush(t *testing.T) {
	r, remote := setup(t)

	staged, err := r.HasStagedChanges()
	if err != nil || staged {
		t.Fatalf("HasStagedChanges = %v, %v", staged, err)
	}

	write(t, r, "a.txt", "b")
	if err := r.Add("a.txt"); err != nil {
		t.Fatal(err)
	}
	if staged, err := r.HasStagedChanges(); err != nil || !staged {
		t.Fatalf("HasStagedChanges = %v, %v", staged, err)
	}

	if err := r.Commit("chore: bump", Bot); err != nil {
		t.Fatal(err)
	}
	if err := r.Tag("v1.0.0", "Release v1.0.0", Bot); err != nil {
		t.Fatal(err)
	}
	head, err := r.Head()
	if err != nil {
		t.Fatal(err)
	}

	if err := r.PushAtomic("origin", "HEAD:refs/heads/main", "refs/tags/v1.0.0"); err != nil {
		t.Fatal(err)
	}

	if got := git(t, remote, "rev-parse", "main"); got != head {
		t.Errorf("remote main = %s, want %s", got, head)
	}
	if got := git(t, remote, "rev-parse", "v1.0.0^{commit}"); got != head {
		t.Errorf("remote tag = %s, want %s", got, head)
	}
	if got := git(t, remote, "cat-file", "-t", "v1.0.0"); got != "tag" {
		t.Errorf("tag type = %s, want annotated", got)
	}
	if got := git(t, remote, "log", "-1", "--format=%an <%ae>|%cn|%s", "main"); got != "github-actions[bot] <41898282+github-actions[bot]@users.noreply.github.com>|github-actions[bot]|chore: bump" {
		t.Errorf("commit = %s", got)
	}
}

func TestPushAtomicRejectsAll(t *testing.T) {
	r, remote := setup(t)
	if err := r.PushAtomic("origin", "HEAD:refs/heads/main"); err != nil {
		t.Fatal(err)
	}

	// Another commit lands on the remote first.
	other := filepath.Join(t.TempDir(), "other")
	git(t, ".", "clone", "-q", remote, other)
	os.WriteFile(filepath.Join(other, "b.txt"), []byte("b"), 0o644)
	git(t, other, "add", "b.txt")
	git(t, other, "-c", "user.name=x", "-c", "user.email=x@x", "commit", "-q", "-m", "other")
	git(t, other, "push", "-q", "origin", "main")

	write(t, r, "a.txt", "c")
	r.Add("a.txt")
	r.Commit("chore: bump", Bot)
	r.Tag("v1.0.0", "Release", Bot)

	err := r.PushAtomic("origin", "HEAD:refs/heads/main", "refs/tags/v1.0.0")
	if err == nil {
		t.Fatal("push did not fail")
	}
	if !strings.Contains(err.Error(), "git push --atomic origin") {
		t.Errorf("err = %v", err)
	}
	if tags := git(t, remote, "tag"); tags != "" {
		t.Errorf("remote has tags %q after a failed atomic push", tags)
	}
}

func TestErrorMessage(t *testing.T) {
	r, _ := setup(t)
	err := r.Add("missing.txt")
	if err == nil || !strings.Contains(err.Error(), "missing.txt") {
		t.Errorf("err = %v", err)
	}
}
