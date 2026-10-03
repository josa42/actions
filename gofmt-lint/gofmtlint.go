// Package gofmtlint implements the gofmt-lint action.
package gofmtlint

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"go/format"
	"go/scanner"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/josa42/actions/toolkit"
)

// maxCells caps the line diff table. Larger changes are reported as one
// block.
const maxCells = 4 << 20

// hunk is a range of lines that gofmt changes. Lines are 1-based, end is
// exclusive, so start == end is an insertion before start.
type hunk struct {
	start, end int
	want       []string
}

func Run(ctx context.Context) error {
	dir := toolkit.Input("working-directory")
	if dir == "" {
		dir = "."
	}
	root := os.Getenv("GITHUB_WORKSPACE")

	files, err := goFiles(dir)
	if err != nil {
		return err
	}

	var bad []string
	for _, f := range files {
		if err := ctx.Err(); err != nil {
			return err
		}
		name := relPath(root, f)
		ok, err := check(f, name)
		if err != nil {
			return err
		}
		if !ok {
			bad = append(bad, name)
		}
	}

	if err := toolkit.SetOutput("files", strings.Join(bad, "\n")); err != nil {
		toolkit.Warning("Could not set the files output: " + err.Error())
	}
	summary(bad, len(files))

	if len(bad) > 0 {
		return fmt.Errorf("%d of %d files are not formatted with gofmt", len(bad), len(files))
	}
	fmt.Printf("All %d files are formatted with gofmt\n", len(files))
	return nil
}

// goFiles lists the .go files under dir, skipping the directories the go
// tool ignores and vendor.
func goFiles(dir string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if d.IsDir() {
			if path != dir && (name == "vendor" || name == "testdata" || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_")) {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Type().IsRegular() && strings.HasSuffix(name, ".go") && !strings.HasPrefix(name, ".") {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

// check annotates the lines of path that gofmt would change, or its syntax
// errors, and reports whether the file is formatted. It never writes path.
func check(path, name string) (bool, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	out, err := format.Source(src)
	if err != nil {
		var list scanner.ErrorList
		if !errors.As(err, &list) {
			return false, fmt.Errorf("%s: %w", name, err)
		}
		for _, e := range list {
			toolkit.Error(e.Msg, toolkit.Annotation{Title: "gofmt", File: name, Line: e.Pos.Line, Col: e.Pos.Column})
		}
		return false, nil
	}
	if bytes.Equal(src, out) {
		return true, nil
	}

	lines := splitLines(string(src))
	for _, h := range diff(lines, splitLines(string(out))) {
		a := toolkit.Annotation{Title: "gofmt", File: name, Line: h.start, EndLine: h.end - 1}
		if h.start == h.end {
			// An insertion has no lines of its own, mark the line before it.
			a.Line = max(1, min(h.start-1, len(lines)))
			a.EndLine = a.Line
		}
		toolkit.Error(message(h), a)
	}
	return false, nil
}

func message(h hunk) string {
	if len(h.want) == 0 {
		return "Not formatted with gofmt. Remove these lines."
	}
	verb := "Expected"
	if h.start == h.end {
		verb = "Expected after this line"
	}
	return "Not formatted with gofmt. " + verb + ":\n" + strings.Join(h.want, "\n")
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(strings.TrimSuffix(s, "\n"), "\n")
}

// diff returns the hunks that turn a into b, from a longest common
// subsequence of the lines after trimming the common prefix and suffix.
func diff(a, b []string) []hunk {
	pre := 0
	for pre < len(a) && pre < len(b) && a[pre] == b[pre] {
		pre++
	}
	suf := 0
	for suf < len(a)-pre && suf < len(b)-pre && a[len(a)-1-suf] == b[len(b)-1-suf] {
		suf++
	}
	a, b = a[pre:len(a)-suf], b[pre:len(b)-suf]
	if len(a) == 0 && len(b) == 0 {
		return nil
	}
	if len(a)*len(b) > maxCells {
		return []hunk{{start: pre + 1, end: pre + 1 + len(a), want: b}}
	}

	// lcs[i][j] is the length of the LCS of a[i:] and b[j:].
	w := len(b) + 1
	lcs := make([]int32, (len(a)+1)*w)
	for i := len(a) - 1; i >= 0; i-- {
		for j := len(b) - 1; j >= 0; j-- {
			if a[i] == b[j] {
				lcs[i*w+j] = lcs[(i+1)*w+j+1] + 1
			} else {
				lcs[i*w+j] = max(lcs[(i+1)*w+j], lcs[i*w+j+1])
			}
		}
	}

	var hunks []hunk
	var cur *hunk
	i, j := 0, 0
	for i < len(a) || j < len(b) {
		if i < len(a) && j < len(b) && a[i] == b[j] {
			cur = nil
			i++
			j++
			continue
		}
		if cur == nil {
			hunks = append(hunks, hunk{start: pre + i + 1, end: pre + i + 1})
			cur = &hunks[len(hunks)-1]
		}
		if j < len(b) && (i == len(a) || lcs[i*w+j+1] >= lcs[(i+1)*w+j]) {
			cur.want = append(cur.want, b[j])
			j++
		} else {
			cur.end++
			i++
		}
	}
	return hunks
}

func relPath(root, path string) string {
	if root != "" {
		if abs, err := filepath.Abs(path); err == nil {
			if rel, err := filepath.Rel(root, abs); err == nil && !strings.HasPrefix(rel, "..") {
				path = rel
			}
		}
	}
	return filepath.ToSlash(filepath.Clean(path))
}

func summary(bad []string, total int) {
	var b strings.Builder
	b.WriteString("### gofmt\n\n")
	if len(bad) == 0 {
		fmt.Fprintf(&b, "All %d files are formatted.\n", total)
	} else {
		fmt.Fprintf(&b, "%d of %d files are not formatted. Run `gofmt -w` on:\n\n", len(bad), total)
		for _, f := range bad {
			fmt.Fprintf(&b, "- `%s`\n", f)
		}
	}
	if err := toolkit.AddSummary(b.String()); err != nil {
		toolkit.Warning("Could not write the job summary: " + err.Error())
	}
}
