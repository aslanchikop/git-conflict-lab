// Package cli implements the git-conflict-lab command surface.
package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/aslanchikop/git-conflict-lab/internal/exercise"
	"github.com/aslanchikop/git-conflict-lab/internal/lab"
	"github.com/aslanchikop/git-conflict-lab/internal/webui"
)

// Exit codes returned by Run. Keep them stable; scripts may rely on them.
const (
	// ExitOK indicates success.
	ExitOK = 0
	// ExitFailure indicates a runtime failure.
	ExitFailure = 1
	// ExitUsage indicates a usage error (unknown command, bad arguments).
	ExitUsage = 2
)

var versionString = "0.1.0-alpha.3"

const usageText = `git-conflict-lab - practice real Git conflict resolution offline

Usage:
  git-conflict-lab <command> [args]

Commands:
  list     Show the exercises available in the embedded catalog.
  start    Create an isolated exercise repository to work in.
  check    Verify your resolution inside an exercise repository.
  hint     Reveal the next hint for an exercise.
  ui       Open the local browser interface.
  version  Show the tool version.
  help     Show this help text.

Example session:

  git-conflict-lab list
  git-conflict-lab start merge-basic
  cd "<path printed by start>"
  # ... use real Git to resolve the conflict ...
  git-conflict-lab check

Run 'check' and 'hint merge-basic' from inside the generated repository.
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
	case "ui":
		return runUI(w, errW, rest)

	default:
		fmt.Fprintf(errW, "Unknown command '%s'. Run 'git-conflict-lab help' to see available commands.\n", cmd)
		return ExitUsage
	}
}

func runUI(w, errW io.Writer, args []string) int {
	if len(args) != 0 {
		fmt.Fprintln(errW, "Usage: git-conflict-lab ui")
		return ExitUsage
	}
	root, err := defaultLabsRoot()
	if err != nil {
		fmt.Fprintf(errW, "Error: %v\n", err)
		return ExitFailure
	}
	if err := ensureLabsRoot(root); err != nil {
		fmt.Fprintf(errW, "Error: %v\n", err)
		return ExitFailure
	}
	url, err := (webui.Server{Root: root}).Listen(context.Background())
	if err != nil {
		fmt.Fprintf(errW, "Error: %v\n", err)
		return ExitFailure
	}
	fmt.Fprintf(w, "Open %s in your browser. Press Ctrl+C to stop.\n", url)
	if f, ok := w.(interface{ Sync() error }); ok {
		_ = f.Sync()
	}
	select {}
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

var startAttemptGenerator = func(ctx context.Context, labsRoot, id, folder string) (string, error) {
	return lab.GenerateAs(ctx, labsRoot, id, folder)
}

// runStart validates arguments and the exercise id, then generates the
// exercise repository.
func runStart(w, errW io.Writer, args []string) int {
	if len(args) != 1 && (len(args) != 3 || args[1] != "--attempt") {
		fmt.Fprintln(errW, "Usage: git-conflict-lab start <exercise-id> [--attempt <name>]")
		fmt.Fprintln(errW, "Run 'git-conflict-lab list' to see available exercise ids.")
		return ExitUsage
	}
	if err := requireExercise(args[0]); err != nil {
		return handleUnknownExercise(errW, err)
	}
	if len(args) == 3 && !regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`).MatchString(args[2]) {
		fmt.Fprintln(errW, "Attempt name must contain lowercase letters, digits, and single hyphens.")
		return ExitUsage
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
	var created string
	if len(args) == 3 {
		created, err = startAttemptGenerator(ctx, labsRoot, args[0], args[0]+"-"+args[2])
	} else {
		created, err = startGenerator(ctx, labsRoot, args[0])
	}
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
	fmt.Fprintf(w, "  cd \"%s\"\n", strings.ReplaceAll(created, "\"", "\\\""))
	fmt.Fprintln(w, "  git log --oneline --all")
	sc, ok := lab.LookupScenario(args[0])
	if !ok {
		fmt.Fprintf(errW, "Error: unknown exercise %q\n", args[0])
		return ExitUsage
	}
	switch sc.Strategy {
	case lab.StrategyRebase:
		fmt.Fprintf(w, "  git switch %s\n", sc.Branch)
		fmt.Fprintln(w, "  git rebase main   # this will conflict, on purpose")
	case lab.StrategyCherryPick:
		fmt.Fprintf(w, "  git cherry-pick %s   # this will conflict, on purpose\n", sc.Branch)
	default:
		fmt.Fprintf(w, "  git merge %s   # this will conflict, on purpose\n", sc.Branch)
	}
	fmt.Fprintf(w, "  git-conflict-lab hint %s   # optional help\n", args[0])
	fmt.Fprintln(w, "  git-conflict-lab check")
	fmt.Fprintln(w, "If the command is not on PATH, invoke the executable by its full path.")
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

func runCheck(w, errW io.Writer, args []string) int {
	switch len(args) {
	case 0:
	case 1:
		if err := requireExercise(args[0]); err != nil {
			return handleUnknownExercise(errW, err)
		}
	default:
		fmt.Fprintln(errW, "Usage: git-conflict-lab check [exercise-id]")
		return ExitUsage
	}
	result, err := lab.Check(context.Background(), ".")
	if err != nil {
		fmt.Fprintf(errW, "Error: %v\n", err)
		return ExitFailure
	}
	if len(args) == 1 && result.ExerciseID != args[0] {
		fmt.Fprintf(errW, "Error: this repository contains %s, not %s.\n", result.ExerciseID, args[0])
		return ExitUsage
	}
	if !result.Passed {
		fmt.Fprintln(errW, result.Message)
		return ExitFailure
	}
	fmt.Fprintln(w, result.Message)
	return ExitOK
}

func runHint(w, errW io.Writer, args []string) int {
	if len(args) != 1 {
		fmt.Fprintln(errW, "Usage: git-conflict-lab hint <exercise-id>")
		return ExitUsage
	}
	if err := requireExercise(args[0]); err != nil {
		return handleUnknownExercise(errW, err)
	}
	hint, err := lab.NextHint(".", args[0])
	if err != nil {
		fmt.Fprintf(errW, "Error: %v\n", err)
		return ExitFailure
	}
	fmt.Fprintln(w, hint)
	return ExitOK
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
