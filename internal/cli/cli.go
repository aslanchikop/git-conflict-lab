// Package cli implements the git-conflict-lab command surface: argument
// dispatch, usage text, and stable exit codes. Runtime work is delegated
// to other packages; this milestone wires the commands that exist today
// and reports the ones that arrive in later milestones.
package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/aslanchikop/git-conflict-lab/internal/check"
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
  start    Create an isolated exercise repository to work in.
  check    Verify your resolution inside an exercise repository.
  hint     Reveal the next hint for an exercise. (Milestone D)
  version  Show the tool version.
  help     Show this help text.

Example session (after Milestone B):

  git-conflict-lab list
  git-conflict-lab start merge-basic
  cd merge-basic
  # ... use real Git to resolve the conflict ...
  git-conflict-lab check

Today 'list', 'start', 'check', 'version' and 'help' are fully functional; the remaining
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

// labsRootEnv is the environment variable that overrides the default
// labs root. It keeps tests and user setups isolated.
const labsRootEnv = "GIT_CONFLICT_LAB_LABS"

// defaultLabsRoot resolves the labs root: the override variable first,
// then a fixed directory under the user home. The directory is created
// on demand. It is a package variable so tests can inject a fake.
var defaultLabsRoot = func() (string, error) {
	if v := os.Getenv(labsRootEnv); v != "" {
		return filepath.Abs(v)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "git-conflict-lab-labs"), nil
}

// startGenerator generates the exercise repository. It is a package
// variable so tests can inject failures.
var startGenerator = func(ctx context.Context, labsRoot, id string) (string, error) {
	return lab.Generate(ctx, labsRoot, id)
}

// runStart validates arguments and the exercise id, then generates the
// exercise repository.
func runStart(w, errW io.Writer, args []string) int {
	if len(args) != 1 {
		fmt.Fprintln(errW, "Usage: git-conflict-lab start <exercise-id>")
		fmt.Fprintln(errW, "Run 'git-conflict-lab list' to see available exercise ids.")
		return ExitUsage
	}
	if err := requireExercise(args[0]); err != nil {
		return handleUnknownExercise(errW, err)
	}
	labsRoot, err := defaultLabsRoot()
	if err != nil {
		fmt.Fprintf(errW, "Error: cannot determine the labs directory: %v\n", err)
		return ExitFailure
	}
	if err := ensureLabsRoot(labsRoot); err != nil {
		fmt.Fprintf(errW, "Error: %v\n", err)
		return ExitFailure
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	created, err := startGenerator(ctx, labsRoot, args[0])
	if err != nil {
		if errors.Is(err, lab.ErrTargetExists) || errors.Is(err, lab.ErrTargetNotEmpty) {
			fmt.Fprintf(errW, "That exercise directory already exists, so nothing was changed.\n")
			fmt.Fprintf(errW, "Remove it yourself if you want a fresh copy, then run the command again.\n")
			return ExitFailure
		}
		fmt.Fprintf(errW, "Error: %v\n", err)
		return ExitFailure
	}
	fmt.Fprintf(w, "Exercise ready: %s\n", created)
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Next steps:")
	fmt.Fprintf(w, "  cd %s\n", filepath.Base(created))
	fmt.Fprintln(w, "  git log --oneline --all")
	fmt.Fprintln(w, "  git merge feature/login   # this will conflict, on purpose")
	fmt.Fprintln(w, "  git-conflict-lab check")
	return ExitOK
}

// ensureLabsRoot creates the labs root on demand.
func ensureLabsRoot(labsRoot string) error {
	info, err := os.Stat(labsRoot)
	if err == nil {
		if !info.IsDir() {
			return fmt.Errorf("labs root %s exists but is not a directory", labsRoot)
		}
		return nil
	}
	if !os.IsNotExist(err) {
		return err
	}
	return os.MkdirAll(labsRoot, 0o755)
}

// runCheck verifies the exercise in the current working directory. An
// optional exercise id must match the generated exercise if given.
func runCheck(w, errW io.Writer, args []string) int {
	switch len(args) {
	case 0, 1:
	default:
		fmt.Fprintln(errW, "Usage: git-conflict-lab check [exercise-id]")
		return ExitUsage
	}
	if len(args) == 1 {
		if err := requireExercise(args[0]); err != nil {
			return handleUnknownExercise(errW, err)
		}
	}
	wd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(errW, "Error: cannot determine the current directory: %v\n", err)
		return ExitFailure
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	result, err := checkFn(ctx, wd)
	if err != nil {
		fmt.Fprintf(errW, "Error: %v\n", err)
		return ExitFailure
	}
	for _, c := range result.Checks {
		if c.Passed {
			fmt.Fprintf(w, "ok   %s - %s\n", c.Name, c.Detail)
		} else {
			fmt.Fprintf(errW, "FAIL %s - %s\n", c.Name, c.Detail)
		}
	}
	if !result.Passed {
		fmt.Fprintln(errW, "")
		fmt.Fprintln(errW, "Not solved yet. Work through the FAIL lines above, then run git-conflict-lab check again.")
		return ExitFailure
	}
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Solved. You resolved a real merge conflict: both sides were preserved,")
	fmt.Fprintln(w, "the result was committed on main, and the worktree is clean.")
	return ExitOK
}

// checkFn is the checker entry point; a package variable so tests can
// inject behavior.
var checkFn = check.Check

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

// lookupExercise resolves an exercise id through the catalog. It is a
// package variable so tests can inject lookup failures.
var lookupExercise = exercise.Get

// requireExercise resolves an exercise id, normalizing lookup errors so
// callers only need to test for exercise.ErrNotFound.
func requireExercise(id string) error {
	_, err := lookupExercise(id)
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
