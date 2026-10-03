// Command toolkit runs the Go actions of this repository. Each action's
// index.js invokes it with the action name as the only argument.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"slices"
	"strings"
	"syscall"

	changedfiles "github.com/josa42/actions/get-changed-files"
	getpr "github.com/josa42/actions/get-pr"
	prcomment "github.com/josa42/actions/pr-comment"
	"github.com/josa42/actions/toolkit"
)

var actions = map[string]func(context.Context) error{
	"get-changed-files": changedfiles.Run,
	"get-pr":            getpr.Run,
	"pr-comment":        prcomment.Run,
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

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx); err != nil {
		toolkit.Error(err.Error())
		stop()
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
