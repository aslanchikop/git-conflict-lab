// Command git-conflict-lab is the entry point of the CLI. It keeps main
// minimal: recover from unexpected panics with a friendly message and
// delegate all behavior to the internal/cli package.
package main

import (
	"fmt"
	"os"

	"github.com/aslanchikop/git-conflict-lab/internal/cli"
)

func main() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintln(os.Stderr, "Unexpected internal error:", r)
			fmt.Fprintln(os.Stderr, "Please report this at https://github.com/aslanchikop/git-conflict-lab/issues")
			os.Exit(1)
		}
	}()

	os.Exit(cli.Run(os.Args[1:]))
}
