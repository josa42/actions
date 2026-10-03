package github

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// Release is a GitHub release.
type Release struct {
	ID        int64   `json:"id"`
	TagName   string  `json:"tag_name"`
	HTMLURL   string  `json:"html_url"`
	UploadURL string  `json:"upload_url"`
	Assets    []Asset `json:"assets"`
}

// Asset is a file attached to a release.
type Asset struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// ReleaseOptions are the fields of a release to create or update.
type ReleaseOptions struct {
	TagName string `json:"tag_name,omitempty"`
	Name    string `json:"name,omitempty"`
	// Body is the release notes. On create, an empty body generates notes.
	Body       string `json:"body,omitempty"`
	Draft      bool   `json:"draft"`
	Prerelease bool   `json:"prerelease"`
	Generate   bool   `json:"generate_release_notes,omitempty"`
}

// Tags lists the tag names of the repository.
func (c *Client) Tags(ctx context.Context, repo Repo) ([]string, error) {
	tags, err := getAll[struct {
		Name string `json:"name"`
	}](ctx, c, fmt.Sprintf("/repos/%s/tags", repo))
	if err != nil {
		return nil, err
	}
	names := make([]string, len(tags))
	for i, t := range tags {
		names[i] = t.Name
	}
	return names, nil
}

// TagExists reports whether the tag exists.
func (c *Client) TagExists(ctx context.Context, repo Repo, tag string) (bool, error) {
	_, err := c.get(ctx, fmt.Sprintf("/repos/%s/git/ref/tags/%s", repo, url.PathEscape(tag)), nil)
	if IsNotFound(err) {
		return false, nil
	}
	return err == nil, err
}

// DefaultBranch returns the default branch of the repository.
func (c *Client) DefaultBranch(ctx context.Context, repo Repo) (string, error) {
	var r struct {
		DefaultBranch string `json:"default_branch"`
	}
	_, err := c.get(ctx, fmt.Sprintf("/repos/%s", repo), &r)
	return r.DefaultBranch, err
}

// ReleaseByTag returns the release of a tag, or nil if there is none. Draft
// releases have no tag yet, so they are looked up in the release list.
func (c *Client) ReleaseByTag(ctx context.Context, repo Repo, tag string) (*Release, error) {
	var r Release
	_, err := c.get(ctx, fmt.Sprintf("/repos/%s/releases/tags/%s", repo, url.PathEscape(tag)), &r)
	if err == nil {
		return &r, nil
	}
	if !IsNotFound(err) {
		return nil, err
	}

	all, err := getAll[Release](ctx, c, fmt.Sprintf("/repos/%s/releases", repo))
	if err != nil {
		return nil, err
	}
	for i := range all {
		if all[i].TagName == tag {
			return &all[i], nil
		}
	}
	return nil, nil
}

// CreateRelease creates a release.
func (c *Client) CreateRelease(ctx context.Context, repo Repo, opts ReleaseOptions) (*Release, error) {
	opts.Generate = opts.Body == ""
	var r Release
	_, _, err := c.do(ctx, http.MethodPost, fmt.Sprintf("/repos/%s/releases", repo), opts, &r)
	return &r, err
}

// UpdateRelease updates a release. An empty body keeps the current notes.
func (c *Client) UpdateRelease(ctx context.Context, repo Repo, id int64, opts ReleaseOptions) (*Release, error) {
	opts.Generate = false
	var r Release
	_, _, err := c.do(ctx, http.MethodPatch, fmt.Sprintf("/repos/%s/releases/%d", repo, id), opts, &r)
	return &r, err
}

// DeleteAsset deletes a release asset.
func (c *Client) DeleteAsset(ctx context.Context, repo Repo, id int64) error {
	_, _, err := c.do(ctx, http.MethodDelete, fmt.Sprintf("/repos/%s/releases/assets/%d", repo, id), nil, nil)
	return err
}

// UploadAsset attaches a file to a release.
func (c *Client) UploadAsset(ctx context.Context, r *Release, name string, data []byte) (*Asset, error) {
	// upload_url is a URI template, e.g. .../assets{?name,label}.
	base, _, _ := strings.Cut(r.UploadURL, "{")
	var a Asset
	_, _, err := c.send(ctx, http.MethodPost, base+"?name="+url.QueryEscape(name), "application/octet-stream", data, &a)
	return &a, err
}

// AssetFile is a local file to attach to a release.
type AssetFile struct {
	Name string
	Data []byte
}

// PublishRelease creates the release of opts.TagName, or updates it if it
// exists, and uploads assets, replacing existing ones of the same name. It
// is safe to repeat after a partial failure.
func (c *Client) PublishRelease(ctx context.Context, repo Repo, opts ReleaseOptions, assets []AssetFile) (*Release, error) {
	if opts.Name == "" {
		opts.Name = opts.TagName
	}

	r, err := c.ReleaseByTag(ctx, repo, opts.TagName)
	if err != nil {
		return nil, err
	}
	if r == nil {
		r, err = c.CreateRelease(ctx, repo, opts)
	} else {
		existing := r.Assets
		r, err = c.UpdateRelease(ctx, repo, r.ID, opts)
		if err == nil && r.Assets == nil {
			r.Assets = existing
		}
	}
	if err != nil {
		return nil, err
	}

	for _, f := range assets {
		for _, a := range r.Assets {
			if a.Name == f.Name {
				if err := c.DeleteAsset(ctx, repo, a.ID); err != nil && !IsNotFound(err) {
					return nil, err
				}
			}
		}
		if _, err := c.UploadAsset(ctx, r, f.Name, f.Data); err != nil {
			return nil, fmt.Errorf("upload %s: %w", f.Name, err)
		}
	}
	return r, nil
}
