package github

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
)

// Repo identifies a repository.
type Repo struct {
	Owner string
	Name  string
}

func (r Repo) String() string { return r.Owner + "/" + r.Name }

// Context describes the running workflow.
type Context struct {
	EventName string
	Repo      Repo
	Event     Event
}

// Event holds the fields of the event payload this package uses.
type Event struct {
	PullRequest *struct {
		Number int `json:"number"`
	} `json:"pull_request"`
	Issue *struct {
		Number      int       `json:"number"`
		PullRequest *struct{} `json:"pull_request"`
	} `json:"issue"`
	Before     string `json:"before"`
	After      string `json:"after"`
	Deployment *struct {
		SHA string `json:"sha"`
	} `json:"deployment"`
	MergeGroup *struct {
		HeadRef string `json:"head_ref"`
	} `json:"merge_group"`
	Repository struct {
		DefaultBranch string `json:"default_branch"`
	} `json:"repository"`
}

// LoadContext reads the workflow context from the runner environment.
func LoadContext() (*Context, error) {
	ctx := &Context{EventName: os.Getenv("GITHUB_EVENT_NAME")}

	owner, name, ok := strings.Cut(os.Getenv("GITHUB_REPOSITORY"), "/")
	if !ok {
		return nil, errors.New("GITHUB_REPOSITORY is not set")
	}
	ctx.Repo = Repo{owner, name}

	if path := os.Getenv("GITHUB_EVENT_PATH"); path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(data, &ctx.Event); err != nil {
			return nil, fmt.Errorf("event payload: %w", err)
		}
	}

	return ctx, nil
}

// isNullSHA reports whether sha is unset or all zeros, as in the before of a
// push creating a branch or the after of a push deleting one.
func isNullSHA(sha string) bool {
	return strings.Trim(sha, "0") == ""
}
