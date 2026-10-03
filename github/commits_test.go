package github

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"testing"
)

func commitJSON(sha, msg string, parents ...string) map[string]any {
	var ps []map[string]string
	for _, p := range parents {
		ps = append(ps, map[string]string{"sha": p})
	}
	return map[string]any{"sha": sha, "commit": map[string]string{"message": msg}, "parents": ps}
}

func shas(commits []Commit) []string {
	var out []string
	for _, c := range commits {
		out = append(out, c.SHA)
	}
	return out
}

func TestPullRequestCommits(t *testing.T) {
	f, c := newFakeAPI(t)
	f.json("GET /repos/o/r/pulls/1/commits", []any{commitJSON("a", "feat: a", "p"), commitJSON("m", "Merge", "a", "b")})

	got, err := c.PullRequestCommits(context.Background(), repo, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(shas(got), []string{"a", "m"}) || got[0].Commit.Message != "feat: a" {
		t.Errorf("got %+v", got)
	}
	if got[0].IsMerge() || !got[1].IsMerge() {
		t.Error("IsMerge")
	}
}

func TestCompareCommitsPaginates(t *testing.T) {
	f, c := newFakeAPI(t)
	f.handle("GET /repos/o/r/compare/aaa...bbb", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("page") == "2" {
			fmt.Fprint(w, `{"commits":[{"sha":"3"}]}`)
			return
		}
		w.Header().Set("Link", fmt.Sprintf(`<%s/repos/o/r/compare/aaa...bbb?per_page=100&page=2>; rel="next"`, f.srv.URL))
		fmt.Fprint(w, `{"commits":[{"sha":"1"},{"sha":"2"}]}`)
	})

	got, err := c.CompareCommits(context.Background(), repo, "aaa", "bbb")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(shas(got), []string{"1", "2", "3"}) {
		t.Errorf("got %v", shas(got))
	}
}

func TestPushCommits(t *testing.T) {
	tests := []struct {
		name    string
		event   string
		payload string
		want    []string
	}{
		{"push", "push", `{"before":"aaa","after":"bbb"}`, []string{"push"}},
		{"new branch", "push", `{"before":"0000000","after":"bbb","repository":{"default_branch":"main"}}`, []string{"branch"}},
		{"deleted branch", "push", `{"before":"aaa","after":"0000000"}`, nil},
		{"pull request", "pull_request", `{"before":"aaa","after":"bbb"}`, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, c := newFakeAPI(t)
			f.json("GET /repos/o/r/compare/aaa...bbb", map[string]any{"commits": []any{commitJSON("push", "x")}})
			f.json("GET /repos/o/r/compare/main...bbb", map[string]any{"commits": []any{commitJSON("branch", "x")}})

			got, err := c.PushCommits(context.Background(), event(t, tt.event, tt.payload))
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(shas(got), tt.want) {
				t.Errorf("got %v, want %v", shas(got), tt.want)
			}
		})
	}
}
