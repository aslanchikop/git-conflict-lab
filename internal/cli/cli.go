// Package cli implements the git-conflict-lab command surface: argument
// dispatch, usage text, and stable exit codes. Runtime work is delegated
// to other packages; this milestone wires the commands that exist today
// and reports the ones that arrive in later milestones.
package cli

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/aslanchikop/git-conflict-lab/internal/exercise"
	"github.com/aslanchikop/git-conflict-lab/internal/lab"
)

// Exit codes returned by Run. Keep them stable; scripts may rely on them.
const (
	// ExitOK indicates success.
	ExitOK = 0
	// ExitFailure indicates a runtime failure.
	ExitFailure = 1
	// ExitUsage indicates a usage error (unknown command, bad arguments).
	ExitUsage = 2
	// ExitNotImplemented indicates a known command whose implementation
	// lands in a later milestone (lab.NotAvailableError).
	ExitNotImplemented = 4
)

const versionString = "0.1.0-alpha.1"

const usageText = `git-conflict-lab - practice real Git conflict resolution offline

Usage:
  git-conflict-lab <command> [args]

Commands:
  list     Show the exercises available in the embedded catalog.
  start    Create an isolated exercise repository to work in. (Milestone B)
  check    Verify your resolution inside an exercise repository. (Milestone C)
  hint     Reveal the next hint for an exercise. (Milestone D)
  version  Show the tool version.
  help     Show this help text.

Example session (after Milestone B):

  git-conflict-lab list
  git-conflict-lab start merge-basic
  cd merge-basic
  # ... use real Git to resolve the conflict ...
  git-conflict-lab check

Today 'list', 'version' and 'help' are fully functional; the remaining
commands are reserved and report the milestone that delivers them.
`

// Run executes the CLI with the given arguments (without the program
// name) and returns the process exit code. Output goes to stdout; usage
// and error output go to stderr.
func Run(args []string) int {
	return run(os.Stdout, os.Stderr, args)
}

// run is the testable core of Run. It splits normal output (w) from
// error output (errW) so tests can capture both.
func run(w, errW io.Writer, args []string) int {
	if len(args) == 0 {
		fmt.Fprint(w, usageText)
		return ExitOK
	}

	cmd, rest := args[0], args[1:]
	switch cmd {
	case "help":
		fmt.Fprint(w, usageText)
		return ExitOK

	case "version":
		fmt.Fprintf(w, "git-conflict-lab %s\n", versionString)
		fmt.Fprintln(w, "An offline CLI trainer for practicing real Git conflict resolution.")
		return ExitOK

	case "list":
		return runList(w, errW)

	case "start":
		return runStart(w, errW, rest)

	case "check":
		return runCheck(w, errW, rest)

	case "hint":
		return runHint(w, errW, rest)

	default:
		fmt.Fprintf(errW, "Unknown command '%s'. Run 'git-conflict-lab help' to see available commands.\n", cmd)
		return ExitUsage
	}
}

// runList prints one block per exercise from the catalog.
func runList(w, errW io.Writer) int {
	specs := exercise.List()
	fmt.Fprintln(w, "Available exercises:")
	for _, spec := range specs {
		fmt.Fprintf(w, "  %s  (%s)  %s\n", spec.ID, spec.Difficulty, spec.Title)
		objective := spec.Objective
		if objective != "" {
			fmt.Fprintf(w, "      %s\n", objective)
		}
	}
	return ExitOK
}

// runStart validates arguments and the exercise id. Directory generation
// itself arrives in Milestone B.
func runStart(w, errW io.Writer, args []string) int {
	if len(args) != 1 {
		fmt.Fprintln(errW, "Usage: git-conflict-lab start <exercise-id>")
		fmt.Fprintln(errW, "Run 'git-conflict-lab list' to see available exercise ids.")
		return ExitUsage
	}
	if err := requireExercise(args[0]); err != nil {
		return handleUnknownExercise(errW, err)
	}
	return reportNotAvailable(errW, "start", "B")
}

// runCheck optionally validates an exercise id. Verification arrives in
// Milestone C.
func runCheck(w, errW io.Writer, args []string) int {
	switch len(args) {
	case 0:
		// No id yet: the checker itself lands in Milestone C.
	case 1:
		if err := requireExercise(args[0]); err != nil {
			return handleUnknownExercise(errW, err)
		}
	default:
		fmt.Fprintln(errW, "Usage: git-conflict-lab check [exercise-id]")
		return ExitUsage
	}
	return reportNotAvailable(errW, "check", "C")
}

// runHint validates the exercise id. Hint delivery arrives in Milestone D.
func runHint(w, errW io.Writer, args []string) int {
	if len(args) != 1 {
		fmt.Fprintln(errW, "Usage: git-conflict-lab hint <exercise-id>")
		return ExitUsage
	}
	if err := requireExercise(args[0]); err != nil {
		return handleUnknownExercise(errW, err)
	}
	return reportNotAvailable(errW, "hint", "D")
}

// requireExercise resolves an exercise id, normalizing lookup errors so
// callers only need to test for exercise.ErrNotFound.
func requireExercise(id string) error {
	_, err := exercise.Get(id)
	return err
}

// handleUnknownExercise prints the friendly not-found message for usage
// errors and returns the matching exit code; other errors are runtime
// failures.
func handleUnknownExercise(errW io.Writer, err error) int {
	if !errors.Is(err, exercise.ErrNotFound) {
		fmt.Fprintf(errW, "Error: %v\n", err)
		return ExitFailure
	}
	fmt.Fprintf(errW, "Unknown exercise. Run 'git-conflict-lab list' to see available exercise ids.\n")
	return ExitUsage
}

// reportNotAvailable prints the shared not-implemented message and maps
// it to the dedicated exit code.
func reportNotAvailable(errW io.Writer, command, milestone string) int {
	err := &lab.NotAvailableError{Command: command, Milestone: milestone}
	fmt.Fprintln(errW, err.Error())
	return ExitNotImplemented
}
