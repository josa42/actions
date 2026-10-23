package versionfile

import (
	"slices"
	"strings"
	"testing"
)

func TestParseList(t *testing.T) {
	got, err := ParseList([]string{
		`custom_components/x/manifest.json: "version": "{version}"`,
		"",
		`  x/__init__.py:   STRATEGY_VERSION = "{version}"  `,
		`card.js: const CARD_VERSION = "{version}";`,
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []File{
		{"custom_components/x/manifest.json", `"version": "{version}"`},
		{"x/__init__.py", `STRATEGY_VERSION = "{version}"`},
		{"card.js", `const CARD_VERSION = "{version}";`},
	}
	if !slices.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}

	for _, line := range []string{
		"manifest.json",
		`: "version": "{version}"`,
		`manifest.json: "version": "1.0.0"`,
		`manifest.json: {version} {version}`,
	} {
		if _, err := ParseList([]string{line}); err == nil {
			t.Errorf("ParseList(%q) did not fail", line)
		}
	}
}

func TestReplace(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		template string
		want     string
		old      string
	}{
		{
			name:     "json",
			content:  "{\n  \"domain\": \"x\",\n  \"version\": \"1.2.3\",\n  \"iot_class\": \"local_polling\"\n}\n",
			template: `"version": "{version}"`,
			want:     "{\n  \"domain\": \"x\",\n  \"version\": \"1.3.0\",\n  \"iot_class\": \"local_polling\"\n}\n",
			old:      "1.2.3",
		},
		{
			name:     "python",
			content:  "\"\"\"Doc.\"\"\"\n\nSTRATEGY_VERSION = \"1.2.3\"\nOTHER = 1\n",
			template: `STRATEGY_VERSION = "{version}"`,
			want:     "\"\"\"Doc.\"\"\"\n\nSTRATEGY_VERSION = \"1.3.0\"\nOTHER = 1\n",
			old:      "1.2.3",
		},
		{
			name:     "javascript with trailing text",
			content:  "const CARD_VERSION = \"1.2.3\"; // shown in the console\n",
			template: `const CARD_VERSION = "{version}";`,
			want:     "const CARD_VERSION = \"1.3.0\"; // shown in the console\n",
			old:      "1.2.3",
		},
		{
			name:     "empty version",
			content:  "version = \"\"\n",
			template: `version = "{version}"`,
			want:     "version = \"1.3.0\"\n",
			old:      "",
		},
		{
			name:     "pre-release version",
			content:  "version = \"1.3.0-beta.1+abc\"\n",
			template: `version = "{version}"`,
			want:     "version = \"1.3.0\"\n",
			old:      "1.3.0-beta.1+abc",
		},
		{
			name:     "unchanged",
			content:  "version = \"1.3.0\"\n",
			template: `version = "{version}"`,
			want:     "version = \"1.3.0\"\n",
			old:      "1.3.0",
		},
		{
			name:     "regexp characters in template",
			content:  "VERSION = (\"1.2.3\",)\n",
			template: `VERSION = ("{version}",)`,
			want:     "VERSION = (\"1.3.0\",)\n",
			old:      "1.2.3",
		},
		{
			name:     "crlf",
			content:  "a\r\nversion: 1.2.3\r\nb\r\n",
			template: `version: {version}`,
			want:     "a\r\nversion: 1.3.0\r\nb\r\n",
			old:      "1.2.3",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, old, err := Replace(tt.content, tt.template, "1.3.0")
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want || old != tt.old {
				t.Errorf("got %q, %q, want %q, %q", got, old, tt.want, tt.old)
			}
		})
	}
}

func TestReplaceErrors(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		template string
	}{
		{"no match", "version = 1.2.3\n", `VERSION = "{version}"`},
		{"two matches", "\"version\": \"1.0.0\"\n\"version\": \"2.0.0\"\n", `"version": "{version}"`},
		{"not at line start", "OLD_STRATEGY_VERSION = \"1.2.3\"\n", `STRATEGY_VERSION = "{version}"`},
		{"not a version", "version = \"latest stable\"\n", `version = "{version}"`},
		{"no placeholder", "version = 1\n", `version = 1`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, _, err := Replace(tt.content, tt.template, "1.3.0"); err == nil {
				t.Error("did not fail")
			}
		})
	}
}

