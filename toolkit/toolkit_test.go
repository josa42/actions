package toolkit

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

func captureStdout(t *testing.T) *strings.Builder {
	t.Helper()
	var b strings.Builder
	old := stdout
	stdout = &b
	t.Cleanup(func() { stdout = old })
	return &b
}

func TestInput(t *testing.T) {
	t.Setenv("INPUT_WORKING-DIRECTORY", "  dir \n")
	t.Setenv("INPUT_MY_NAME", "x")

	if got := Input("working-directory"); got != "dir" {
		t.Errorf("Input(working-directory) = %q", got)
	}
	if got := Input("my name"); got != "x" {
		t.Errorf("Input(my name) = %q", got)
	}
	if got := Input("missing"); got != "" {
		t.Errorf("Input(missing) = %q", got)
	}
}

func TestInputBool(t *testing.T) {
	for v, want := range map[string]bool{"true": true, "TRUE": true, "False": false, "": false} {
		t.Setenv("INPUT_FLAG", v)
		got, err := InputBool("flag")
		if err != nil || got != want {
			t.Errorf("InputBool(%q) = %v, %v", v, got, err)
		}
	}

	t.Setenv("INPUT_FLAG", "yes")
	if _, err := InputBool("flag"); err == nil {
		t.Error("InputBool(yes) did not fail")
	}
}

func TestInputList(t *testing.T) {
	t.Setenv("INPUT_FILES", "a.yaml\n\n  b.yaml \n")
	if got := InputList("files"); !slices.Equal(got, []string{"a.yaml", "b.yaml"}) {
		t.Errorf("InputList = %q", got)
	}
}

func TestSetOutput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "output")
	t.Setenv("GITHUB_OUTPUT", path)

	if err := SetOutput("a", "1"); err != nil {
		t.Fatal(err)
	}
	if err := SetOutput("b", "x\ny"); err != nil {
		t.Fatal(err)
	}

	got, _ := os.ReadFile(path)
	re := regexp.MustCompile(`ghadelimiter_[0-9a-f]{32}`)
	want := "a<<D\n1\nD\nb<<D\nx\ny\nD\n"
	if s := re.ReplaceAllString(string(got), "D"); s != want {
		t.Errorf("GITHUB_OUTPUT = %q, want %q", s, want)
	}
}

func TestSetOutputUnset(t *testing.T) {
	t.Setenv("GITHUB_OUTPUT", "")
	if err := SetOutput("a", "1"); err == nil {
		t.Error("SetOutput without GITHUB_OUTPUT did not fail")
	}
}

func TestAnnotations(t *testing.T) {
	out := captureStdout(t)

	Notice("plain")
	Error("50%\nbroken", Annotation{Title: "a: b, c", File: "x.yaml", Line: 3, Col: 7})
	SetSecret("s3cret")

	want := "::notice::plain\n" +
		"::error title=a%3A b%2C c,file=x.yaml,line=3,col=7::50%25%0Abroken\n" +
		"::add-mask::s3cret\n"
	if out.String() != want {
		t.Errorf("stdout = %q, want %q", out.String(), want)
	}
}

func TestGroup(t *testing.T) {
	out := captureStdout(t)

	Group("build", func() error {
		out.WriteString("inside\n")
		return nil
	})

	if want := "::group::build\ninside\n::endgroup::\n"; out.String() != want {
		t.Errorf("stdout = %q, want %q", out.String(), want)
	}
}
