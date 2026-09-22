package cli

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/aslanchikop/git-conflict-lab/internal/exercise"
)

// runArgs executes run with fresh stdout/stderr buffers and returns both
// captures plus the exit code.
func runArgs(args ...string) (stdout, stderr string, code int) {
	var out, errOut bytes.Buffer
	code = run(&out, &errOut, args)
	return out.String(), errOut.String(), code
}

func TestHelpListsAllCommands(t *testing.T) {
	for _, args := range [][]string{nil, {"help"}} {
		stdout, _, code := runArgs(args...)
		if code != ExitOK {
			t.Fatalf("help: exit = %d, want %d", code, ExitOK)
		}
		for _, want := range []string{"list", "start", "check", "hint", "version", "help"} {
			if !strings.Contains(stdout, want) {
				t.Errorf("help output missing command %q", want)
			}
		}
		if !strings.Contains(stdout, "git-conflict-lab start merge-basic") {
			t.Error("help output missing example session")
		}
	}
}

func TestVersionPrintsVersion(t *testing.T) {
	stdout, _, code := runArgs("version")
	if code != ExitOK {
		t.Fatalf("version: exit = %d, want %d", code, ExitOK)
	}
	if !strings.Contains(stdout, "git-conflict-lab 0.1.0-alpha.1") {
		t.Errorf("version output = %q, want version string", stdout)
	}
}

func TestListShowsCatalogEntry(t *testing.T) {
	stdout, _, code := runArgs("list")
	if code != ExitOK {
		t.Fatalf("list: exit = %d, want %d", code, ExitOK)
	}
	if !strings.Contains(stdout, "Available exercises:") {
		t.Error("list output missing header")
	}
	for _, want := range []string{"merge-basic", "easy"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("list output missing %q", want)
		}
	}
}

func TestStartRequiresOneArg(t *testing.T) {
	cases := [][]string{
		{"start"},
		{"start", "merge-basic", "extra"},
	}
	for _, args := range cases {
		_, stderr, code := runArgs(args...)
		if code != ExitUsage {
			t.Errorf("start %v: exit = %d, want %d", args, code, ExitUsage)
		}
		if !strings.Contains(stderr, "Usage: git-conflict-lab start <exercise-id>") {
			t.Errorf("start %v: stderr = %q, want usage hint", args, stderr)
		}
	}
}

func TestStartUnknownExercise(t *testing.T) {
	_, stderr, code := runArgs("start", "unknown-id")
	if code != ExitUsage {
		t.Fatalf("start unknown-id: exit = %d, want %d", code, ExitUsage)
	}
	if !strings.Contains(stderr, "Unknown exercise") {
		t.Errorf("stderr = %q, want friendly unknown-exercise message", stderr)
	}
}

func TestStartKnownExerciseNotAvailable(t *testing.T) {
	_, stderr, code := runArgs("start", "merge-basic")
	if code != ExitNotImplemented {
		t.Fatalf("start merge-basic: exit = %d, want %d", code, ExitNotImplemented)
	}
	if !strings.Contains(stderr, "Milestone B") {
		t.Errorf("stderr = %q, want Milestone B notice", stderr)
	}
}

func TestCheckNotAvailable(t *testing.T) {
	_, stderr, code := runArgs("check")
	if code != ExitNotImplemented {
		t.Fatalf("check: exit = %d, want %d", code, ExitNotImplemented)
	}
	if !strings.Contains(stderr, "Milestone C") {
		t.Errorf("stderr = %q, want Milestone C notice", stderr)
	}
}

func TestCheckUnknownExercise(t *testing.T) {
	_, stderr, code := runArgs("check", "unknown-id")
	if code != ExitUsage {
		t.Fatalf("check unknown-id: exit = %d, want %d", code, ExitUsage)
	}
	if !strings.Contains(stderr, "Unknown exercise") {
		t.Errorf("stderr = %q, want friendly unknown-exercise message", stderr)
	}
}

func TestCheckRejectsExtraArgs(t *testing.T) {
	_, _, code := runArgs("check", "merge-basic", "extra")
	if code != ExitUsage {
		t.Fatalf("check with extra args: exit = %d, want %d", code, ExitUsage)
	}
}

func TestHintNotAvailable(t *testing.T) {
	_, stderr, code := runArgs("hint", "merge-basic")
	if code != ExitNotImplemented {
		t.Fatalf("hint merge-basic: exit = %d, want %d", code, ExitNotImplemented)
	}
	if !strings.Contains(stderr, "Milestone D") {
		t.Errorf("stderr = %q, want Milestone D notice", stderr)
	}
}

func TestHintRequiresOneArg(t *testing.T) {
	cases := [][]string{
		{"hint"},
		{"hint", "merge-basic", "extra"},
	}
	for _, args := range cases {
		_, _, code := runArgs(args...)
		if code != ExitUsage {
			t.Errorf("hint %v: exit = %d, want %d", args, code, ExitUsage)
		}
	}
}

func TestHintUnknownExercise(t *testing.T) {
	_, stderr, code := runArgs("hint", "unknown-id")
	if code != ExitUsage {
		t.Fatalf("hint unknown-id: exit = %d, want %d", code, ExitUsage)
	}
	if !strings.Contains(stderr, "Unknown exercise") {
		t.Errorf("stderr = %q, want friendly unknown-exercise message", stderr)
	}
}

func TestUnknownCommand(t *testing.T) {
	_, stderr, code := runArgs("bogus")
	if code != ExitUsage {
		t.Fatalf("bogus: exit = %d, want %d", code, ExitUsage)
	}
	if !strings.Contains(stderr, "Unknown command 'bogus'") {
		t.Errorf("stderr = %q, want unknown-command message", stderr)
	}
	if !strings.Contains(stderr, "git-conflict-lab help") {
		t.Errorf("stderr = %q, want help hint", stderr)
	}
}

func TestLookupFailureReturnsExitFailure(t *testing.T) {
	// Inject a runtime lookup failure that is NOT ErrNotFound: the CLI must
	// classify it as a runtime failure (exit 1) with the error message,
	// not as a usage error (exit 2).
	original := lookupExercise
	lookupExercise = func(id string) (exercise.Spec, error) {
		return exercise.Spec{}, fmt.Errorf("catalog backend offline (injected)")
	}
	t.Cleanup(func() { lookupExercise = original })

	_, stderr, code := runArgs("start", "merge-basic")
	if code != ExitFailure {
		t.Fatalf("start with failed lookup: exit = %d, want %d", code, ExitFailure)
	}
	if !strings.Contains(stderr, "Error: catalog backend offline (injected)") {
		t.Errorf("stderr = %q, want injected error message", stderr)
	}
}

func TestRunMapsExitCodes(t *testing.T) {
	if got := Run([]string{"help"}); got != ExitOK {
		t.Errorf("Run(help) = %d, want %d", got, ExitOK)
	}
	if got := Run([]string{"nope"}); got != ExitUsage {
		t.Errorf("Run(nope) = %d, want %d", got, ExitUsage)
	}
	if got := Run([]string{"start", "merge-basic"}); got != ExitNotImplemented {
		t.Errorf("Run(start merge-basic) = %d, want %d", got, ExitNotImplemented)
	}
}
