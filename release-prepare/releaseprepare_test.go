package releaseprepare

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/josa42/actions/versionfile"
)

func write(t *testing.T, path, content string) {
	t.Helper()
	os.MkdirAll(filepath.Dir(path), 0o755)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func run(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %s", args, out)
	}
	return strings.TrimSpace(string(out))
}

const manifest = "{\n  \"domain\": \"x\",\n  \"version\": \"1.2.0\"\n}\n"

func TestPlan(t *testing.T) {
	t.Chdir(t.TempDir())
	write(t, "CHANGELOG.md", "# Changelog\n\n## Unreleased\n\n- New\n\n## 1.2.0 - 2026-01-01\n")
	write(t, "x/manifest.json", manifest)
	write(t, "x/__init__.py", "A = \"1.2.0\"\nB = \"1.2.0\"\n")

	in := inputs{
		changelog: "CHANGELOG.md",
		files: []versionfile.File{
			{Path: "x/manifest.json", Template: `"version": "{version}"`},
			{Path: "x/__init__.py", Template: `A = "{version}"`},
			{Path: "x/__init__.py", Template: `B = "{version}"`},
		},
	}
	p, err := plan(in, "1.3.0", "2026-10-03")
	if err != nil {
		t.Fatal(err)
	}

	want := map[string]string{
		"CHANGELOG.md":    "# Changelog\n\n## 1.3.0 - 2026-10-03\n\n- New\n\n## 1.2.0 - 2026-01-01\n",
		"x/manifest.json": strings.Replace(manifest, "1.2.0", "1.3.0", 1),
		"x/__init__.py":   "A = \"1.3.0\"\nB = \"1.3.0\"\n",
	}
	if len(p.edits) != len(want) {
		t.Fatalf("edits = %v", p.edits)
	}
	for _, e := range p.edits {
		if e.content != want[e.path] {
			t.Errorf("%s = %q, want %q", e.path, e.content, want[e.path])
		}
	}
	if p.notes != "- New" {
		t.Errorf("notes = %q", p.notes)
	}
	// Nothing is written by plan.
	if read(t, "x/manifest.json") != manifest {
		t.Error("plan wrote a file")
	}
}

func TestPlanWithoutChangelog(t *testing.T) {
	t.Chdir(t.TempDir())
	write(t, "manifest.json", manifest)

	p, err := plan(inputs{
		changelog: "CHANGELOG.md",
		files:     []versionfile.File{{Path: "manifest.json", Template: `"version": "{version}"`}},
	}, "1.3.0", "2026-10-03")
	if err != nil {
		t.Fatal(err)
	}
	if len(p.edits) != 1 || p.notes != "" {
		t.Errorf("plan = %+v", p)
	}
}

func TestPlanErrors(t *testing.T) {
	tests := []struct {
		name  string
		files map[string]string
		in    inputs
	}{
		{
			name:  "changelog without unreleased",
			files: map[string]string{"CHANGELOG.md": "## 1.2.0\n- x\n"},
			in:    inputs{changelog: "CHANGELOG.md"},
		},
		{
			name:  "missing version file",
			files: map[string]string{},
			in:    inputs{files: []versionfile.File{{Path: "manifest.json", Template: `"version": "{version}"`}}},
		},
		{
			name:  "template does not match",
			files: map[string]string{"manifest.json": manifest},
			in:    inputs{files: []versionfile.File{{Path: "manifest.json", Template: `"vers": "{version}"`}}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			for p, c := range tt.files {
				write(t, p, c)
			}
			if _, err := plan(tt.in, "1.3.0", "2026-10-03"); err == nil {
				t.Error("did not fail")
			}
		})
	}
}

func TestReadInputs(t *testing.T) {
	set := func(env map[string]string) {
		for _, k := range []string{"VERSION", "VERSION-FILES", "CHANGELOG", "TAG-FORMAT", "COMMIT-MESSAGE", "ALLOW-BRANCH"} {
			t.Setenv("INPUT_"+k, env[k])
		}
	}
	defaults := func() map[string]string {
		return map[string]string{"VERSION": "patch", "TAG-FORMAT": "v{version}", "COMMIT-MESSAGE": "chore: bump version to {version}", "CHANGELOG": "CHANGELOG.md"}
	}

	set(defaults())
	if _, err := readInputs(); err != nil {
		t.Errorf("defaults: %v", err)
	}

	for name, change := range map[string][2]string{
		"no version":        {"VERSION", ""},
		"tag format":        {"TAG-FORMAT", "v"},
		"no commit message": {"COMMIT-MESSAGE", ""},
		"bad version file":  {"VERSION-FILES", "manifest.json"},
		"bad allow-branch":  {"ALLOW-BRANCH", "yes"},
	} {
		env := defaults()
		env[change[0]] = change[1]
		set(env)
		if _, err := readInputs(); err == nil {
			t.Errorf("%s: did not fail", name)
		}
	}
}

