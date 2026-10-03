// Package releasepublish implements the release-publish action.
package releasepublish

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strconv"

	"github.com/josa42/actions/github"
	"github.com/josa42/actions/pathmatch"
	"github.com/josa42/actions/toolkit"
)

func Run(ctx context.Context) error {
	tag := toolkit.Input("tag")
	if tag == "" {
		return errors.New("input tag is required")
	}
	draft, err := toolkit.InputBool("draft")
	if err != nil {
		return err
	}
	prerelease, err := toolkit.InputBool("prerelease")
	if err != nil {
		return err
	}

	assets, err := collectAssets(".", toolkit.InputList("files"))
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

	r, err := client.PublishRelease(ctx, gh.Repo, github.ReleaseOptions{
		TagName:    tag,
		Body:       toolkit.Input("notes"),
		Draft:      draft,
		Prerelease: prerelease,
	}, assets)
	if err != nil {
		return err
	}
	for _, a := range assets {
		fmt.Printf("Uploaded %s\n", a.Name)
	}
	fmt.Printf("Release: %s\n", r.HTMLURL)

	if err := toolkit.SetOutput("id", strconv.FormatInt(r.ID, 10)); err != nil {
		return err
	}
	return toolkit.SetOutput("url", r.HTMLURL)
}

// collectAssets returns the files below root matching patterns. Each
// pattern must match a file, and file names must be unique, since assets are
// named by the file name alone.
func collectAssets(root string, patterns []string) ([]github.AssetFile, error) {
	var assets []github.AssetFile
	if len(patterns) == 0 {
		return assets, nil
	}

	all, err := files(root)
	if err != nil {
		return nil, err
	}

	seen := map[string]string{}
	for _, p := range patterns {
		m, err := pathmatch.Compile([]string{p})
		if err != nil {
			return nil, fmt.Errorf("input files: %w", err)
		}
		if m.Empty() {
			continue
		}

		matched := m.Filter(all)
		if len(matched) == 0 {
			return nil, fmt.Errorf("input files: %q matches no file", p)
		}
		for _, f := range matched {
			name := path.Base(f)
			if other, ok := seen[name]; ok {
				if other == f {
					continue
				}
				return nil, fmt.Errorf("input files: %s and %s would both be named %s", other, f, name)
			}
			seen[name] = f

			data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(f)))
			if err != nil {
				return nil, err
			}
			assets = append(assets, github.AssetFile{Name: name, Data: data})
		}
	}
	return assets, nil
}

// files lists the regular files below root as slash separated paths,
// skipping .git.
func files(root string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && d.Name() == ".git" {
			return filepath.SkipDir
		}
		if !d.Type().IsRegular() {
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		out = append(out, filepath.ToSlash(rel))
		return nil
	})
	return out, err
}
