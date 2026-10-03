package semver

import "testing"

func TestParse(t *testing.T) {
	for s, want := range map[string]Version{
		"0.0.0":    {0, 0, 0},
		"1.2.3":    {1, 2, 3},
		"10.20.30": {10, 20, 30},
	} {
		got, err := Parse(s)
		if err != nil || got != want {
			t.Errorf("Parse(%q) = %v, %v", s, got, err)
		}
		if got.String() != s {
			t.Errorf("String() = %q, want %q", got.String(), s)
		}
	}

	for _, s := range []string{"", "1", "1.2", "1.2.3.4", "v1.2.3", "01.2.3", "1.02.3", "1.2.3-beta", "1.2.3+build", "a.b.c", " 1.2.3"} {
		if _, err := Parse(s); err == nil {
			t.Errorf("Parse(%q) did not fail", s)
		}
	}
}

func TestCompare(t *testing.T) {
	tests := []struct {
		a, b Version
		want int
	}{
		{Version{1, 2, 3}, Version{1, 2, 3}, 0},
		{Version{1, 2, 3}, Version{1, 2, 4}, -1},
		{Version{1, 3, 0}, Version{1, 2, 9}, 1},
		{Version{2, 0, 0}, Version{1, 99, 99}, 1},
		{Version{0, 9, 0}, Version{0, 10, 0}, -1},
	}
	for _, tt := range tests {
		if got := tt.a.Compare(tt.b); got != tt.want {
			t.Errorf("%v.Compare(%v) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestBump(t *testing.T) {
	v := Version{1, 2, 3}
	for part, want := range map[string]Version{
		"major": {2, 0, 0},
		"minor": {1, 3, 0},
		"patch": {1, 2, 4},
	} {
		if got, err := v.Bump(part); err != nil || got != want {
			t.Errorf("Bump(%q) = %v, %v", part, got, err)
		}
	}
	if _, err := v.Bump("micro"); err == nil {
		t.Error("Bump(micro) did not fail")
	}
}

func TestLatest(t *testing.T) {
	tags := []string{"v1.2.0", "v1.10.0", "v1.9.9", "1.20.0", "v2.0.0-beta", "latest", "v1.10.0-rc.1", "release-3.0.0"}

	tests := []struct {
		format string
		want   Version
		ok     bool
	}{
		{"v{version}", Version{1, 10, 0}, true},
		{"{version}", Version{1, 20, 0}, true},
		{"release-{version}", Version{3, 0, 0}, true},
		{"x{version}", Version{}, false},
		{"v", Version{}, false},
	}
	for _, tt := range tests {
		got, ok := Latest(tags, tt.format)
		if got != tt.want || ok != tt.ok {
			t.Errorf("Latest(%q) = %v, %v, want %v, %v", tt.format, got, ok, tt.want, tt.ok)
		}
	}

	if _, ok := Latest(nil, "v{version}"); ok {
		t.Error("Latest(nil) found a version")
	}
}

func TestNext(t *testing.T) {
	current := Version{1, 2, 3}

	tests := []struct {
		input      string
		current    Version
		hasCurrent bool
		want       Version
		err        bool
	}{
		{"patch", current, true, Version{1, 2, 4}, false},
		{"minor", current, true, Version{1, 3, 0}, false},
		{"major", current, true, Version{2, 0, 0}, false},
		{"patch", Version{}, false, Version{0, 0, 1}, false},
		{"minor", Version{}, false, Version{0, 1, 0}, false},
		{"major", Version{}, false, Version{1, 0, 0}, false},
		{"1.2.4", current, true, Version{1, 2, 4}, false},
		{"2.0.0", current, true, Version{2, 0, 0}, false},
		{"0.1.0", Version{}, false, Version{0, 1, 0}, false},
		{"1.2.3", current, true, Version{}, true},
		{"1.2.2", current, true, Version{}, true},
		{"v1.3.0", current, true, Version{}, true},
		{"Patch", current, true, Version{}, true},
		{"", current, true, Version{}, true},
	}
	for _, tt := range tests {
		got, err := Next(tt.input, tt.current, tt.hasCurrent)
		if (err != nil) != tt.err || got != tt.want {
			t.Errorf("Next(%q, %v, %v) = %v, %v", tt.input, tt.current, tt.hasCurrent, got, err)
		}
	}
}
