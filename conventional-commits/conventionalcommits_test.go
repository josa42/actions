package conventionalcommits

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/josa42/actions/conventional"
)

func setInputs(t *testing.T, env map[string]string) {
	t.Helper()
	defaults := map[string]string{
		"TYPES": "", "SCOPES": "", "MAX-LENGTH": "72",
		"LOWERCASE": "true", "NO-TRAILING-PERIOD": "true", "COMMENT": "true",
		"GITHUB-TOKEN": "token",
	}
	for k, v := range defaults {
		if o, ok := env[k]; ok {
			v = o
		}
		t.Setenv("INPUT_"+k, v)
	}
}

func TestReadInputs(t *testing.T) {
	setInputs(t, map[string]string{"TYPES": "feat\nfix", "SCOPES": "a\n\nb", "MAX-LENGTH": "50", "COMMENT": "false"})
	in, err := readInputs()
	if err != nil {
		t.Fatal(err)
	}
	want := conventional.Config{Types: []string{"feat", "fix"}, Scopes: []string{"a", "b"}, MaxLength: 50, Lowercase: true, NoPeriod: true}
	if fmt.Sprint(in.cfg) != fmt.Sprint(want) || in.comment {
		t.Errorf("got %+v", in)
	}

	for name, env := range map[string]map[string]string{
		"max-length":         {"MAX-LENGTH": "x"},
		"negative length":    {"MAX-LENGTH": "-1"},
		"lowercase":          {"LOWERCASE": "yes"},
		"no-trailing-period": {"NO-TRAILING-PERIOD": "1"},
		"comment":            {"COMMENT": "on"},
	} {
		setInputs(t, env)
		if _, err := readInputs(); err == nil {
			t.Errorf("%s: did not fail", name)
		}
	}
}

func TestCheck(t *testing.T) {
	cfg := conventional.Config{MaxLength: 72, Lowercase: true, NoPeriod: true}
	results := check([]item{
		{label: "PR title", message: "feat: add x", title: true},
		{label: "PR title", message: "Add x", title: true},
		{label: "abc", message: "fix: x\nbody without blank line"},
		{label: "def", message: "fix: y\n\nbody"},
	}, cfg)

	for i, valid := range []bool{true, false, false, true} {
		if (len(results[i].violations) == 0) != valid {
			t.Errorf("%d %q: %q", i, results[i].message, results[i].violations)
		}
	}
}

func TestTable(t *testing.T) {
	got := table([]result{
		{item: item{label: "abc1234", url: "https://x/c/abc", message: "feat: a|b\n\nbody"}},
		{item: item{label: "PR title", message: "use `x`"}, violations: []string{"one", "two"}},
	}, true)
	want := "| | Message | Header | Problems |\n| --- | --- | --- | --- |\n" +
		"| ✅ | [abc1234](https://x/c/abc) | `feat: a\\|b` |  |\n" +
		"| ❌ | PR title | `` use `x` `` | one<br>two |\n"
	if got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}

type fakeGitHub struct {
	mu       sync.Mutex
	title    string
	commits  []map[string]any
	comments []map[string]any
	readOnly bool
}

func commit(sha, msg string, parents int) map[string]any {
	var ps []map[string]string
	for i := range parents {
		ps = append(ps, map[string]string{"sha": fmt.Sprint("p", i)})
	}
	return map[string]any{"sha": sha, "html_url": "https://github.com/o/r/commit/" + sha, "commit": map[string]string{"message": msg}, "parents": ps}
}

