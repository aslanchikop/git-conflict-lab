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

	// git merge validates the committer identity before merging, so the
	// merge (like any commit-creating operation) needs one. Users run this
	// with their own identity; tests pin the generator identity.
	mergeEnv := []string{
		"GIT_AUTHOR_NAME=Git Conflict Lab",
		"GIT_AUTHOR_EMAIL=git-conflict-lab@localhost",
		"GIT_COMMITTER_NAME=Git Conflict Lab",
		"GIT_COMMITTER_EMAIL=git-conflict-lab@localhost",
	}
	out, err := gitx.RunWithEnv(ctx, repo, mergeEnv, "merge", "feature/login")
	var exitErr *gitx.ExitError
	isExit := asExitError(err, &exitErr)
	if err == nil {
		t.Fatalf("merge unexpectedly succeeded; output:\n%s", out)
	}
	if !isExit {
		t.Fatalf("merge error is not an ExitError: %v", err)
	}
	t.Logf("merge exit=%d output:\n%s", exitErr.ExitCode, out)

	// Diagnostics for unexpected failures: show the actual merge output
	// and repository state so CI failures are debuggable.
	if _, statErr := os.Stat(filepath.Join(repo, ".git", "MERGE_HEAD")); statErr != nil {
		st, _ := gitx.Run(ctx, repo, "status", "--porcelain")
		t.Fatalf("MERGE_HEAD missing after failed merge (exit=%d): merge output:\n%s\nstatus:\n%s",
			exitErr.ExitCode, out, st)
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

func TestGenerateIgnoresHostileGlobalConfig(t *testing.T) {
	// A user with commit.gpgsign=true (and no usable GPG setup for the
	// tool) must not break generation: the generator neutralizes global
	// and system Git config per invocation.
	global := filepath.Join(t.TempDir(), "gitconfig")
	cfg := "[commit]\n\tgpgsign = true\n[user]\n\tname = Someone Else\n\temail = someone@else.invalid\n"
	if err := os.WriteFile(global, []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", global)

	repo, err := Generate(context.Background(), t.TempDir(), "merge-basic")
	if err != nil {
		t.Fatalf("Generate failed with hostile global config: %v", err)
	}
	// Identity must be the generator's, not the user's.
	out, err := gitx.Run(context.Background(), repo, "log", "-1", "--format=%an <%ae>", "main")
	if err != nil {
		t.Fatalf("git log failed: %v", err)
	}
	if got := strings.TrimSpace(out); got != "Git Conflict Lab <git-conflict-lab@localhost>" {
		t.Errorf("commit identity = %q, want generator identity", got)
	}
}

func TestGeneratedLabMergesWithoutGlobalIdentity(t *testing.T) {
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	for _, key := range []string{"GIT_AUTHOR_NAME", "GIT_AUTHOR_EMAIL", "GIT_COMMITTER_NAME", "GIT_COMMITTER_EMAIL"} {
		value, exists := os.LookupEnv(key)
		if err := os.Unsetenv(key); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if exists {
				_ = os.Setenv(key, value)
			} else {
				_ = os.Unsetenv(key)
			}
		})
	}
	repo := generateInto(t)
	for _, key := range []string{"user.name", "user.email"} {
		if out, err := gitx.Run(context.Background(), repo, "config", "--local", "--get", key); err != nil || strings.TrimSpace(out) == "" {
			t.Fatalf("local %s missing: %q, %v", key, out, err)
		}
	}
	if out, err := gitx.Run(context.Background(), repo, "merge", "feature/login"); err == nil || !strings.Contains(out, "CONFLICT") {
		t.Fatalf("expected conflict without global identity, got output %q, error %v", out, err)
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

	// The manifest must not dirty the user's worktree.
	status, err := gitx.Run(context.Background(), repo, "status", "--porcelain")
	if err != nil {
		t.Fatalf("git status failed: %v", err)
	}
	if strings.TrimSpace(status) != "" {
		t.Errorf("generated repo is dirty, want clean status:\n%s", status)
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
