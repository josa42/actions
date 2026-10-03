// Package toolkit implements the GitHub Actions runner protocol: inputs,
// outputs, environment files and workflow commands.
//
// See https://docs.github.com/en/actions/reference/workflow-commands-for-github-actions
package toolkit

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

var stdout io.Writer = os.Stdout

// Input returns the value of the action input name, with surrounding
// whitespace trimmed. Missing inputs return "".
func Input(name string) string {
	key := "INPUT_" + strings.ToUpper(strings.ReplaceAll(name, " ", "_"))
	return strings.TrimSpace(os.Getenv(key))
}

// InputBool parses the input name as a YAML 1.2 core schema boolean.
// Missing inputs return false.
func InputBool(name string) (bool, error) {
	switch v := Input(name); v {
	case "true", "True", "TRUE":
		return true, nil
	case "false", "False", "FALSE", "":
		return false, nil
	default:
		return false, fmt.Errorf("input %s: %q is not a boolean", name, v)
	}
}

// InputList splits the input name on newlines, trimming each line and
// dropping empty ones.
func InputList(name string) []string {
	var list []string
	for _, line := range strings.Split(Input(name), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			list = append(list, line)
		}
	}
	return list
}

// SetOutput sets the step output name.
func SetOutput(name, value string) error {
	return appendKeyValue("GITHUB_OUTPUT", name, value)
}

// SetState saves state for the post step of the action.
func SetState(name, value string) error {
	return appendKeyValue("GITHUB_STATE", name, value)
}

// ExportVariable sets an environment variable for this and all later steps.
func ExportVariable(name, value string) error {
	if err := appendKeyValue("GITHUB_ENV", name, value); err != nil {
		return err
	}
	return os.Setenv(name, value)
}

// AddPath prepends dir to PATH for this and all later steps.
func AddPath(dir string) error {
	if err := appendFile("GITHUB_PATH", dir+"\n"); err != nil {
		return err
	}
	return os.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// AddSummary appends markdown to the job summary.
func AddSummary(markdown string) error {
	return appendFile("GITHUB_STEP_SUMMARY", markdown+"\n")
}

// SetSecret masks value in all later log output.
func SetSecret(value string) {
	command("add-mask", nil, value)
}

// IsDebug reports whether step debug logging is enabled.
func IsDebug() bool {
	return os.Getenv("RUNNER_DEBUG") == "1"
}

// Annotation places a message on a file and line in the workflow run and
// pull request diff.
type Annotation struct {
	Title     string
	File      string
	Line      int
	EndLine   int
	Col       int
	EndColumn int
}

func (a Annotation) properties() map[string]string {
	p := map[string]string{}
	set := func(k, v string) {
		if v != "" {
			p[k] = v
		}
	}
	setInt := func(k string, v int) {
		if v > 0 {
			p[k] = strconv.Itoa(v)
		}
	}
	set("title", a.Title)
	set("file", a.File)
	setInt("line", a.Line)
	setInt("endLine", a.EndLine)
	setInt("col", a.Col)
	setInt("endColumn", a.EndColumn)
	return p
}

// Debug logs msg, visible only when step debug logging is enabled.
func Debug(msg string) { command("debug", nil, msg) }

// Notice logs msg as a notice annotation.
func Notice(msg string, a ...Annotation) { annotate("notice", msg, a) }

// Warning logs msg as a warning annotation.
func Warning(msg string, a ...Annotation) { annotate("warning", msg, a) }

// Error logs msg as an error annotation. It does not fail the step.
func Error(msg string, a ...Annotation) { annotate("error", msg, a) }

func annotate(name, msg string, a []Annotation) {
	var p map[string]string
	if len(a) > 0 {
		p = a[0].properties()
	}
	command(name, p, msg)
}

// Group runs fn with its log output folded under name.
func Group(name string, fn func() error) error {
	command("group", nil, name)
	defer command("endgroup", nil, "")
	return fn()
}

func command(name string, props map[string]string, msg string) {
	var b strings.Builder
	b.WriteString("::" + name)
	sep := " "
	// Fixed order keeps output stable.
	for _, k := range []string{"title", "file", "line", "endLine", "col", "endColumn"} {
		if v, ok := props[k]; ok {
			b.WriteString(sep + k + "=" + escapeProperty(v))
			sep = ","
		}
	}
	b.WriteString("::" + escapeData(msg) + "\n")
	io.WriteString(stdout, b.String())
}

func escapeData(s string) string {
	return strings.NewReplacer("%", "%25", "\r", "%0D", "\n", "%0A").Replace(s)
}

func escapeProperty(s string) string {
	return strings.NewReplacer("%", "%25", "\r", "%0D", "\n", "%0A", ":", "%3A", ",", "%2C").Replace(s)
}

func appendKeyValue(envVar, name, value string) error {
	delim, err := delimiter()
	if err != nil {
		return err
	}
	if strings.Contains(name, delim) || strings.Contains(value, delim) {
		return fmt.Errorf("%s: value contains delimiter", name)
	}
	return appendFile(envVar, name+"<<"+delim+"\n"+value+"\n"+delim+"\n")
}

func appendFile(envVar, content string) error {
	path := os.Getenv(envVar)
	if path == "" {
		return fmt.Errorf("%s is not set", envVar)
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.WriteString(content); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

func delimiter() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "ghadelimiter_" + hex.EncodeToString(b), nil
}