func (f *fakeGitHub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	switch key := r.Method + " " + r.URL.Path; {
	case key == "GET /repos/o/r/pulls/1":
		json.NewEncoder(w).Encode(map[string]any{"number": 1, "title": f.title, "html_url": "https://github.com/o/r/pull/1"})
	case key == "GET /repos/o/r/pulls/1/commits":
		json.NewEncoder(w).Encode(f.commits)
	case key == "GET /repos/o/r/compare/aaa...bbb":
		json.NewEncoder(w).Encode(map[string]any{"commits": f.commits})
	case key == "GET /repos/o/r/issues/1/comments":
		json.NewEncoder(w).Encode(f.comments)
	case f.readOnly && r.Method != "GET":
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprint(w, `{"message":"Resource not accessible by integration"}`)
	case key == "POST /repos/o/r/issues/1/comments":
		var in map[string]any
		json.NewDecoder(r.Body).Decode(&in)
		in["id"] = len(f.comments) + 1
		f.comments = append(f.comments, in)
		json.NewEncoder(w).Encode(in)
	case strings.HasPrefix(key, "PATCH /repos/o/r/issues/comments/"):
		var in map[string]any
		json.NewDecoder(r.Body).Decode(&in)
		f.comments[0]["body"] = in["body"]
		json.NewEncoder(w).Encode(f.comments[0])
	case strings.HasPrefix(key, "DELETE /repos/o/r/issues/comments/"):
		f.comments = nil
		w.WriteHeader(http.StatusNoContent)
	default:
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"message":"Not Found"}`)
	}
}

func setupRun(t *testing.T, f *fakeGitHub, eventName, payload string) string {
	t.Helper()
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)

	dir := t.TempDir()
	event := filepath.Join(dir, "event.json")
	os.WriteFile(event, []byte(payload), 0o644)
	summary := filepath.Join(dir, "summary.md")
	os.WriteFile(summary, nil, 0o644)

	t.Setenv("GITHUB_API_URL", srv.URL)
	t.Setenv("GITHUB_REPOSITORY", "o/r")
	t.Setenv("GITHUB_EVENT_NAME", eventName)
	t.Setenv("GITHUB_EVENT_PATH", event)
	t.Setenv("GITHUB_STEP_SUMMARY", summary)
	setInputs(t, nil)
	return summary
}

func read(t *testing.T, path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

const prEvent = `{"pull_request":{"number":1}}`

func TestRunPullRequestValid(t *testing.T) {
	f := &fakeGitHub{title: "feat: add x", commits: []map[string]any{
		commit("aaaaaaa1", "feat: add x", 1),
		commit("bbbbbbb2", "Merge branch 'main' into x", 2),
		commit("ccccccc3", "Revert \"fix: y\"\n\nThis reverts commit 123.", 1),
	}}
	summary := setupRun(t, f, "pull_request", prEvent)

	if err := Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	s := read(t, summary)
	if !strings.Contains(s, "| ✅ | [PR title]") || !strings.Contains(s, "aaaaaaa") || strings.Contains(s, "bbbbbbb") {
		t.Errorf("summary:\n%s", s)
	}
	if len(f.comments) != 0 {
		t.Errorf("comments = %v", f.comments)
	}
}

func TestRunPullRequestInvalid(t *testing.T) {
	f := &fakeGitHub{title: "Add x", commits: []map[string]any{
		commit("aaaaaaa1", "feat: add x", 1),
		commit("bbbbbbb2", "fixup! feat: add x", 1),
	}}
	summary := setupRun(t, f, "pull_request", prEvent)

	err := Run(context.Background())
	if err == nil || err.Error() != "2 of 3 messages don't follow Conventional Commits" {
		t.Fatalf("err = %v", err)
	}
	if s := read(t, summary); strings.Count(s, "❌") != 2 || strings.Count(s, "✅") != 1 {
		t.Errorf("summary:\n%s", s)
	}
	if len(f.comments) != 1 {
		t.Fatalf("comments = %v", f.comments)
	}
	body := f.comments[0]["body"].(string)
	for _, want := range []string{"2 of 3 messages", "PR title", "bbbbbbb", "squashed before merging", "<!-- comment-identifier: conventional-commits -->"} {
		if !strings.Contains(body, want) {
			t.Errorf("comment misses %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "aaaaaaa") {
		t.Errorf("comment lists a valid commit:\n%s", body)
	}

	// Fixing one message updates the comment in place.
	f.title = "feat: add x"
	if err := Run(context.Background()); err == nil {
		t.Fatal("did not fail")
	}
	if len(f.comments) != 1 || !strings.Contains(f.comments[0]["body"].(string), "1 of 3 messages") {
		t.Errorf("comments = %v", f.comments)
	}

	// Fixing all deletes it.
	f.commits = f.commits[:1]
	if err := Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(f.comments) != 0 {
		t.Errorf("comments = %v", f.comments)
	}
}

func TestRunReadOnlyToken(t *testing.T) {
	f := &fakeGitHub{title: "Add x", readOnly: true}
	setupRun(t, f, "pull_request_target", prEvent)

	// The check result stands, the failed comment is only a warning.
	err := Run(context.Background())
	if err == nil || err.Error() != "1 of 1 messages don't follow Conventional Commits" {
		t.Errorf("err = %v", err)
	}
}

func TestRunCommentDisabled(t *testing.T) {
	f := &fakeGitHub{title: "Add x"}
	setupRun(t, f, "pull_request", prEvent)
	t.Setenv("INPUT_COMMENT", "false")

	if err := Run(context.Background()); err == nil {
		t.Fatal("did not fail")
	}
	if len(f.comments) != 0 {
		t.Errorf("comments = %v", f.comments)
	}
}

func TestRunPush(t *testing.T) {
	f := &fakeGitHub{commits: []map[string]any{commit("aaaaaaa1", "feat: add x", 1), commit("bbbbbbb2", "Add y", 1)}}
	summary := setupRun(t, f, "push", `{"before":"aaa","after":"bbb"}`)

	err := Run(context.Background())
	if err == nil || err.Error() != "1 of 2 messages don't follow Conventional Commits" {
		t.Fatalf("err = %v", err)
	}
	if s := read(t, summary); strings.Contains(s, "PR title") {
		t.Errorf("summary:\n%s", s)
	}
	if len(f.comments) != 0 {
		t.Errorf("push commented: %v", f.comments)
	}
}

func TestRunOtherEvent(t *testing.T) {
	setupRun(t, &fakeGitHub{}, "workflow_dispatch", `{}`)
	if err := Run(context.Background()); err != nil {
		t.Fatal(err)
	}
}
