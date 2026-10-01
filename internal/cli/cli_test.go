package cli

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
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
		for _, want := range []string{"list", "start", "reset", "check", "hint", "clean", "doctor", "ui", "version", "help"} {
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
	if !strings.Contains(stdout, "git-conflict-lab 0.1.0") {
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

func TestStartGeneratesExercise(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping slow generation test in -short mode")
	}
	labs := t.TempDir()
	t.Setenv(labsRootEnv, labs)

	stdout, _, code := runArgs("start", "merge-basic")
	if code != ExitOK {
		t.Fatalf("start merge-basic: exit = %d, want %d", code, ExitOK)
	}
	if !strings.Contains(stdout, "Exercise ready") {
		t.Errorf("stdout = %q, want ready notice", stdout)
	}
	created := filepath.Join(labs, "merge-basic")
	if info, err := os.Stat(created); err != nil || !info.IsDir() {
		t.Fatalf("exercise dir %s missing: %v", created, err)
	}
	if _, err := os.Stat(filepath.Join(created, ".git")); err != nil {
		t.Errorf("generated dir is not a Git repository: %v", err)
	}
}

func TestStartTwiceFailsSafe(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping slow generation test in -short mode")
	}
	t.Setenv(labsRootEnv, t.TempDir())

	stdout, _, code := runArgs("start", "merge-basic")
	if code != ExitOK {
		t.Fatalf("first start: exit = %d, want %d (stdout=%q)", code, ExitOK, stdout)
	}
	_, stderr, code := runArgs("start", "merge-basic")
	if code == ExitOK {
		t.Fatalf("second start: exit = %d, want non-zero", code)
	}
	if !strings.Contains(stderr, "already exists") {
		t.Errorf("stderr = %q, want existing-dir notice", stderr)
	}
}

