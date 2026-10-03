// Command toolkit runs the Go actions of this repository. Each action's
// index.js invokes it with the action name as the only argument.
package main

import (
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/josa42/actions/hello"
	"github.com/josa42/actions/toolkit"
)

var actions = map[string]func() error{
	"hello": hello.Run,
}

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintf(os.Stderr, "usage: toolkit <%s>\n", strings.Join(names(), "|"))
		os.Exit(2)
	}

	run, ok := actions[os.Args[1]]
	if !ok {
		fmt.Fprintf(os.Stderr, "unknown action %q\n", os.Args[1])
		os.Exit(2)
	}

	if err := run(); err != nil {
		toolkit.Error(err.Error())
		os.Exit(1)
	}
}

func names() []string {
	var n []string
	for name := range actions {
		n = append(n, name)
	}
	slices.Sort(n)
	return n
}
