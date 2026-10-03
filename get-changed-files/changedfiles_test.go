package changedfiles

import (
	"slices"
	"testing"

	"github.com/josa42/actions/github"
	"github.com/josa42/actions/pathmatch"
)

func TestFilter(t *testing.T) {
	files := []github.ChangedFile{
		{Path: "src/a.go", Status: "modified"},
		{Path: "src/gone.go", Status: "removed"},
		{Path: "lib/moved.go", PreviousPath: "src/moved.go", Status: "renamed"},
		{Path: "docs/x.md", Status: "added"},
	}

	tests := []struct {
		name     string
		patterns []string
		paths    []string
		matched  bool
	}{
		{"no filter", nil, []string{"src/a.go", "lib/moved.go", "docs/x.md"}, true},
		{"directory", []string{"src"}, []string{"src/a.go"}, true},
		{"renamed into", []string{"lib"}, []string{"lib/moved.go"}, true},
		{"only deleted", []string{"src/gone.go"}, []string{}, true},
		{"only renamed away", []string{"src/moved.go"}, []string{}, true},
		{"nothing", []string{"test/**"}, []string{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := pathmatch.Compile(tt.patterns)
			if err != nil {
				t.Fatal(err)
			}
			paths, matched := filter(m, files)
			if !slices.Equal(paths, tt.paths) || matched != tt.matched {
				t.Errorf("got %q, %v, want %q, %v", paths, matched, tt.paths, tt.matched)
			}
		})
	}
}

func TestFilterNoFiles(t *testing.T) {
	m, _ := pathmatch.Compile(nil)
	paths, matched := filter(m, nil)
	if paths == nil || len(paths) != 0 || matched {
		t.Errorf("got %#v, %v", paths, matched)
	}
}