// setupRun prepares a checkout with a bare origin, a fake GitHub API and the
// runner environment for Run.
func setupRun(t *testing.T, tags []string, branch string) (remote string, outputs string) {
	t.Helper()
	dir := t.TempDir()
	remote = filepath.Join(dir, "remote.git")
	work := filepath.Join(dir, "work")
	run(t, dir, "init", "-q", "--bare", "-b", "main", remote)
	run(t, dir, "init", "-q", "-b", branch, work)
	run(t, work, "remote", "add", "origin", remote)
	write(t, filepath.Join(work, "CHANGELOG.md"), "# Changelog\n\n## Unreleased\n\n- New\n")
	write(t, filepath.Join(work, "x/manifest.json"), manifest)
	run(t, work, "add", ".")
	run(t, work, "commit", "-q", "-m", "init")
	run(t, work, "push", "-q", "origin", branch)
	t.Chdir(work)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/repos/o/r/tags":
			var list []map[string]string
			for _, tag := range tags {
				list = append(list, map[string]string{"name": tag})
			}
			json.NewEncoder(w).Encode(list)
		case strings.HasPrefix(r.URL.Path, "/repos/o/r/git/ref/tags/"):
			name := strings.TrimPrefix(r.URL.Path, "/repos/o/r/git/ref/tags/")
			for _, tag := range tags {
				if tag == name {
					io.WriteString(w, `{}`)
					return
				}
			}
			w.WriteHeader(http.StatusNotFound)
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)

	event := filepath.Join(dir, "event.json")
	write(t, event, `{"repository":{"default_branch":"main"}}`)
	outputs = filepath.Join(dir, "outputs")
	write(t, outputs, "")

	for k, v := range map[string]string{
		"GITHUB_API_URL":       srv.URL,
		"GITHUB_REPOSITORY":    "o/r",
		"GITHUB_EVENT_NAME":    "workflow_dispatch",
		"GITHUB_EVENT_PATH":    event,
		"GITHUB_REF_TYPE":      "branch",
		"GITHUB_REF_NAME":      branch,
		"GITHUB_OUTPUT":        outputs,
		"INPUT_GITHUB-TOKEN":   "token",
		"INPUT_VERSION":        "minor",
		"INPUT_VERSION-FILES":  `x/manifest.json: "version": "{version}"`,
		"INPUT_CHANGELOG":      "CHANGELOG.md",
		"INPUT_TAG-FORMAT":     "v{version}",
		"INPUT_COMMIT-MESSAGE": "chore: bump version to {version}",
		"INPUT_ALLOW-BRANCH":   "false",
	} {
		t.Setenv(k, v)
	}
	return remote, outputs
}

func outputValue(t *testing.T, path, name string) string {
	t.Helper()
	m := regexp.MustCompile(`(?s)` + name + `<<(\S+)\n(.*?)\n\S+\n`).FindStringSubmatch(read(t, path))
	if m == nil {
		t.Fatalf("output %s not set", name)
	}
	return m[2]
}

func TestRun(t *testing.T) {
	remote, outputs := setupRun(t, []string{"v1.1.0", "v1.2.0", "other"}, "main")

	if err := Run(context.Background()); err != nil {
		t.Fatal(err)
	}

	head := run(t, remote, "rev-parse", "main")
	if got := run(t, remote, "rev-parse", "v1.3.0^{commit}"); got != head {
		t.Errorf("tag at %s, main at %s", got, head)
	}
	if got := run(t, remote, "log", "-1", "--format=%an|%s", "main"); got != "github-actions[bot]|chore: bump version to 1.3.0" {
		t.Errorf("commit = %s", got)
	}
	if got := run(t, remote, "show", "main:x/manifest.json"); !strings.Contains(got, `"version": "1.3.0"`) {
		t.Errorf("manifest = %s", got)
	}
	if got := run(t, remote, "show", "main:CHANGELOG.md"); !strings.Contains(got, "## 1.3.0 - ") {
		t.Errorf("changelog = %s", got)
	}

	for name, want := range map[string]string{"version": "1.3.0", "tag": "v1.3.0", "sha": head, "notes": "- New"} {
		if got := outputValue(t, outputs, name); got != want {
			t.Errorf("output %s = %q, want %q", name, got, want)
		}
	}
}

func TestRunNothingToCommit(t *testing.T) {
	remote, _ := setupRun(t, nil, "main")
	t.Setenv("INPUT_VERSION", "1.2.0")
	t.Setenv("INPUT_CHANGELOG", "")

	before := run(t, remote, "rev-parse", "main")
	if err := Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := run(t, remote, "rev-parse", "v1.2.0^{commit}"); got != before {
		t.Errorf("tag at %s, want the unchanged main %s", got, before)
	}
}

func TestRunFailsBeforeChanges(t *testing.T) {
	tests := []struct {
		name   string
		tags   []string
		branch string
		env    map[string]string
	}{
		{"tag exists", []string{"v1.2.0"}, "main", map[string]string{"INPUT_VERSION": "1.2.0"}},
		{"version not higher", []string{"v1.2.0"}, "main", map[string]string{"INPUT_VERSION": "1.1.0"}},
		{"other branch", nil, "feature", nil},
		{"tag ref", nil, "main", map[string]string{"GITHUB_REF_TYPE": "tag"}},
		{"bad changelog", nil, "main", map[string]string{"INPUT_CHANGELOG": "x/manifest.json"}},
		{"bad version file", nil, "main", map[string]string{"INPUT_VERSION-FILES": `x/manifest.json: "vers": "{version}"`}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			remote, _ := setupRun(t, tt.tags, tt.branch)
			for k, v := range tt.env {
				t.Setenv(k, v)
			}
			before := run(t, remote, "rev-parse", tt.branch)

			if err := Run(context.Background()); err == nil {
				t.Fatal("did not fail")
			}
			if got := run(t, ".", "status", "--porcelain"); got != "" {
				t.Errorf("working tree changed:\n%s", got)
			}
			if got := run(t, remote, "rev-parse", tt.branch); got != before {
				t.Error("pushed")
			}
			if got := run(t, remote, "tag"); got != "" {
				t.Errorf("remote tags %q", got)
			}
		})
	}
}

func TestRunAllowBranch(t *testing.T) {
	remote, _ := setupRun(t, nil, "feature")
	t.Setenv("INPUT_ALLOW-BRANCH", "true")

	if err := Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := run(t, remote, "log", "-1", "--format=%s", "feature"); got != "chore: bump version to 0.1.0" {
		t.Errorf("feature = %s", got)
	}
}
