package markdown

import (
	"strings"
	"testing"
)

func lines(prefix string, n int) string {
	var b strings.Builder
	for i := range n {
		b.WriteString(prefix + strings.Repeat("x", 9-len(prefix)) + string(rune('0'+i%10)) + "\n")
	}
	return b.String()
}

func TestLen(t *testing.T) {
	for s, want := range map[string]int{"": 0, "abc": 3, "äö": 2, "😀": 2, "a😀b": 4} {
		if got := Len(s); got != want {
			t.Errorf("Len(%q) = %d, want %d", s, got, want)
		}
	}
}

func TestTruncateShort(t *testing.T) {
	s := "line 1\nline 2\n"
	for _, limit := range []int{Len(s), Len(s) + 1} {
		if got, truncated := Truncate(s, limit); got != s || truncated {
			t.Errorf("Truncate(s, %d) = %q, %v", limit, got, truncated)
		}
	}
}

func TestTruncateLineBoundary(t *testing.T) {
	s := lines("", 100) // 100 lines of 10 characters plus newline
	limit := Len(TruncatedNotice) + 55

	got, truncated := Truncate(s, limit)

	if !truncated {
		t.Fatal("not truncated")
	}
	if want := strings.Join(strings.Split(s, "\n")[:5], "\n") + TruncatedNotice; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestTruncateClosesFence(t *testing.T) {
	tests := []struct {
		name  string
		open  string
		close string
	}{
		{"backticks", "```go", "```"},
		{"long backticks", "`````", "`````"},
		{"tildes", "~~~", "~~~"},
		{"indented", "   ```", "```"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := "intro\n" + tt.open + "\n" + lines("", 100) + tt.close + "\n"
			got, _ := Truncate(s, Len(TruncatedNotice)+100)

			body := strings.TrimSuffix(got, TruncatedNotice)
			if !strings.HasSuffix(body, "\n"+strings.TrimSpace(tt.open)[:len(tt.close)]) {
				t.Errorf("fence not closed: %q", got)
			}
			if Len(got) > Len(TruncatedNotice)+100 {
				t.Errorf("too long: %d", Len(got))
			}
		})
	}
}

func TestTruncateClosedFence(t *testing.T) {
	s := "```\ncode\n```\n" + lines("", 100)
	got, _ := Truncate(s, Len(TruncatedNotice)+50)

	if strings.Count(got, "```") != 2 {
		t.Errorf("unexpected fences: %q", got)
	}
}

func TestTruncateFenceNeedsMatchingClose(t *testing.T) {
	// ``` inside a ~~~ fence and a shorter ``` inside a ```` fence don't close it.
	for _, open := range []string{"~~~", "````"} {
		s := open + "\n```\n" + lines("", 100)
		got, _ := Truncate(s, Len(TruncatedNotice)+50)

		body := strings.TrimSuffix(got, TruncatedNotice)
		if !strings.HasSuffix(body, "\n"+open) {
			t.Errorf("%s fence not closed: %q", open, got)
		}
	}
}

func TestTruncateInlineBackticksAreNoFence(t *testing.T) {
	s := "```inline``` code\n" + lines("", 100)
	got, _ := Truncate(s, Len(TruncatedNotice)+50)

	if strings.Count(got, "```") != 2 {
		t.Errorf("unexpected fence: %q", got)
	}
}

func TestTruncateClosesDetails(t *testing.T) {
	s := "<details>\n<summary>outer</summary>\n" +
		"<DETAILS open>\n<summary>inner</summary>\n" +
		lines("", 100) +
		"</details>\n</details>\n"
	got, _ := Truncate(s, Len(TruncatedNotice)+120)

	body := strings.TrimSuffix(got, TruncatedNotice)
	if !strings.HasSuffix(body, "\n</details>\n</details>") {
		t.Errorf("details not closed: %q", got)
	}
	if Len(got) > Len(TruncatedNotice)+120 {
		t.Errorf("too long: %d", Len(got))
	}
}

func TestTruncateClosedDetails(t *testing.T) {
	s := "<details><summary>a</summary>\nbody\n</details>\n" + lines("", 100)
	got, _ := Truncate(s, Len(TruncatedNotice)+80)

	if strings.Count(got, "</details>") != 1 {
		t.Errorf("unexpected closing tags: %q", got)
	}
}

func TestTruncateDetailsInFence(t *testing.T) {
	s := "```html\n<details>\n```\n" + lines("", 100)
	got, _ := Truncate(s, Len(TruncatedNotice)+50)

	if strings.Contains(got, "</details>") {
		t.Errorf("closed details from a code block: %q", got)
	}
}

func TestTruncateFenceAndDetails(t *testing.T) {
	s := "<details>\n\n```\n" + lines("", 100) + "```\n</details>\n"
	got, _ := Truncate(s, Len(TruncatedNotice)+80)

	body := strings.TrimSuffix(got, TruncatedNotice)
	if !strings.HasSuffix(body, "\n```\n</details>") {
		t.Errorf("not closed in order: %q", got)
	}
}

func TestTruncateLongLine(t *testing.T) {
	s := strings.Repeat("ab", 100)
	got, truncated := Truncate(s, Len(TruncatedNotice)+15)

	if want := s[:15] + TruncatedNotice; got != want || !truncated {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestTruncateCountsUTF16(t *testing.T) {
	s := strings.Repeat("😀\n", 50)
	limit := Len(TruncatedNotice) + 10

	got, _ := Truncate(s, limit)

	if Len(got) > limit {
		t.Errorf("too long: %d > %d", Len(got), limit)
	}
	if want := "😀\n😀\n😀" + TruncatedNotice; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestTruncateNeverExceedsLimit(t *testing.T) {
	s := "<details>\n\n```\n" + lines("", 30) + "```\n</details>\n" + lines("", 30)
	for limit := Len(TruncatedNotice) + 30; limit < Len(s); limit++ {
		got, truncated := Truncate(s, limit)
		if !truncated || Len(got) > limit {
			t.Fatalf("Truncate(s, %d): len %d, truncated %v", limit, Len(got), truncated)
		}
	}
}
