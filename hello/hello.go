// Package hello is an example Go action.
package hello

import (
	"fmt"

	"github.com/josa42/actions/toolkit"
)

func Run() error {
	name := toolkit.Input("name")
	if name == "" {
		return fmt.Errorf("input name is required")
	}

	greeting := fmt.Sprintf("Hello, %s!", name)
	toolkit.Notice(greeting)
	return toolkit.SetOutput("greeting", greeting)
}
