package conventional

import (
	"slices"
	"strings"
	"testing"
)

var strict = Config{MaxLength: 72, Lowercase: true, NoPeriod: true}

func TestCheckValid(t *testing.T) {
	for _, msg := range []string{
		// Examples of the specification.
		"feat: allow provided config object to extend other configs\n\nBREAKING CHANGE: `extends` key in config file is now used for extending other config files",
		"feat!: send an email to the customer when a product is shipped",
		"feat(api)!: send an email to the customer when a product is shipped",
		"chore!: drop support for Node 6\n\nBREAKING CHANGE: use JavaScript features not available in Node 6.",
		"docs: correct spelling of CHANGELOG",
		"feat(lang): add Polish language",
		"fix: prevent racing of requests\n\nIntroduce a request id and a reference to latest request. Dismiss\nincoming responses other than from latest request.\n\nReviewed-by: Z\nRefs: #123",
		// From the history of these repositories.
		"feat(esphome-lint): add esphome config lint action",
		"fix(go): use latest golangci-lint to support newer go versions",
		"feat!: bump, tag and publish in ha-release\n\nBody.\n\nBREAKING-CHANGE: callers start releases with workflow_dispatch.",
		"chore: bump version to 1.14.0",
		"ci: release from the release workflow",
		// Every default type.
		"build: x", "perf: x", "refactor: x", "style: x", "test: x",
		// Descriptions starting with something other than a letter.
		"fix: 404 on missing pages",
		"docs: `README` typo",
		// git revert, git revert of a revert.
		"Revert \"feat: add x\"\n\nThis reverts commit 0123abc.",
		"Reapply \"feat: add x\"",
		// Windows line endings and surrounding whitespace.
		"fix: x\r\n\r\nbody\r\n",
		"  fix: x  \n",
		// Exactly the maximum length.
		"fix: " + strings.Repeat("x", 67),
		// Unicode counts as one character per rune.
		"fix: " + strings.Repeat("ä", 67),
	} {
		if v := Check(msg, strict); v != nil {
			t.Errorf("Check(%q) = %q", msg, v)
		}
	}
}

func TestCheckInvalid(t *testing.T) {
	tests := []struct {
		msg  string
		want []string
	}{
		{"Add release script", []string{"the header must be type(scope): description, with type one of feat, fix, docs, refactor, test, chore, build, ci, perf, style"}},
		{"Merge pull request #1 from josa42/x", []string{"the header must be type(scope): description, with type one of feat, fix, docs, refactor, test, chore, build, ci, perf, style"}},
		{"feature: add x", []string{`type "feature" is not one of feat, fix, docs, refactor, test, chore, build, ci, perf, style`}},
		{"Feat: add x", []string{`type "Feat" is not one of feat, fix, docs, refactor, test, chore, build, ci, perf, style`}},
		{"feat(): add x", []string{"the scope is empty"}},
		{"feat:add x", []string{"a single space must follow the colon"}},
		{"feat:  add x", []string{"a single space must follow the colon"}},
		{"feat:  \t", []string{"a single space must follow the colon", "the description is empty"}},
		{"feat:", []string{"a single space must follow the colon", "the description is empty"}},
		{"feat: Add x", []string{"the description must start with a lowercase letter"}},
		{"feat: add x.", []string{"the description must not end with a period"}},
		{"fix: " + strings.Repeat("x", 68), []string{"the header is 73 characters, at most 72 are allowed"}},
		{"Feat(): Add x.", []string{`type "Feat" is not one of feat, fix, docs, refactor, test, chore, build, ci, perf, style`, "the scope is empty", "the description must start with a lowercase letter", "the description must not end with a period"}},
		{"fix: x\nbody", []string{"a blank line must separate the header from the body"}},
		{"fix: x\n\nbreaking change: y", []string{"BREAKING CHANGE must be uppercase"}},
		{"fix: x\n\nBreaking-Change: y", []string{"BREAKING CHANGE must be uppercase"}},
		{"fixup! feat: add x", []string{"fixup!, squash! and amend! commits must be squashed before merging"}},
		{"squash! feat: add x", []string{"fixup!, squash! and amend! commits must be squashed before merging"}},
		{"amend! feat: add x", []string{"fixup!, squash! and amend! commits must be squashed before merging"}},
		{"Revert \"Add release script\"", []string{"the header must be type(scope): description, with type one of feat, fix, docs, refactor, test, chore, build, ci, perf, style"}},
		{"Revert \"feat: Add x\"", []string{"the description must start with a lowercase letter"}},
		{"Revert feat: add x", []string{"the header must be type(scope): description, with type one of feat, fix, docs, refactor, test, chore, build, ci, perf, style"}},
	}
	for _, tt := range tests {
		if got := Check(tt.msg, strict); !slices.Equal(got, tt.want) {
			t.Errorf("Check(%q)\n got %q\nwant %q", tt.msg, got, tt.want)
		}
	}
}

func TestCheckConfig(t *testing.T) {
	tests := []struct {
		name  string
		msg   string
		cfg   Config
		valid bool
	}{
		{"custom type", "deps: bump x", Config{Types: []string{"deps"}}, true},
		{"custom types replace defaults", "feat: x", Config{Types: []string{"deps"}}, false},
		{"scope in list", "feat(github): x", Config{Scopes: []string{"github", "git"}}, true},
		{"scope not in list", "feat(gihub): x", Config{Scopes: []string{"github", "git"}}, false},
		{"scope list keeps scope optional", "feat: x", Config{Scopes: []string{"github"}}, true},
		{"scope list with breaking change", "feat(git)!: x", Config{Scopes: []string{"git"}}, true},
		{"uppercase allowed", "feat: Add x", Config{}, true},
		{"period allowed", "feat: add x.", Config{}, true},
		{"no length limit", "feat: " + strings.Repeat("x", 200), Config{}, true},
		{"custom length limit", "feat: " + strings.Repeat("x", 50), Config{MaxLength: 50}, false},
		{"empty scope still invalid", "feat(): x", Config{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := Check(tt.msg, tt.cfg)
			if (v == nil) != tt.valid {
				t.Errorf("Check(%q) = %q", tt.msg, v)
			}
		})
	}
}

func TestCheckHeaderIgnoresBodyRules(t *testing.T) {
	// A pull request title has no body, so only the header rules apply.
	if v := CheckHeader("feat: add x", strict); v != nil {
		t.Errorf("CheckHeader = %q", v)
	}
	if v := CheckHeader("Add x", strict); v == nil {
		t.Error("CheckHeader(Add x) is valid")
	}
}
