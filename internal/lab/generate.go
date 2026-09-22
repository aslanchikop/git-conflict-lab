// generate.go implements deterministic exercise-repository generation.
// Milestone B scope: create a real, isolated Git repository for an
// exercise at a validated target path, with a per-exercise state manifest
// for the Milestone C checker.
//
// Determinism: all commits use a fixed identity and fixed author/committer
// dates via per-invocation environment, so two generations produce
// byte-identical commits (identical SHAs).
//
// Safety: the target path is re-validated immediately before creation
// (TOCTOU); generation never touches global Git configuration, never adds
// remotes, and never runs user-supplied commands - every Git invocation
// uses explicit argument arrays through internal/gitx.
package lab

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/aslanchikop/git-conflict-lab/internal/gitx"
)

// Generation identity and dates are fixed so that exercise repositories
// are reproducible byte-for-byte. These are synthetic values; they never
// touch the user's real Git identity.
const (
	generatorName  = "Git Conflict Lab"
	generatorEmail = "git-conflict-lab@localhost"
	// fixedDate pins author and committer dates for determinism.
	fixedDate = "2026-01-01T00:00:00+00:00"
)

// baseEnv returns the per-invocation environment used for every Git call
// during generation: system and global config are disabled, prompts are
// off, and dates are pinned.
func baseEnv(date string) []string {
	return []string{
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_TERMINAL_PROMPT=0",
		"GIT_AUTHOR_DATE=" + date,
		"GIT_COMMITTER_DATE=" + date,
	}
}

// commitEnv returns the environment plus inline identity configuration.
// Identity is passed via -c flags on every commit command; no git config
// file is ever written.
func commitEnv(date string) []string {
	return append(baseEnv(date),
		"GIT_AUTHOR_NAME="+generatorName,
		"GIT_AUTHOR_EMAIL="+generatorEmail,
		"GIT_COMMITTER_NAME="+generatorName,
		"GIT_COMMITTER_EMAIL="+generatorEmail,
	)
}

// State is the advisory manifest written inside a generated exercise
// repository at .git-conflict-lab/state.json. It records what the
// generator produced so the Milestone C checker can cross-check the
// manifest against the actual Git state.
//
// Trust limits: users can modify this file freely, so it is advisory
// only - never a security boundary. The checker must verify the manifest
// against the real Git refs and treat any mismatch as a failure, not
// trust the file itself.
type State struct {
	ExerciseID     string `json:"exercise_id"`
	CreatedAt      string `json:"created_at"`
	LabsRoot       string `json:"labs_root"`
	BaseSHA        string `json:"base_sha"`
	MainSHA        string `json:"main_sha"`
	FeatureSHA     string `json:"feature_sha"`
	FeatureBranch  string `json:"feature_branch"`
	ConflictFile   string `json:"conflict_file"`
	CheckerVersion int    `json:"checker_version"`
}

// stateDir is the manifest location inside a generated repository.
const stateDir = ".git-conflict-lab"

// Generate creates a new, isolated exercise repository for the exercise
// with the given name under labsRoot. It returns the absolute path of the
// created repository.
//
// The caller must pass a labsRoot that already exists. All validations
// from ResolveTargetDir apply; the target path is additionally re-checked
// immediately before creation (TOCTOU).
func Generate(ctx context.Context, labsRoot, name string) (string, error) {
	candidate, err := ResolveTargetDir(labsRoot, name)
	if err != nil {
		return "", err
	}

	// TOCTOU re-validation: the candidate must still not exist right
	// before we create it. ResolveTargetDir already verified this, but
	// filesystem state can change between validation and creation.
	if _, err := os.Lstat(candidate); err == nil {
		return "", fmt.Errorf("%w: %s appeared during generation", ErrTargetExists, candidate)
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("%w: cannot re-verify candidate %s: %v", ErrUnsafeLocation, candidate, err)
	}

	if err := os.MkdirAll(candidate, 0o755); err != nil {
		return "", fmt.Errorf("%w: cannot create exercise directory %s: %v", ErrUnsafeLocation, candidate, err)
	}

	if err := generateMergeBasic(ctx, candidate); err != nil {
		return "", err
	}

	if err := writeState(ctx, labsRoot, candidate); err != nil {
		return "", err
	}

	return candidate, nil
}

// gitRun is a helper that runs git inside the exercise repository with
// the generation environment applied.
func gitRun(ctx context.Context, dir string, date string, args ...string) (string, error) {
	return gitx.RunWithEnv(ctx, dir, baseEnv(date), args...)
}

// gitCommit runs git commit with the fixed generator identity.
func gitCommit(ctx context.Context, dir string, args ...string) (string, error) {
	full := append([]string{"-c", "user.name=" + generatorName, "-c", "user.email=" + generatorEmail}, args...)
	return gitx.RunWithEnv(ctx, dir, commitEnv(fixedDate), full...)
}

// writeFileSync writes data to path using LF line endings.
func writeFileSync(path string, data []byte) error {
	return os.WriteFile(path, data, 0o644)
}

