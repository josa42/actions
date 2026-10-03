package pathmatch

import (
	"slices"
	"testing"
)

func TestMatch(t *testing.T) {
	tests := []struct {
		name     string
		patterns []string
		match    []string
		noMatch  []string
	}{
		{
			name:     "literal file",
			patterns: []string{"a/b/file.txt"},
			match:    []string{"a/b/file.txt"},
			noMatch:  []string{"a/b/file.txt.bak", "a/b/file", "x/a/b/file.txt"},
		},
		{
			name:     "literal directory matches below",
			patterns: []string{"a/b"},
			match:    []string{"a/b", "a/b/file.txt", "a/b/c/d/file.txt"},
			noMatch:  []string{"a/bc/file.txt", "a/file.txt", "x/a/b/file.txt"},
		},
		{
			name:     "top level literal directory",
			patterns: []string{"a"},
			match:    []string{"a/b/file.txt"},
			noMatch:  []string{"ab/file.txt", "b/a/file.txt"},
		},
		{
			name:     "trailing slash",
			patterns: []string{"a/b/"},
			match:    []string{"a/b/file.txt", "a/b/c/file.txt"},
			noMatch:  []string{"a/bc/file.txt"},
		},
		{
			name:     "trailing slash with wildcard",
			patterns: []string{"packages/*/"},
			match:    []string{"packages/x/file.txt", "packages/x/src/file.txt"},
			noMatch:  []string{"other/x/file.txt"},
		},
		{
			name:     "leading ./ and / in pattern",
			patterns: []string{"./a/b/file.txt", "/c/file.txt"},
			match:    []string{"a/b/file.txt", "c/file.txt"},
		},
		{
			name:     "leading ./ and / in path",
			patterns: []string{"a/*.txt"},
			match:    []string{"./a/file.txt", "/a/file.txt"},
		},
		{
			name:     "star stays in one directory",
			patterns: []string{"*.txt"},
			match:    []string{"file.txt", ".txt"},
			noMatch:  []string{"a/file.txt", "file.md"},
		},
		{
			name:     "star in directory",
			patterns: []string{"a/*.txt"},
			match:    []string{"a/file.txt"},
			noMatch:  []string{"a/b/file.txt", "file.txt"},
		},
		{
			name:     "star pattern does not match below",
			patterns: []string{"a/*"},
			match:    []string{"a/x", "a/b"},
			noMatch:  []string{"a/b/file.txt", "a"},
		},
		{
			name:     "star in the middle",
			patterns: []string{"packages/*/package.json"},
			match:    []string{"packages/a/package.json"},
			noMatch:  []string{"packages/package.json", "packages/a/b/package.json"},
		},
		{
			name:     "double star alone",
			patterns: []string{"**"},
			match:    []string{"file.txt", "a/b/file.txt", ".github/workflows/ci.yml"},
		},
		{
			name:     "double star slash matches zero directories",
			patterns: []string{"**/*.txt"},
			match:    []string{"file.txt", "a/file.txt", "a/b/c/file.txt"},
			noMatch:  []string{"file.md", "a/file.txt/x"},
		},
		{
			name:     "trailing double star",
			patterns: []string{"a/**"},
			match:    []string{"a/file.txt", "a/b/c/file.txt"},
			noMatch:  []string{"a", "ab/file.txt", "b/a/file.txt"},
		},
		{
			name:     "double star in the middle",
			patterns: []string{"a/**/*.txt"},
			match:    []string{"a/file.txt", "a/b/file.txt", "a/b/c/file.txt"},
			noMatch:  []string{"b/a/file.txt", "a/b/file.md"},
		},
		{
			name:     "double star around a directory",
			patterns: []string{"**/test/**"},
			match:    []string{"test/a.go", "a/test/b.go", "a/b/test/c/d.go"},
			noMatch:  []string{"a/testing/b.go", "a/test"},
		},
		{
			name:     "double star inside a segment",
			patterns: []string{"**.js"},
			match:    []string{"x.js", "a/b/x.js"},
			noMatch:  []string{"x.ts", "a/x.jsx"},
		},
		{
			name:     "question mark",
			patterns: []string{"file?.txt"},
			match:    []string{"file1.txt", "filea.txt"},
			noMatch:  []string{"file.txt", "file10.txt"},
		},
		{
			name:     "question mark does not match slash",
			patterns: []string{"a?b"},
			match:    []string{"axb"},
			noMatch:  []string{"a/b"},
		},
		{
			name:     "character range",
			patterns: []string{"file[0-9].txt"},
			match:    []string{"file0.txt", "file9.txt"},
			noMatch:  []string{"filea.txt", "file10.txt"},
		},
		{
			name:     "character set",
			patterns: []string{"[ab].txt"},
			match:    []string{"a.txt", "b.txt"},
			noMatch:  []string{"c.txt"},
		},
		{
			name:     "negated character class with !",
			patterns: []string{"file[!0-9].txt"},
			match:    []string{"filea.txt"},
			noMatch:  []string{"file5.txt"},
		},
		{
			name:     "negated character class with ^",
			patterns: []string{"file[^0-9].txt"},
			match:    []string{"filea.txt"},
			noMatch:  []string{"file5.txt"},
		},
		{
			name:     "negated character class does not match slash",
			patterns: []string{"a[!x]b"},
			match:    []string{"ayb"},
			noMatch:  []string{"a/b", "axb"},
		},
		{
			name:     "closing bracket first in class is literal",
			patterns: []string{"x[]]"},
			match:    []string{"x]"},
			noMatch:  []string{"x"},
		},
		{
			name:     "escaped wildcards are literal",
			patterns: []string{`\*.txt`, `file\[1\].md`, `q\?`},
			match:    []string{"*.txt", "file[1].md", "q?"},
			noMatch:  []string{"a.txt", "file1.md", "qa"},
		},
		{
			name:     "regexp characters are literal",
			patterns: []string{"a+b/(x).txt", "c.d", "e$|f"},
			match:    []string{"a+b/(x).txt", "c.d", "e$|f"},
			noMatch:  []string{"aab/x.txt", "cxd", "e", "f"},
		},
		{
			name:     "dot files",
			patterns: []string{"*", "**/*.yml"},
			match:    []string{".env", ".github/workflows/ci.yml"},
		},
		{
			name:     "unicode",
			patterns: []string{"dócs/?.md", "[äö].txt"},
			match:    []string{"dócs/ä.md", "ä.txt", "ö.txt"},
			noMatch:  []string{"docs/a.md", "dócs/ab.md", "a.txt"},
		},
		{
			name:     "spaces in paths",
			patterns: []string{"my docs/*.md"},
			match:    []string{"my docs/read me.md"},
		},
		{
			name:     "negation excludes",
			patterns: []string{"src/**", "!src/**/*.test.ts"},
			match:    []string{"src/a.ts", "src/b/c.ts"},
			noMatch:  []string{"src/a.test.ts", "src/b/c.test.ts", "lib/a.ts"},
		},
		{
			name:     "later pattern overrides earlier",
			patterns: []string{"src/**", "!src/gen/**", "src/gen/keep.ts"},
			match:    []string{"src/a.ts", "src/gen/keep.ts"},
			noMatch:  []string{"src/gen/other.ts"},
		},
		{
			name:     "only negated patterns exclude from everything",
			patterns: []string{"!docs/**", "!*.md"},
			match:    []string{"src/a.ts", "docs"},
			noMatch:  []string{"docs/a.txt", "README.md"},
		},
		{
			name:     "negated literal directory",
			patterns: []string{"**", "!vendor"},
			match:    []string{"src/a.go"},
			noMatch:  []string{"vendor/x/y.go"},
		},
		{
			name:     "negation re-included",
			patterns: []string{"!docs/**", "docs/keep.md"},
			match:    []string{"docs/keep.md", "src/a.ts"},
			noMatch:  []string{"docs/other.md"},
		},
		{
			name:     "any of several patterns",
			patterns: []string{"x", "a/**"},
			match:    []string{"a/b/file.txt", "x/y"},
			noMatch:  []string{"b/file.txt"},
		},
		{
			name:     "whitespace around patterns",
			patterns: []string{"  a/*.txt  ", "\tb\t"},
			match:    []string{"a/file.txt", "b/file.txt"},
		},
		{
			name:     "no patterns match everything",
			patterns: nil,
			match:    []string{"a", "a/b/file.txt", ".env"},
		},
		{
			name:     "blank patterns match everything",
			patterns: []string{"", "  "},
			match:    []string{"a/b/file.txt"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := Compile(tt.patterns)
			if err != nil {
				t.Fatal(err)
			}
			for _, p := range tt.match {
				if !m.Match(p) {
					t.Errorf("%q should match %q", tt.patterns, p)
				}
			}
			for _, p := range tt.noMatch {
				if m.Match(p) {
					t.Errorf("%q should not match %q", tt.patterns, p)
				}
			}
		})
	}
}

func TestCompileErrors(t *testing.T) {
	for _, p := range []string{"[abc", "a/[b/c]", `a\`, "!", "/", "./", "[!"} {
		if _, err := Compile([]string{p}); err == nil {
			t.Errorf("Compile(%q) did not fail", p)
		}
	}
}

func TestEmpty(t *testing.T) {
	tests := []struct {
		patterns []string
		want     bool
	}{
		{nil, true},
		{[]string{"", " "}, true},
		{[]string{"a/**"}, false},
		{[]string{"!a/**"}, false},
	}
	for _, tt := range tests {
		m, err := Compile(tt.patterns)
		if err != nil {
			t.Fatal(err)
		}
		if m.Empty() != tt.want {
			t.Errorf("Compile(%q).Empty() = %v", tt.patterns, !tt.want)
		}
	}
}

func TestFilter(t *testing.T) {
	m, err := Compile([]string{"**/*.txt"})
	if err != nil {
		t.Fatal(err)
	}

	got := m.Filter([]string{"a/b/file.txt", "b/c.md", "d.txt"})
	if want := []string{"a/b/file.txt", "d.txt"}; !slices.Equal(got, want) {
		t.Errorf("Filter = %q, want %q", got, want)
	}
}
