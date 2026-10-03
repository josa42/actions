package changelog

import "testing"

func TestPromote(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
		notes   string
	}{
		{
			name:    "unreleased before releases",
			content: "# Changelog\n\n## Unreleased\n\n### Fixed\n\n- A bug\n\n## 1.0.0 - 2026-01-01\n\n- First\n",
			want:    "# Changelog\n\n## 1.1.0 - 2026-10-03\n\n### Fixed\n\n- A bug\n\n## 1.0.0 - 2026-01-01\n\n- First\n",
			notes:   "### Fixed\n\n- A bug",
		},
		{
			name:    "only section",
			content: "# Changelog\n\n## Unreleased\n\n- Initial release\n",
			want:    "# Changelog\n\n## 1.1.0 - 2026-10-03\n\n- Initial release\n",
			notes:   "- Initial release",
		},
		{
			name:    "no trailing newline",
			content: "## Unreleased\n- x",
			want:    "## 1.1.0 - 2026-10-03\n- x",
			notes:   "- x",
		},
		{
			name:    "keep a changelog brackets",
			content: "## [Unreleased]\n\n- x\n\n## [1.0.0] - 2026-01-01\n\n[Unreleased]: https://example.com\n",
			want:    "## [1.1.0] - 2026-10-03\n\n- x\n\n## [1.0.0] - 2026-01-01\n\n[Unreleased]: https://example.com\n",
			notes:   "- x",
		},
		{
			name:    "case and spacing",
			content: "##   unreleased  \n- x\n",
			want:    "## 1.1.0 - 2026-10-03\n- x\n",
			notes:   "- x",
		},
		{
			name:    "crlf",
			content: "## Unreleased\r\n\r\n- x\r\n\r\n## 1.0.0\r\n",
			want:    "## 1.1.0 - 2026-10-03\r\n\r\n- x\r\n\r\n## 1.0.0\r\n",
			notes:   "- x",
		},
		{
			name:    "subsections are part of the body",
			content: "## Unreleased\n### Added\n- a\n### Fixed\n- b\n## 1.0.0\n- old\n",
			want:    "## 1.1.0 - 2026-10-03\n### Added\n- a\n### Fixed\n- b\n## 1.0.0\n- old\n",
			notes:   "### Added\n- a\n### Fixed\n- b",
		},
		{
			name:    "similar version is not a duplicate",
			content: "## Unreleased\n- x\n## 1.1.0.1\n## 1.1.01\n## 11.1.0\n",
			want:    "## 1.1.0 - 2026-10-03\n- x\n## 1.1.0.1\n## 1.1.01\n## 11.1.0\n",
			notes:   "- x",
		},
		{
			name:    "unreleased in text is ignored",
			content: "Unreleased changes go below.\n\n## Unreleased\n- x\n",
			want:    "Unreleased changes go below.\n\n## 1.1.0 - 2026-10-03\n- x\n",
			notes:   "- x",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, notes, err := Promote(tt.content, "1.1.0", "2026-10-03")
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Errorf("content = %q, want %q", got, tt.want)
			}
			if notes != tt.notes {
				t.Errorf("notes = %q, want %q", notes, tt.notes)
			}
		})
	}
}

func TestPromoteErrors(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{"no unreleased", "# Changelog\n\n## 1.0.0\n- x\n"},
		{"unreleased as h3", "### Unreleased\n- x\n"},
		{"unreleased with suffix", "## Unreleased changes\n- x\n"},
		{"empty", "## Unreleased\n\n## 1.0.0\n- x\n"},
		{"blank at end", "## Unreleased\n  \n\n"},
		{"two unreleased", "## Unreleased\n- a\n## Unreleased\n- b\n"},
		{"version exists", "## Unreleased\n- x\n## 1.1.0 - 2026-01-01\n"},
		{"version exists in brackets", "## Unreleased\n- x\n## [1.1.0]\n"},
		{"version exists undated", "## Unreleased\n- x\n## 1.1.0\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, _, err := Promote(tt.content, "1.1.0", "2026-10-03"); err == nil {
				t.Error("did not fail")
			}
		})
	}
}
