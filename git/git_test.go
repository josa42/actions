package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
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

func TestChangedSinceAndSubjects(t *testing.T) {
	r, _ := setup(t)
	git(t, r.Dir, "tag", "v1.0.0")

	os.MkdirAll(filepath.Join(r.Dir, "app"), 0o755)
	os.MkdirAll(filepath.Join(r.Dir, "server"), 0o755)
	write(t, r, "server/main.go", "package main")
	r.Add("server/main.go")
	r.Commit("feat(server): add main", Bot)
	write(t, r, "app/index.js", "")
	r.Add("app/index.js")
	r.Commit("fix(app): add index", Bot)
	git(t, r.Dir, "tag", "v1.1.0")
	write(t, r, "app/index.js", "x")
	r.Add("app/index.js")
	r.Commit("feat(app): change index", Bot)

	tests := []struct {
		rev, path string
		want      bool
	}{
		{"v1.0.0", "app", true},
		{"v1.0.0", "server", true},
		{"v1.1.0", "app", true},
		{"v1.1.0", "server", false},
		{"v1.1.0", "missing", false},
		{"HEAD", "a.txt", false},
	}
	for _, tt := range tests {
		if got, err := r.ChangedSince(tt.rev, tt.path); err != nil || got != tt.want {
			t.Errorf("ChangedSince(%s, %s) = %v, %v, want %v", tt.rev, tt.path, got, err, tt.want)
		}
	}

	// Uncommitted changes count too.
	write(t, r, "server/main.go", "package server")
	if got, err := r.ChangedSince("v1.1.0", "server"); err != nil || !got {
		t.Errorf("ChangedSince with a modified file = %v, %v", got, err)
	}

	if _, err := r.ChangedSince("v9.9.9", "app"); err == nil {
		t.Error("ChangedSince of a missing tag did not fail")
	}

	for rev, want := range map[string][]string{
		"v1.0.0": {"feat(server): add main", "fix(app): add index", "feat(app): change index"},
		"v1.1.0": {"feat(app): change index"},
		"HEAD":   nil,
		"":       {"init", "feat(server): add main", "fix(app): add index", "feat(app): change index"},
	} {
		if got, err := r.Subjects(rev); err != nil || !slices.Equal(got, want) {
			t.Errorf("Subjects(%q) = %q, %v, want %q", rev, got, err, want)
		}
	}
}
