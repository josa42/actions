package gofmtlint

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestDiff(t *testing.T) {
	tests := []struct {
		name string
		a, b string
		want string
	}{
		{"equal", "a\nb", "a\nb", "[]"},
		{"change", "a\nx\nc", "a\nb\nc", "[{2 3 [b]}]"},
		{"delete", "a\n\n\nb", "a\n\nb", "[{3 4 []}]"},
		{"insert", "a\nc", "a\nb\nc", "[{2 2 [b]}]"},
		{"two hunks", "a\nx\nb\nc\ny", "a\n1\nb\nc\n2", "[{2 3 [1]} {5 6 [2]}]"},
		{"at start", "x\na", "a", "[{1 2 []}]"},
		{"at end", "a", "a\nb", "[{2 2 [b]}]"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fmt.Sprint(diff(splitLines(tt.a), splitLines(tt.b)))
			if got != tt.want {
				t.Errorf("got %s, want %s", got, tt.want)
			}
		})
	}
}

func TestGoFiles(t *testing.T) {
	dir := t.TempDir()
	for _, f := range []string{"a.go", "a.txt", ".b.go", "sub/c.go", "vendor/d.go", "testdata/e.go", ".git/f.go", "_x/g.go"} {
		write(t, filepath.Join(dir, f), "package x\n")
	}
	files, err := goFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	for i, f := range files {
		files[i], _ = filepath.Rel(dir, f)
	}
	if want := []string{"a.go", filepath.Join("sub", "c.go")}; !slices.Equal(files, want) {
		t.Errorf("got %q, want %q", files, want)
	}
}

func TestRun(t *testing.T) {
	dir := t.TempDir()
	unformatted := "package x\n\nfunc f() {\n  return\n}\n"
	write(t, filepath.Join(dir, "ok.go"), "package x\n")
	write(t, filepath.Join(dir, "bad.go"), unformatted)
	write(t, filepath.Join(dir, "broken.go"), "package x\n\nfunc {\n")

	env := t.TempDir()
	for _, f := range []string{"GITHUB_OUTPUT", "GITHUB_STEP_SUMMARY"} {
		t.Setenv(f, filepath.Join(env, f))
	}
	t.Setenv("GITHUB_WORKSPACE", dir)
	t.Setenv("INPUT_WORKING-DIRECTORY", dir)

	err := Run(context.Background())
	if err == nil || !strings.Contains(err.Error(), "2 of 3 files") {
		t.Errorf("got error %v", err)
	}

	got, _ := os.ReadFile(filepath.Join(dir, "bad.go"))
	if string(got) != unformatted {
		t.Errorf("bad.go was changed:\n%s", got)
	}
	out, _ := os.ReadFile(filepath.Join(env, "GITHUB_OUTPUT"))
	if !strings.Contains(string(out), "\nbad.go\nbroken.go\n") {
		t.Errorf("output:\n%s", out)
	}
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
