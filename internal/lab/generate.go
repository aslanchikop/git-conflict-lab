// generate.go implements deterministic exercise-repository generation.
// It creates real, isolated Git repositories at validated target paths
// and writes an advisory state manifest for the checker.
//
// Determinism: all commits use a fixed identity and fixed author/committer
// dates via per-invocation environment, so two generations produce
// byte-identical starting commits (identical SHAs).
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
	"strings"
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
		// Neutralize user global config (commit.gpgsign, core.autocrlf,
		// user.name overrides, ...): generation must not depend on it and
		// must never fail because of it. os.DevNull is "nul" on Windows
		// and /dev/null elsewhere.
		"GIT_CONFIG_GLOBAL=" + os.DevNull,
		"GIT_CONFIG_COUNT=0",
		"GIT_TERMINAL_PROMPT=0",
		"GIT_AUTHOR_DATE=" + date,
		"GIT_COMMITTER_DATE=" + date,
	}
}

// commitEnv pins the generator identity for its own commits. A local lab
// identity is written later so learners can merge and commit without setup.
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
// generator produced so the checker can cross-check the
// manifest against the actual Git state.
//
// Trust limits: users can modify this file freely, so it is advisory
// only - never a security boundary. The checker must verify the manifest
// against the real Git refs and treat any mismatch as a failure, not
// trust the file itself.
type State struct {
	ExerciseID     string   `json:"exercise_id"`
	CreatedAt      string   `json:"created_at"`
	LabsRoot       string   `json:"labs_root"`
	BaseSHA        string   `json:"base_sha"`
	MainSHA        string   `json:"main_sha"`
	FeatureSHA     string   `json:"feature_sha"`
	FeatureBranch  string   `json:"feature_branch"`
	ConflictFile   string   `json:"conflict_file"`
	ConflictFiles  []string `json:"conflict_files,omitempty"`
	CheckerVersion int      `json:"checker_version"`
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
	return GenerateAs(ctx, labsRoot, name, name)
}

// GenerateAs creates another named attempt without replacing an existing one.
func GenerateAs(ctx context.Context, labsRoot, id, folder string) (string, error) {
	if _, ok := LookupScenario(id); !ok {
		return "", fmt.Errorf("unknown exercise %q", id)
	}
	candidate, err := ResolveTargetDir(labsRoot, folder)
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

	// The candidate did not exist moments ago (re-verified above), so any
	// partially generated directory is debris from this very call. Remove
	// it on failure so a retry starts clean instead of being refused with
	// ErrTargetNotEmpty (review finding B-003).
	if err := populateExercise(ctx, labsRoot, candidate, id); err != nil {
		_ = os.RemoveAll(candidate)
		return "", err
	}

	return candidate, nil
}

// populateExercise runs every post-creation step of GenerateAs: the
// scenario-specific Git history, the advisory manifest, the
// repository-local exclude entry, and the lab-local commit identity.
func populateExercise(ctx context.Context, labsRoot, repo, id string) error {
	if err := generateScenario(ctx, id, repo); err != nil {
		return err
	}
	if err := writeState(ctx, labsRoot, repo, id); err != nil {
		return err
	}
	// Keep the user's `git status` clean: the advisory manifest lives in
	// the worktree, so exclude it via the repository-local exclude file
	// (never the global config, never a tracked .gitignore).
	if err := excludeManifestDir(repo); err != nil {
		return err
	}
	return configureLabGit(repo)
}

// generateScenario builds the scenario-specific Git history in a freshly
// created repository. It is a package variable so tests can inject
// failures and verify the cleanup behavior of GenerateAs.
var generateScenario = func(ctx context.Context, id, repo string) error {
	switch id {
	case "merge-basic":
		return generateMergeBasic(ctx, repo)
	case "add-add":
		return generateAddAdd(ctx, repo)
	case "modify-delete":
		return generateModifyDelete(ctx, repo)
	case "rebase-basic":
		return generateRebaseBasic(ctx, repo)
	case "merge-multi":
		return generateMergeMulti(ctx, repo)
	case "cherry-pick":
		return generateCherryPick(ctx, repo)
	default:
		return fmt.Errorf("unknown exercise %q", id)
	}
}

// configureLabGit gives beginners a working commit identity in this one
// generated repository, without reading or writing their global Git config.
func configureLabGit(repo string) error {
	ctx := context.Background()
	for key, value := range map[string]string{
		"user.name":      generatorName,
		"user.email":     generatorEmail,
		"commit.gpgsign": "false",
		"core.autocrlf":  "false",
	} {
		if _, err := gitx.RunWithEnv(ctx, repo, baseEnv(fixedDate), "config", "--local", key, value); err != nil {
			return fmt.Errorf("cannot set lab-local Git config %s: %w", key, err)
		}
	}
	return nil
}