const npmLock = `{
  "name": "app",
  "version": "1.2.3",
  "lockfileVersion": 3,
  "requires": true,
  "packages": {
    "": {
      "name": "app",
      "version": "1.2.3",
      "dependencies": {
        "react": "^19.0.0"
      }
    },
    "node_modules/react": {
      "version": "19.0.0",
      "resolved": "https://registry.npmjs.org/react/-/react-19.0.0.tgz",
      "license": "MIT"
    }
  }
}
`

func TestReplaceNPMLock(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "two spaces",
			content: npmLock,
			want:    strings.Replace(strings.Replace(npmLock, `"version": "1.2.3"`, `"version": "1.3.0"`, 1), `"version": "1.2.3"`, `"version": "1.3.0"`, 1),
		},
		{
			name:    "tabs",
			content: "{\n\t\"name\": \"app\",\n\t\"version\": \"1.2.3\",\n\t\"packages\": {\n\t\t\"\": {\n\t\t\t\"version\": \"1.2.3\"\n\t\t},\n\t\t\"node_modules/a\": {\n\t\t\t\"version\": \"1.2.3\"\n\t\t}\n\t}\n}\n",
			want:    "{\n\t\"name\": \"app\",\n\t\"version\": \"1.3.0\",\n\t\"packages\": {\n\t\t\"\": {\n\t\t\t\"version\": \"1.3.0\"\n\t\t},\n\t\t\"node_modules/a\": {\n\t\t\t\"version\": \"1.2.3\"\n\t\t}\n\t}\n}\n",
		},
		{
			name:    "crlf",
			content: strings.ReplaceAll(npmLock, "\n", "\r\n"),
			want:    strings.ReplaceAll(strings.Replace(strings.Replace(npmLock, `"version": "1.2.3"`, `"version": "1.3.0"`, 1), `"version": "1.2.3"`, `"version": "1.3.0"`, 1), "\n", "\r\n"),
		},
		{
			name:    "root package after a dependency with the same version",
			content: "{\n  \"version\": \"1.2.3\",\n  \"packages\": {\n    \"node_modules/a\": {\n      \"version\": \"1.2.3\"\n    },\n    \"\": {\n      \"version\": \"1.2.3\"\n    }\n  }\n}\n",
			want:    "{\n  \"version\": \"1.3.0\",\n  \"packages\": {\n    \"node_modules/a\": {\n      \"version\": \"1.2.3\"\n    },\n    \"\": {\n      \"version\": \"1.3.0\"\n    }\n  }\n}\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, old, err := ReplaceNPMLock(tt.content, "1.3.0")
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want || old != "1.2.3" {
				t.Errorf("got %q, %q, want %q, %q", got, old, tt.want, "1.2.3")
			}
		})
	}
}

func TestReplaceNPMLockErrors(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{"empty", ""},
		{"no top-level version", "{\n  \"name\": \"app\",\n  \"packages\": {\n    \"\": {\n      \"version\": \"1.2.3\"\n    }\n  }\n}\n"},
		{"no root package", "{\n  \"version\": \"1.2.3\",\n  \"lockfileVersion\": 1,\n  \"dependencies\": {\n    \"a\": {\n      \"version\": \"1.0.0\"\n    }\n  }\n}\n"},
		{"root package without version", "{\n  \"version\": \"1.2.3\",\n  \"packages\": {\n    \"\": {\n      \"name\": \"app\"\n    },\n    \"node_modules/a\": {\n      \"version\": \"1.0.0\"\n    }\n  }\n}\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, _, err := ReplaceNPMLock(tt.content, "1.3.0"); err == nil {
				t.Error("did not fail")
			}
		})
	}
}
