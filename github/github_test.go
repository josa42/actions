package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeAPI is a GitHub API test double. Routes are keyed by "METHOD /path",
// without the query.
type fakeAPI struct {
	t      *testing.T
	srv    *httptest.Server
	mu     sync.Mutex
	routes map[string]http.HandlerFunc
	calls  []string
}

func newFakeAPI(t *testing.T) (*fakeAPI, *Client) {
	f := &fakeAPI{t: t, routes: map[string]http.HandlerFunc{}}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Method + " " + r.URL.Path
		f.mu.Lock()
		f.calls = append(f.calls, key)
		h, ok := f.routes[key]
		f.mu.Unlock()
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			io.WriteString(w, `{"message":"Not Found"}`)
			return
		}
		h(w, r)
	}))
	t.Cleanup(f.srv.Close)

	c := newClient(f.srv.URL, "token")
	c.sleep = func(context.Context, time.Duration) error { return nil }
	return f, c
}

func (f *fakeAPI) handle(key string, h http.HandlerFunc) {
	f.routes[key] = h
}

func (f *fakeAPI) json(key string, v any) {
	f.handle(key, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(v)
	})
}

func (f *fakeAPI) called(key string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, c := range f.calls {
		if c == key {
			n++
		}
	}
	return n
}

var repo = Repo{"o", "r"}

func TestClientHeaders(t *testing.T) {
	f, c := newFakeAPI(t)
	f.handle("GET /x", func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer token" {
			t.Errorf("Authorization = %q", got)
		}
		if got := r.Header.Get("Accept"); got != "application/vnd.github+json" {
			t.Errorf("Accept = %q", got)
		}
		io.WriteString(w, `{}`)
	})

	if _, err := c.get(context.Background(), "/x", nil); err != nil {
		t.Fatal(err)
	}
}

func TestClientPagination(t *testing.T) {
	f, c := newFakeAPI(t)
	f.handle("GET /items", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("per_page") != "100" {
			t.Errorf("per_page = %q", r.URL.Query().Get("per_page"))
		}
		switch r.URL.Query().Get("page") {
		case "":
			w.Header().Set("Link", fmt.Sprintf(`<%s/items?per_page=100&page=2>; rel="next", <%[1]s/items?per_page=100&page=3>; rel="last"`, f.srv.URL))
			io.WriteString(w, `[1,2]`)
		case "2":
			w.Header().Set("Link", fmt.Sprintf(`<%s/items?per_page=100&page=3>; rel="next"`, f.srv.URL))
			io.WriteString(w, `[3]`)
		case "3":
			io.WriteString(w, `[4]`)
		}
	})

	got, err := getAll[int](context.Background(), c, "/items")
	if err != nil {
		t.Fatal(err)
	}
	if want := []int{1, 2, 3, 4}; !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestClientError(t *testing.T) {
	f, c := newFakeAPI(t)
	f.handle("GET /x", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		io.WriteString(w, `{"message":"Validation Failed"}`)
	})

	_, err := c.get(context.Background(), "/x", nil)
	if err == nil || err.Error() != "GET /x: 422 Validation Failed" {
		t.Errorf("err = %v", err)
	}
	if IsNotFound(err) {
		t.Error("IsNotFound(422)")
	}

	_, err = c.get(context.Background(), "/missing", nil)
	if !IsNotFound(err) {
		t.Errorf("IsNotFound(%v) = false", err)
	}
}

func failing(n int, status int, header http.Header) http.HandlerFunc {
	var mu sync.Mutex
	return func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if n > 0 {
			n--
			for k, v := range header {
				w.Header()[k] = v
			}
			w.WriteHeader(status)
			io.WriteString(w, `{"message":"failed"}`)
			return
		}
		io.WriteString(w, `{}`)
	}
}

