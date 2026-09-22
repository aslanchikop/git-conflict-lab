package lab

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aslanchikop/git-conflict-lab/internal/gitx"
)

// generateInto runs Generate into a fresh temp labs root and returns the
// created repo path.
func generateInto(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	repo, err := Generate(context.Background(), root, "merge-basic")
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	return repo
}

func TestGenerateCreatesRepoWithBranches(t *testing.T) {
	repo := generateInto(t)

	for _, ref := range []string{"main", "feature/login"} {
		if _, err := gitx.Run(context.Background(), repo, "rev-parse", "--verify", ref); err != nil {
			t.Errorf("branch %s missing: %v", ref, err)
		}
	}

	out, err := gitx.Run(context.Background(), repo, "symbolic-ref", "--short", "HEAD")
	if err != nil {
		t.Fatalf("cannot read HEAD: %v", err)
	}
	if got := strings.TrimSpace(out); got != "main" {
		t.Errorf("HEAD = %q, want main", got)
	}
}

func TestGenerateRealMergeConflict(t *testing.T) {
	repo := generateInto(t)
	ctx := context.Background()

	out, err := gitx.Run(ctx, repo, "merge", "feature/login")
	if err == nil {
		t.Fatalf("merge unexpectedly succeeded; output:\n%s", out)
	}
	var exitErr *gitx.ExitError
	if !asExitError(err, &exitErr) {
		t.Fatalf("merge error is not an ExitError: %v", err)
	}

	// A real merge conflict leaves MERGE_HEAD behind.
	if _, statErr := os.Stat(filepath.Join(repo, ".git", "MERGE_HEAD")); statErr != nil {
		t.Errorf("MERGE_HEAD missing after failed merge: %v", statErr)
	}

	// The conflicted file carries conflict markers now.
	data, readErr := os.ReadFile(filepath.Join(repo, "login.go"))
	if readErr != nil {
		t.Fatalf("cannot read conflicted login.go: %v", readErr)
	}
	if !strings.Contains(string(data), "<<<<<<<") || !strings.Contains(string(data), ">>>>>>>") {
		t.Errorf("conflicted login.go has no conflict markers:\n%s", data)
	}

	// Cleanup path users would take: abort restores the pre-merge state.
	if _, abortErr := gitx.Run(ctx, repo, "merge", "--abort"); abortErr != nil {
		t.Errorf("merge --abort failed: %v", abortErr)
	}
	if _, statErr := os.Stat(filepath.Join(repo, ".git", "MERGE_HEAD")); !os.IsNotExist(statErr) {
		t.Errorf("MERGE_HEAD still present after abort")
	}
}

func TestGenerateDeterministicSHAs(t *testing.T) {
	repoA := generateInto(t)
	rootB := t.TempDir()
	repoB, err := Generate(context.Background(), rootB, "merge-basic")
	if err != nil {
		t.Fatalf("second Generate failed: %v", err)
	}
	ctx := context.Background()
	for _, ref := range []string{"main", "feature/login", "main~2"} {
		a, errA := gitx.Run(ctx, repoA, "rev-parse", ref)
		b, errB := gitx.Run(ctx, repoB, "rev-parse", ref)
		if errA != nil || errB != nil {
			t.Fatalf("rev-parse %s failed: %v / %v", ref, errA, errB)
		}
		if strings.TrimSpace(a) != strings.TrimSpace(b) {
			t.Errorf("%s SHA differs between runs: %s vs %s", ref, strings.TrimSpace(a), strings.TrimSpace(b))
		}
	}
}

func TestGenerateSecondRunFailsSafe(t *testing.T) {
	root := t.TempDir()
	if _, err := Generate(context.Background(), root, "merge-basic"); err != nil {
		t.Fatalf("first Generate failed: %v", err)
	}
	_, err := Generate(context.Background(), root, "merge-basic")
	if !errorIs(err, ErrTargetExists) && !errorIs(err, ErrTargetNotEmpty) {
		t.Fatalf("second Generate error = %v, want existing-target family", err)
	}
}

func TestGenerateWritesStateManifest(t *testing.T) {
	repo := generateInto(t)
	data, err := os.ReadFile(filepath.Join(repo, stateDir, "state.json"))
	if err != nil {
		t.Fatalf("state manifest missing: %v", err)
	}
	state := parseStateForTest(t, data)
	if state.ExerciseID != "merge-basic" {
		t.Errorf("manifest exercise_id = %q", state.ExerciseID)
	}
	if state.MainSHA == "" || state.FeatureSHA == "" || state.BaseSHA == "" {
		t.Errorf("manifest missing SHAs: %+v", state)
	}
	if _, err := time.Parse(time.RFC3339, state.CreatedAt); err != nil {
		t.Errorf("manifest created_at not RFC3339: %v", err)
	}
}

// asExitError reports whether err is a *gitx.ExitError.
func asExitError(err error, target **gitx.ExitError) bool {
	e, ok := err.(*gitx.ExitError)
	if ok {
		*target = e
	}
	return ok
}

// errorIs is a thin wrapper so tests read consistently.
func errorIs(err, target error) bool {
	return err != nil && (err == target || strings.Contains(err.Error(), target.Error()))
}

// parseStateForTest unmarshals a state manifest.
func parseStateForTest(t *testing.T, data []byte) State {
	t.Helper()
	var s State
	if err := jsonUnmarshal(data, &s); err != nil {
		t.Fatalf("cannot parse state manifest: %v", err)
	}
	return s
}