// excludeManifestDir appends the manifest directory to the repository's
// local .git/info/exclude so the generated repo starts with a clean
// `git status`. The file is repository-local and untracked by design.
func excludeManifestDir(repo string) error {
	excludePath := filepath.Join(repo, ".git", "info", "exclude")
	line := stateDir + "/"
	var content string
	if data, err := os.ReadFile(excludePath); err == nil {
		content = string(data)
		if strings.Contains(content, line) {
			return nil
		}
		if len(content) > 0 && !strings.HasSuffix(content, "\n") {
			content += "\n"
		}
	}
	content += "# git-conflict-lab advisory state (not part of the exercise)\n" + line + "\n"
	return os.WriteFile(excludePath, []byte(content), 0o644)
}

// gitRun is a helper that runs git inside the exercise repository with
// the generation environment applied.
func gitRun(ctx context.Context, dir string, args ...string) (string, error) {
	return gitx.RunWithEnv(ctx, dir, baseEnv(fixedDate), args...)
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
//	base:   login.go created (both sides descend from this commit)
//	main:   two commits - a doc-comment edit and a real code edit that
//	        rewrites the same return line the feature branch rewrites
//	feature/login: rewrites that same line differently
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
	if _, err := gitRun(ctx, repo, "checkout", "-b", "feature/login"); err != nil {
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
	if _, err := gitRun(ctx, repo, "checkout", "main"); err != nil {
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

func generateAddAdd(ctx context.Context, repo string) error {
	if _, err := gitx.RunWithEnv(ctx, repo, baseEnv(fixedDate), "-c", "init.defaultBranch=main", "init"); err != nil {
		return fmt.Errorf("git init failed: %w", err)
	}
	if err := writeFileSync(filepath.Join(repo, "exercise.txt"), []byte("Both branches will add notes.txt.\n")); err != nil {
		return err
	}
	if _, err := gitCommit(ctx, repo, "add", "exercise.txt"); err != nil {
		return err
	}
	if _, err := gitCommit(ctx, repo, "commit", "-m", "chore: initialize notes exercise"); err != nil {
		return err
	}
	if _, err := gitRun(ctx, repo, "checkout", "-b", "feature/notes"); err != nil {
		return err
	}
	if err := writeFileSync(filepath.Join(repo, "notes.txt"), []byte("Feature note: document the review steps.\n")); err != nil {
		return err
	}
	if _, err := gitCommit(ctx, repo, "add", "notes.txt"); err != nil {
		return err
	}
	if _, err := gitCommit(ctx, repo, "commit", "-m", "docs: add feature notes"); err != nil {
		return err
	}
	if _, err := gitRun(ctx, repo, "checkout", "main"); err != nil {
		return err
	}
	if err := writeFileSync(filepath.Join(repo, "notes.txt"), []byte("Main note: record the release checklist.\n")); err != nil {
		return err
	}
	if _, err := gitCommit(ctx, repo, "add", "notes.txt"); err != nil {
		return err
	}
	if _, err := gitCommit(ctx, repo, "commit", "-m", "docs: add main notes"); err != nil {
		return err
	}
	return nil
}

func generateModifyDelete(ctx context.Context, repo string) error {
	if _, err := gitx.RunWithEnv(ctx, repo, baseEnv(fixedDate), "-c", "init.defaultBranch=main", "init"); err != nil {
		return fmt.Errorf("git init failed: %w", err)
	}
	if err := writeFileSync(filepath.Join(repo, "legacy.txt"), []byte("Legacy integration instructions.\n")); err != nil {
		return err
	}
	if _, err := gitCommit(ctx, repo, "add", "legacy.txt"); err != nil {
		return err
	}
	if _, err := gitCommit(ctx, repo, "commit", "-m", "docs: add legacy instructions"); err != nil {
		return err
	}
	if _, err := gitRun(ctx, repo, "checkout", "-b", "feature/cleanup"); err != nil {
		return err
	}
	if _, err := gitRun(ctx, repo, "rm", "legacy.txt"); err != nil {
		return err
	}
	if _, err := gitCommit(ctx, repo, "commit", "-m", "chore: remove obsolete instructions"); err != nil {
		return err
	}
	if _, err := gitRun(ctx, repo, "checkout", "main"); err != nil {
		return err
	}
	if err := writeFileSync(filepath.Join(repo, "legacy.txt"), []byte("Legacy integration instructions, revised.\n")); err != nil {
		return err
	}
	if _, err := gitCommit(ctx, repo, "add", "legacy.txt"); err != nil {
		return err
	}
	if _, err := gitCommit(ctx, repo, "commit", "-m", "docs: revise legacy instructions"); err != nil {
		return err
	}
	return nil
}

func generateRebaseBasic(ctx context.Context, repo string) error {
	if _, err := gitx.RunWithEnv(ctx, repo, baseEnv(fixedDate), "-c", "init.defaultBranch=main", "init"); err != nil {
		return fmt.Errorf("git init failed: %w", err)
	}
	file := filepath.Join(repo, "settings.txt")
	if err := writeFileSync(file, []byte("mode=standard\n")); err != nil {
		return err
	}
	if _, err := gitCommit(ctx, repo, "add", "settings.txt"); err != nil {
		return err
	}
	if _, err := gitCommit(ctx, repo, "commit", "-m", "chore: add default mode"); err != nil {
		return err
	}
	if _, err := gitRun(ctx, repo, "checkout", "-b", "feature/fast"); err != nil {
		return err
	}
	if err := writeFileSync(file, []byte("mode=fast\n")); err != nil {
		return err
	}
	if _, err := gitCommit(ctx, repo, "add", "settings.txt"); err != nil {
		return err
	}
	if _, err := gitCommit(ctx, repo, "commit", "-m", "feat: enable fast mode"); err != nil {
		return err
	}
	if _, err := gitRun(ctx, repo, "checkout", "main"); err != nil {
		return err
	}
	if err := writeFileSync(file, []byte("mode=safe\n")); err != nil {
		return err
	}
	if _, err := gitCommit(ctx, repo, "add", "settings.txt"); err != nil {
		return err
	}
	if _, err := gitCommit(ctx, repo, "commit", "-m", "feat: add safe mode"); err != nil {
		return err
	}
	return nil
}

func generateMergeMulti(ctx context.Context, repo string) error {
	if _, err := gitx.RunWithEnv(ctx, repo, baseEnv(fixedDate), "-c", "init.defaultBranch=main", "init"); err != nil {
		return err
	}
	write := func(config, review string) error {
		if err := writeFileSync(filepath.Join(repo, "config.txt"), []byte(config+"\n")); err != nil {
			return err
		}
		return writeFileSync(filepath.Join(repo, "review.txt"), []byte(review+"\n"))
	}
	commit := func(message string) error {
		if _, err := gitCommit(ctx, repo, "add", "config.txt", "review.txt"); err != nil {
			return err
		}
		_, err := gitCommit(ctx, repo, "commit", "-m", message)
		return err
	}
	if err := write("timeout=30", "review=none"); err != nil {
		return err
	}
	if err := commit("chore: add deployment defaults"); err != nil {
		return err
	}
	if _, err := gitRun(ctx, repo, "checkout", "-b", "feature/release"); err != nil {
		return err
	}
	if err := write("timeout=60", "review=automated"); err != nil {
		return err
	}
	if err := commit("feat: prepare automated release"); err != nil {
		return err
	}
	if _, err := gitRun(ctx, repo, "checkout", "main"); err != nil {
		return err
	}
	if err := write("timeout=45", "review=manual"); err != nil {
		return err
	}
	return commit("feat: require manual release review")
}

func generateCherryPick(ctx context.Context, repo string) error {
	if _, err := gitx.RunWithEnv(ctx, repo, baseEnv(fixedDate), "-c", "init.defaultBranch=main", "init"); err != nil {
		return err
	}
	file := filepath.Join(repo, "policy.txt")
	commit := func(content, message string) error {
		if err := writeFileSync(file, []byte(content+"\n")); err != nil {
			return err
		}
		if _, err := gitCommit(ctx, repo, "add", "policy.txt"); err != nil {
			return err
		}
		_, err := gitCommit(ctx, repo, "commit", "-m", message)
		return err
	}
	if err := commit("audit=off", "chore: add audit policy"); err != nil {
		return err
	}
	if _, err := gitRun(ctx, repo, "checkout", "-b", "feature/audit"); err != nil {
		return err
	}
	if err := commit("audit=verbose", "feat: add verbose audit"); err != nil {
		return err
	}
	if _, err := gitRun(ctx, repo, "checkout", "main"); err != nil {
		return err
	}
	return commit("audit=required", "feat: require audit")
}

// writeState records the generated scenario in the advisory manifest.
func writeState(ctx context.Context, labsRoot, repo, id string) error {
	sc, ok := LookupScenario(id)
	if !ok {
		return fmt.Errorf("unknown exercise %q", id)
	}
	sha := func(ref string) string {
		out, err := gitx.RunWithEnv(ctx, repo, baseEnv(fixedDate), "rev-parse", ref)
		if err != nil {
			return ""
		}
		return trimSpace(out)
	}

	state := State{
		ExerciseID:     id,
		CreatedAt:      time.Now().UTC().Format(time.RFC3339),
		LabsRoot:       labsRoot,
		BaseSHA:        sha(sc.BaseRef),
		MainSHA:        sha("main"),
		FeatureSHA:     sha(sc.Branch),
		FeatureBranch:  sc.Branch,
		ConflictFile:   sc.File,
		CheckerVersion: 0,
	}
	if len(sc.Files) > 1 {
		state.ConflictFiles = append([]string(nil), sc.Files...)
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