func TestClientRetries(t *testing.T) {
	soon := fmt.Sprint(time.Now().Add(10 * time.Second).Unix())
	late := fmt.Sprint(time.Now().Add(time.Hour).Unix())

	tests := []struct {
		name   string
		method string
		fails  int
		status int
		header http.Header
		calls  int
		ok     bool
	}{
		{"server error", "GET", 2, 502, nil, 3, true},
		{"server error gives up", "GET", 3, 502, nil, 3, false},
		{"server error on patch", "PATCH", 1, 500, nil, 2, true},
		{"server error on post", "POST", 1, 502, nil, 1, false},
		{"client error", "GET", 1, 400, nil, 1, false},
		{"too many requests", "POST", 1, 429, nil, 2, true},
		{"retry after", "POST", 1, 403, http.Header{"Retry-After": {"5"}}, 2, true},
		{"retry after too long", "GET", 1, 403, http.Header{"Retry-After": {"3600"}}, 1, false},
		{"rate limit reset", "GET", 1, 403, http.Header{"X-Ratelimit-Remaining": {"0"}, "X-Ratelimit-Reset": {soon}}, 2, true},
		{"rate limit reset too late", "GET", 1, 403, http.Header{"X-Ratelimit-Remaining": {"0"}, "X-Ratelimit-Reset": {late}}, 1, false},
		{"forbidden", "GET", 1, 403, nil, 1, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, c := newFakeAPI(t)
			f.handle(tt.method+" /x", failing(tt.fails, tt.status, tt.header))

			_, _, err := c.do(context.Background(), tt.method, "/x", nil, nil)
			if (err == nil) != tt.ok {
				t.Errorf("err = %v", err)
			}
			if n := f.called(tt.method + " /x"); n != tt.calls {
				t.Errorf("calls = %d, want %d", n, tt.calls)
			}
		})
	}
}

func TestLoadContext(t *testing.T) {
	path := filepath.Join(t.TempDir(), "event.json")
	os.WriteFile(path, []byte(`{"pull_request":{"number":7},"repository":{"default_branch":"main"}}`), 0o644)
	t.Setenv("GITHUB_REPOSITORY", "o/r")
	t.Setenv("GITHUB_EVENT_NAME", "pull_request")
	t.Setenv("GITHUB_EVENT_PATH", path)

	gh, err := LoadContext()
	if err != nil {
		t.Fatal(err)
	}
	if gh.Repo != repo || gh.EventName != "pull_request" || gh.Event.PullRequest.Number != 7 || gh.Event.Repository.DefaultBranch != "main" {
		t.Errorf("unexpected context %+v", gh)
	}

	t.Setenv("GITHUB_REPOSITORY", "")
	if _, err := LoadContext(); err == nil {
		t.Error("LoadContext without GITHUB_REPOSITORY did not fail")
	}
}

func event(t *testing.T, name, payload string) *Context {
	t.Helper()
	gh := &Context{EventName: name, Repo: repo}
	if err := json.Unmarshal([]byte(payload), &gh.Event); err != nil {
		t.Fatal(err)
	}
	return gh
}

func pr(number int, state, headSHA string) map[string]any {
	return map[string]any{"number": number, "state": state, "head": map[string]any{"sha": headSHA}}
}

