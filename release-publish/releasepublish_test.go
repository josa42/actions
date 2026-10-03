package releasepublish

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func setup(t *testing.T, files ...string) string {
	t.Helper()
	root := t.TempDir()
	for _, f := range files {
		p := filepath.Join(root, filepath.FromSlash(f))
		os.MkdirAll(filepath.Dir(p), 0o755)
		if err := os.WriteFile(p, []byte(f), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestCollectAssets(t *testing.T) {
	root := setup(t, "x.zip", "dist/card.js", "dist/card.js.map", "src/a.go", ".git/config", "dist/.hidden")

	tests := []struct {
		name     string
		patterns []string
		want     []string
	}{
		{"none", nil, nil},
		{"file", []string{"x.zip"}, []string{"x.zip"}},
		{"glob", []string{"dist/*.js"}, []string{"card.js"}},
		{"several", []string{"x.zip", "dist/card.js*"}, []string{"x.zip", "card.js", "card.js.map"}},
		{"overlapping", []string{"dist/*", "dist/card.js"}, []string{".hidden", "card.js", "card.js.map"}},
		{"directory", []string{"dist"}, []string{".hidden", "card.js", "card.js.map"}},
		{"blank lines", []string{"", "x.zip"}, []string{"x.zip"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assets, err := collectAssets(root, tt.patterns)
			if err != nil {
				t.Fatal(err)
			}
			var names []string
			for _, a := range assets {
				names = append(names, a.Name)
				if a.Name == "card.js" && string(a.Data) != "dist/card.js" {
					t.Errorf("card.js data = %q", a.Data)
				}
			}
			if !slices.Equal(names, tt.want) {
				t.Errorf("got %q, want %q", names, tt.want)
			}
		})
	}
}

func TestCollectAssetsErrors(t *testing.T) {
	root := setup(t, "a/x.zip", "b/x.zip", ".git/config")

	for name, patterns := range map[string][]string{
		"no match":       {"missing.zip"},
		"git is skipped": {".git/config"},
		"duplicate name": {"**/x.zip"},
		"bad pattern":    {"[x"},
	} {
		if _, err := collectAssets(root, patterns); err == nil {
			t.Errorf("%s: did not fail", name)
		}
	}
}