// generateMergeBasic builds the merge-basic scenario:
//
//   base:   login.go created (both sides descend from this commit)
//   main:   two commits - a doc-comment edit and a real code edit that
//           rewrites the same return line the feature branch rewrites
//   feature/login: rewrites that same line differently
//
// Merging feature into main conflicts because both sides change the same
// line differently (content conflict under the ort strategy).
func generateMergeBasic(ctx context.Context, repo string) error {
	// git init with main as the default branch. We use init.defaultBranch
	// via -c so no global or system configuration is consulted or written.
	if _, err := gitx.RunWithEnv(ctx, repo, baseEnv(fixedDate),
		"-c", "init.defaultBranch=main", "init"); err != nil {
		return fmt.Errorf("git init failed: %w", err)
	}

	loginBase := `package auth

// Login validates the given credentials.
func Login(user, password string) bool {
	if user == "" || password == "" {
		return false
	}
	return true
}
`
	if err := writeFileSync(filepath.Join(repo, "login.go"), []byte(loginBase)); err != nil {
		return fmt.Errorf("cannot write login.go: %w", err)
	}
	if _, err := gitCommit(ctx, repo, "add", "login.go"); err != nil {
		return fmt.Errorf("cannot stage login.go: %w", err)
	}
	if _, err := gitCommit(ctx, repo, "commit", "-m", "feat: add login flow"); err != nil {
		return fmt.Errorf("cannot commit on main: %w", err)
	}

	// feature/login: same region, different implementation.
	if _, err := gitRun(ctx, repo, fixedDate, "checkout", "-b", "feature/login"); err != nil {
		return fmt.Errorf("cannot create feature/login: %w", err)
	}
	loginFeature := `package auth

// Login validates the given credentials.
func Login(user, password string) bool {
	if user == "" || password == "" {
		return false
	}
	return len(user) >= 3 && len(password) >= 12
}
`
	if err := writeFileSync(filepath.Join(repo, "login.go"), []byte(loginFeature)); err != nil {
		return fmt.Errorf("cannot write feature login.go: %w", err)
	}
	if _, err := gitCommit(ctx, repo, "add", "login.go"); err != nil {
		return fmt.Errorf("cannot stage feature login.go: %w", err)
	}
	if _, err := gitCommit(ctx, repo, "commit", "-m", "feat: stricter login validation"); err != nil {
		return fmt.Errorf("cannot commit on feature/login: %w", err)
	}

	// main, commit 1: doc comment above the function (independent edit).
	if _, err := gitRun(ctx, repo, fixedDate, "checkout", "main"); err != nil {
		return fmt.Errorf("cannot switch back to main: %w", err)
	}
	loginDoc := `package auth

// Login checks the given credentials and reports validity.
func Login(user, password string) bool {
	if user == "" || password == "" {
		return false
	}
	return true
}
`
	if err := writeFileSync(filepath.Join(repo, "login.go"), []byte(loginDoc)); err != nil {
		return fmt.Errorf("cannot write main doc login.go: %w", err)
	}
	if _, err := gitCommit(ctx, repo, "add", "login.go"); err != nil {
		return fmt.Errorf("cannot stage main doc login.go: %w", err)
	}
	if _, err := gitCommit(ctx, repo, "commit", "-m", "docs: clarify login comment"); err != nil {
		return fmt.Errorf("cannot commit docs change: %w", err)
	}

	// main, commit 2: rewrites the same return line the feature branch
	// rewrote, but differently -> guaranteed content conflict on merge.
	loginMain := `package auth

// Login checks the given credentials and reports validity.
func Login(user, password string) bool {
	if user == "" || password == "" {
		return false
	}
	return len(password) >= 8
}
`
	if err := writeFileSync(filepath.Join(repo, "login.go"), []byte(loginMain)); err != nil {
		return fmt.Errorf("cannot write main login.go: %w", err)
	}
	if _, err := gitCommit(ctx, repo, "add", "login.go"); err != nil {
		return fmt.Errorf("cannot stage main login.go: %w", err)
	}
	if _, err := gitCommit(ctx, repo, "commit", "-m", "feat: enforce password length on main"); err != nil {
		return fmt.Errorf("cannot commit main change: %w", err)
	}

	return nil
}
// writeState records the generated scenario in the advisory manifest.
func writeState(ctx context.Context, labsRoot, repo string) error {
	sha := func(ref string) string {
		out, err := gitx.RunWithEnv(ctx, repo, baseEnv(fixedDate), "rev-parse", ref)
		if err != nil {
			return ""
		}
		return trimSpace(out)
	}

	state := State{
		ExerciseID:     "merge-basic",
		CreatedAt:      time.Now().UTC().Format(time.RFC3339),
		LabsRoot:       labsRoot,
		BaseSHA:        sha("main~2"),
		MainSHA:        sha("main"),
		FeatureSHA:     sha("feature/login"),
		FeatureBranch:  "feature/login",
		ConflictFile:   "login.go",
		CheckerVersion: 0,
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("cannot marshal state: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(repo, stateDir), 0o755); err != nil {
		return fmt.Errorf("cannot create state dir: %w", err)
	}
	return writeFileSync(filepath.Join(repo, stateDir, "state.json"), data)
}

// trimSpace is a tiny helper to keep the sha helper readable.
func trimSpace(s string) string {
	start := 0
	for start < len(s) && (s[start] == ' ' || s[start] == '\n' || s[start] == '\r' || s[start] == '	') {
		start++
	}
	end := len(s)
	for end > start && (s[end-1] == ' ' || s[end-1] == '\n' || s[end-1] == '\r' || s[end-1] == '	') {
		end--
	}
	return s[start:end]
}
// jsonUnmarshal is a small indirection over encoding/json for tests.
func jsonUnmarshal(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}