func TestFindPullRequest(t *testing.T) {
	tests := []struct {
		name    string
		event   string
		payload string
		want    int
	}{
		{"pull_request", "pull_request", `{"pull_request":{"number":1}}`, 1},
		{"pull_request_target", "pull_request_target", `{"pull_request":{"number":1}}`, 1},
		{"issue_comment on pull request", "issue_comment", `{"issue":{"number":1,"pull_request":{}}}`, 1},
		{"issue_comment on issue", "issue_comment", `{"issue":{"number":1}}`, 0},
		{"merge_group", "merge_group", `{"merge_group":{"head_ref":"refs/heads/gh-readonly-queue/main/pr-1-0123abcd"}}`, 1},
		{"merge_group nested base", "merge_group", `{"merge_group":{"head_ref":"refs/heads/gh-readonly-queue/release/v2/pr-1-0123abcd"}}`, 1},
		{"merge_group unknown ref", "merge_group", `{"merge_group":{"head_ref":"refs/heads/other"}}`, 0},
		{"push", "push", `{"after":"abc"}`, 2},
		{"push without pull request", "push", `{"after":"def"}`, 0},
		{"push deleting branch", "push", `{"after":"0000000000000000000000000000000000000000"}`, 0},
		{"deployment", "deployment", `{"deployment":{"sha":"abc"}}`, 2},
		{"deployment_status", "deployment_status", `{"deployment":{"sha":"abc"}}`, 2},
		{"workflow_dispatch", "workflow_dispatch", `{}`, 0},
		{"schedule", "schedule", `{}`, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, c := newFakeAPI(t)
			f.json("GET /repos/o/r/pulls/1", map[string]any{"number": 1, "title": "one", "unknown_field": true})
			f.json("GET /repos/o/r/pulls/2", map[string]any{"number": 2, "title": "two"})
			f.json("GET /repos/o/r/commits/abc/pulls", []any{pr(2, "open", "abc")})
			f.json("GET /repos/o/r/commits/def/pulls", []any{pr(3, "closed", "def")})

			got, err := c.FindPullRequest(context.Background(), event(t, tt.event, tt.payload))
			if err != nil {
				t.Fatal(err)
			}
			if tt.want == 0 {
				if got != nil {
					t.Errorf("got #%d, want none", got.Number)
				}
				return
			}
			if got == nil || got.Number != tt.want {
				t.Fatalf("got %+v, want #%d", got, tt.want)
			}
			// Always the full object from the pulls endpoint.
			var raw map[string]any
			json.Unmarshal(got.Raw, &raw)
			if raw["title"] == nil {
				t.Errorf("Raw = %s", got.Raw)
			}
		})
	}
}

func TestOpenPullRequestForCommit(t *testing.T) {
	f, c := newFakeAPI(t)
	f.json("GET /repos/o/r/commits/abc/pulls", []any{pr(1, "closed", "abc"), pr(2, "open", "old"), pr(3, "open", "abc")})
	f.json("GET /repos/o/r/commits/def/pulls", []any{pr(1, "closed", "def"), pr(2, "open", "other")})
	f.json("GET /repos/o/r/pulls/2", pr(2, "open", "other"))
	f.json("GET /repos/o/r/pulls/3", pr(3, "open", "abc"))

	for sha, want := range map[string]int{"abc": 3, "def": 2} {
		got, err := c.OpenPullRequestForCommit(context.Background(), repo, sha)
		if err != nil {
			t.Fatal(err)
		}
		if got == nil || got.Number != want {
			t.Errorf("%s: got %+v, want #%d", sha, got, want)
		}
	}
}

func files(paths ...string) []map[string]string {
	var out []map[string]string
	for _, p := range paths {
		out = append(out, map[string]string{"filename": p, "status": "modified"})
	}
	return out
}

func TestChangedFiles(t *testing.T) {
	tests := []struct {
		name    string
		event   string
		payload string
		pr      *PullRequest
		want    []string
		limit   int
	}{
		{"pull request", "pull_request", `{}`, &PullRequest{Number: 1}, []string{"pr.go"}, MaxPullRequestFiles},
		{"push with pull request", "push", `{"before":"aaa","after":"bbb"}`, &PullRequest{Number: 1}, []string{"pr.go"}, MaxPullRequestFiles},
		{"push", "push", `{"before":"aaa","after":"bbb"}`, nil, []string{"push.go"}, MaxCompareFiles},
		{"push new branch", "push", `{"before":"0000000","after":"bbb","repository":{"default_branch":"main"}}`, nil, []string{"branch.go"}, MaxCompareFiles},
		{"push deleting branch", "push", `{"before":"aaa","after":"0000000"}`, nil, nil, 0},
		{"workflow_dispatch", "workflow_dispatch", `{}`, nil, nil, 0},
		{"deployment without pull request", "deployment", `{"deployment":{"sha":"bbb"}}`, nil, nil, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, c := newFakeAPI(t)
			f.json("GET /repos/o/r/pulls/1/files", files("pr.go"))
			f.json("GET /repos/o/r/compare/aaa...bbb", map[string]any{"files": files("push.go")})
			f.json("GET /repos/o/r/compare/main...bbb", map[string]any{"files": files("branch.go")})

			got, limit, err := c.ChangedFiles(context.Background(), event(t, tt.event, tt.payload), tt.pr)
			if err != nil {
				t.Fatal(err)
			}
			var paths []string
			for _, f := range got {
				paths = append(paths, f.Path)
			}
			if !slices.Equal(paths, tt.want) || limit != tt.limit {
				t.Errorf("got %q, %d, want %q, %d", paths, limit, tt.want, tt.limit)
			}
		})
	}
}