func TestStartNamedAttempt(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping slow generation test in -short mode")
	}
	labs := t.TempDir()
	t.Setenv(labsRootEnv, labs)
	_, stderr, code := runArgs("start", "add-add", "--attempt", "second")
	if code != ExitOK {
		t.Fatalf("start named attempt: exit=%d stderr=%q", code, stderr)
	}
	if _, err := os.Stat(filepath.Join(labs, "add-add-second", ".git")); err != nil {
		t.Fatal(err)
	}
	_, _, code = runArgs("start", "add-add", "--attempt", "second")
	if code != ExitFailure {
		t.Fatalf("same attempt should fail safely, got %d", code)
	}
}
func TestCheckOutsideLab(t *testing.T) {
	_, stderr, code := runArgs("check")
	if code != ExitFailure {
		t.Fatalf("check: exit = %d, want %d", code, ExitFailure)
	}
	if !strings.Contains(stderr, "no exercise found") {
		t.Errorf("stderr = %q, want no exercise notice", stderr)
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

func TestHintOutsideLab(t *testing.T) {
	_, stderr, code := runArgs("hint", "merge-basic")
	if code != ExitFailure {
		t.Fatalf("hint merge-basic: exit = %d, want %d", code, ExitFailure)
	}
	if !strings.Contains(stderr, "no exercise found") {
		t.Errorf("stderr = %q, want no exercise notice", stderr)
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
	// start is functional now; verify the real path with an isolated labs
	// root so the user's home is never touched by tests.
	t.Setenv(labsRootEnv, t.TempDir())
	if got := Run([]string{"start", "merge-basic"}); got != ExitOK {
		t.Errorf("Run(start merge-basic) = %d, want %d", got, ExitOK)
	}
}

func TestResetRegeneratesAttempt(t *testing.T) {
	if testing.Short() {
		t.Skip("generates a real Git repository")
	}
	labs := t.TempDir()
	t.Setenv(labsRootEnv, labs)

	if _, _, code := runArgs("start", "merge-basic"); code != ExitOK {
		t.Fatalf("start: exit = %d, want %d", code, ExitOK)
	}
	junk := filepath.Join(labs, "merge-basic", "junk.txt")
	if err := os.WriteFile(junk, []byte("leftover"), 0o644); err != nil {
		t.Fatal(err)
	}

	stdout, _, code := runArgs("reset", "merge-basic")
	if code != ExitOK {
		t.Fatalf("reset: exit = %d, stdout = %q", code, stdout)
	}
	if !strings.Contains(stdout, "Exercise regenerated") {
		t.Errorf("stdout = %q, want regenerated notice", stdout)
	}
	if _, err := os.Stat(junk); !os.IsNotExist(err) {
		t.Error("reset kept files from the previous attempt")
	}
	if _, err := os.Stat(filepath.Join(labs, "merge-basic", ".git")); err != nil {
		t.Errorf("repository missing after reset: %v", err)
	}
}

func TestResetUnknownExercise(t *testing.T) {
	_, stderr, code := runArgs("reset", "unknown-id")
	if code != ExitUsage {
		t.Fatalf("reset unknown-id: exit = %d, want %d", code, ExitUsage)
	}
	if !strings.Contains(stderr, "Unknown exercise") {
		t.Errorf("stderr = %q, want friendly unknown-exercise message", stderr)
	}
}

func TestResetRefusesNonLabDirectory(t *testing.T) {
	labs := t.TempDir()
	t.Setenv(labsRootEnv, labs)
	if err := os.MkdirAll(filepath.Join(labs, "merge-basic"), 0o755); err != nil {
		t.Fatal(err)
	}

	_, stderr, code := runArgs("reset", "merge-basic")
	if code != ExitFailure {
		t.Fatalf("reset non-lab: exit = %d, want %d", code, ExitFailure)
	}
	if !strings.Contains(stderr, "nothing was deleted") {
		t.Errorf("stderr = %q, want refusal notice", stderr)
	}
	if _, err := os.Stat(filepath.Join(labs, "merge-basic")); err != nil {
		t.Fatal("refused reset must leave the directory in place")
	}
}

func TestCleanDryRunThenRemove(t *testing.T) {
	labs := t.TempDir()
	t.Setenv(labsRootEnv, labs)
	manifest := filepath.Join(labs, "merge-basic", ".git-conflict-lab", "state.json")
	if err := os.MkdirAll(filepath.Dir(manifest), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifest, []byte(`{"exercise_id":"merge-basic"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	stdout, _, code := runArgs("clean")
	if code != ExitOK || !strings.Contains(stdout, "dry run") || !strings.Contains(stdout, "merge-basic") {
		t.Fatalf("clean dry run: exit = %d, stdout = %q", code, stdout)
	}
	if _, err := os.Stat(manifest); err != nil {
		t.Fatal("dry run must not delete anything")
	}

	stdout, _, code = runArgs("clean", "--yes")
	if code != ExitOK || !strings.Contains(stdout, "Removed 1") {
		t.Fatalf("clean --yes: exit = %d, stdout = %q", code, stdout)
	}
	if _, err := os.Stat(manifest); !os.IsNotExist(err) {
		t.Error("attempt must be removed after clean --yes")
	}
}

func TestCleanWithoutLabsRoot(t *testing.T) {
	t.Setenv(labsRootEnv, filepath.Join(t.TempDir(), "missing"))
	stdout, _, code := runArgs("clean")
	if code != ExitOK || !strings.Contains(stdout, "No exercise attempts found") {
		t.Fatalf("clean without labs root: exit = %d, stdout = %q", code, stdout)
	}
}

func TestCleanRejectsUnknownFlags(t *testing.T) {
	_, _, code := runArgs("clean", "--bogus")
	if code != ExitUsage {
		t.Fatalf("clean --bogus: exit = %d, want %d", code, ExitUsage)
	}
}

func TestDoctorReportsEnvironment(t *testing.T) {
	t.Setenv(labsRootEnv, t.TempDir())
	stdout, _, code := runArgs("doctor")
	if code != ExitOK {
		t.Fatalf("doctor: exit = %d, want %d", code, ExitOK)
	}
	for _, want := range []string{"Git:", "Labs directory:", "Attempts:", "Environment ready"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("doctor output missing %q:\n%s", want, stdout)
		}
	}
}

func TestDoctorRejectsExtraArgs(t *testing.T) {
	_, _, code := runArgs("doctor", "extra")
	if code != ExitUsage {
		t.Fatalf("doctor extra: exit = %d, want %d", code, ExitUsage)
	}
}