func TestChangedFileFields(t *testing.T) {
	f, c := newFakeAPI(t)
	f.json("GET /repos/o/r/pulls/1/files", []map[string]string{
		{"filename": "new.go", "previous_filename": "old.go", "status": "renamed"},
		{"filename": "gone.go", "status": "removed"},
	})

	got, err := c.PullRequestFiles(context.Background(), repo, 1)
	if err != nil {
		t.Fatal(err)
	}
	want := []ChangedFile{{"new.go", "old.go", "renamed"}, {"gone.go", "", "removed"}}
	if !slices.Equal(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
	if !got[0].Exists() || got[1].Exists() {
		t.Error("Exists")
	}
}

// fakeComments serves the comments of issue 1 and records writes.
type fakeComments struct {
	f        *fakeAPI
	mu       sync.Mutex
	comments []Comment
	nextID   int64
}

func newFakeComments(t *testing.T, existing ...Comment) (*fakeComments, *Client) {
	f, c := newFakeAPI(t)
	fc := &fakeComments{f: f, comments: existing, nextID: 100}

	f.handle("GET /repos/o/r/issues/1/comments", func(w http.ResponseWriter, r *http.Request) {
		fc.mu.Lock()
		defer fc.mu.Unlock()
		json.NewEncoder(w).Encode(fc.comments)
	})
	f.handle("POST /repos/o/r/issues/1/comments", func(w http.ResponseWriter, r *http.Request) {
		fc.mu.Lock()
		defer fc.mu.Unlock()
		var in Comment
		json.NewDecoder(r.Body).Decode(&in)
		in.ID = fc.nextID
		in.HTMLURL = fmt.Sprintf("https://github.com/o/r/pull/1#issuecomment-%d", in.ID)
		in.User.Login = "github-actions[bot]"
		fc.nextID++
		fc.comments = append(fc.comments, in)
		json.NewEncoder(w).Encode(in)
	})
	for _, cm := range existing {
		path := fmt.Sprintf("/repos/o/r/issues/comments/%d", cm.ID)
		f.handle("PATCH "+path, func(w http.ResponseWriter, r *http.Request) {
			fc.mu.Lock()
			defer fc.mu.Unlock()
			var in Comment
			json.NewDecoder(r.Body).Decode(&in)
			for i := range fc.comments {
				if fc.comments[i].ID == cm.ID {
					fc.comments[i].Body = in.Body
					json.NewEncoder(w).Encode(fc.comments[i])
				}
			}
		})
		f.handle("DELETE "+path, func(w http.ResponseWriter, r *http.Request) {
			fc.mu.Lock()
			defer fc.mu.Unlock()
			fc.comments = slices.DeleteFunc(fc.comments, func(c Comment) bool { return c.ID == cm.ID })
			w.WriteHeader(http.StatusNoContent)
		})
	}
	return fc, c
}

func (fc *fakeComments) bodies() map[int64]string {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	out := map[int64]string{}
	for _, c := range fc.comments {
		out[c.ID] = c.Body
	}
	return out
}

func comment(id int64, login, body string) Comment {
	c := Comment{ID: id, Body: body}
	c.User.Login = login
	return c
}

var target = CommentTarget{Repo: repo, Issue: 1, Identifier: "lint"}

const marker = "\n\n<!-- comment-identifier: lint -->"

func TestPostCommentWithoutIdentifier(t *testing.T) {
	fc, c := newFakeComments(t, comment(1, "bot", "hi"))

	cm, _, err := c.PostComment(context.Background(), CommentTarget{Repo: repo, Issue: 1}, "hi")
	if err != nil {
		t.Fatal(err)
	}
	if cm.ID != 100 || cm.HTMLURL == "" {
		t.Errorf("comment = %+v", cm)
	}
	if want := map[int64]string{1: "hi", 100: "hi"}; !maps.Equal(fc.bodies(), want) {
		t.Errorf("comments = %q", fc.bodies())
	}
}

func TestPostCommentCreates(t *testing.T) {
	fc, c := newFakeComments(t, comment(1, "bot", "other"+strings.Replace(marker, "lint", "other", 1)))

	cm, _, err := c.PostComment(context.Background(), target, "body")
	if err != nil {
		t.Fatal(err)
	}
	if cm.ID != 100 {
		t.Errorf("comment = %+v", cm)
	}
	if got := fc.bodies()[100]; got != "body"+marker {
		t.Errorf("body = %q", got)
	}
}

func TestPostCommentUpdatesNewestAndDeletesOlder(t *testing.T) {
	fc, c := newFakeComments(t,
		comment(1, "bot", "old 1"+marker),
		comment(2, "human", "unrelated"),
		comment(3, "bot", "old 2"+marker),
	)

	cm, _, err := c.PostComment(context.Background(), target, "new")
	if err != nil {
		t.Fatal(err)
	}
	if cm.ID != 3 {
		t.Errorf("updated %d, want 3", cm.ID)
	}
	if want := map[int64]string{2: "unrelated", 3: "new" + marker}; !maps.Equal(fc.bodies(), want) {
		t.Errorf("comments = %q", fc.bodies())
	}
}

func TestPostCommentUnchanged(t *testing.T) {
	fc, c := newFakeComments(t, comment(1, "bot", "same"+marker))

	cm, _, err := c.PostComment(context.Background(), target, "same")
	if err != nil {
		t.Fatal(err)
	}
	if cm.ID != 1 {
		t.Errorf("comment = %+v", cm)
	}
	if n := fc.f.called("PATCH /repos/o/r/issues/comments/1"); n != 0 {
		t.Errorf("updated unchanged comment %d times", n)
	}
}

func TestPostCommentAuthor(t *testing.T) {
	fc, c := newFakeComments(t,
		comment(1, "human", "quoted"+marker),
		comment(2, "bot", "old"+marker),
	)

	tg := target
	tg.Author = "bot"
	if _, _, err := c.PostComment(context.Background(), tg, "new"); err != nil {
		t.Fatal(err)
	}
	if want := map[int64]string{1: "quoted" + marker, 2: "new" + marker}; !maps.Equal(fc.bodies(), want) {
		t.Errorf("comments = %q", fc.bodies())
	}
}

func TestPostCommentTruncatesBeforeMarker(t *testing.T) {
	fc, c := newFakeComments(t)

	_, truncated, err := c.PostComment(context.Background(), target, strings.Repeat("line\n", 20000))
	if err != nil {
		t.Fatal(err)
	}
	body := fc.bodies()[100]
	if !truncated || !strings.HasSuffix(body, marker) || len(body) > MaxCommentLength {
		t.Errorf("truncated = %v, len = %d, suffix = %q", truncated, len(body), body[len(body)-60:])
	}
}

func TestDeleteComments(t *testing.T) {
	fc, c := newFakeComments(t,
		comment(1, "bot", "a"+marker),
		comment(2, "bot", "b"),
		comment(3, "bot", "c"+marker),
	)

	n, err := c.DeleteComments(context.Background(), target)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Errorf("deleted %d, want 2", n)
	}
	if want := map[int64]string{2: "b"}; !maps.Equal(fc.bodies(), want) {
		t.Errorf("comments = %q", fc.bodies())
	}

	if _, err := c.DeleteComments(context.Background(), CommentTarget{Repo: repo, Issue: 1}); err == nil {
		t.Error("DeleteComments without identifier did not fail")
	}
}

func TestCommentMarkerEscaping(t *testing.T) {
	tg := CommentTarget{Identifier: "a --> b\n%"}
	if got, want := tg.marker(), "<!-- comment-identifier: a --%3E b%0A%25 -->"; got != want {
		t.Errorf("marker = %q, want %q", got, want)
	}
	// One identifier must not match another that contains it.
	if (CommentTarget{Identifier: "lint"}).matches(comment(1, "", "<!-- comment-identifier: lint-strict -->")) {
		t.Error("lint matched lint-strict")
	}
}
